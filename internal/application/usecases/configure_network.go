package usecases

import (
	"context"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"multinic-agent-v2/internal/domain/services"
	"net"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ConfigureNetworkUseCase는 네트워크 설정을 처리하는 유스케이스입니다
type ConfigureNetworkUseCase struct {
	repository    interfaces.NetworkInterfaceRepository
	configurer    interfaces.NetworkConfigurer
	rollbacker    interfaces.NetworkRollbacker
	namingService *services.InterfaceNamingService
	fileSystem    interfaces.FileSystem // 파일 시스템 의존성 추가
	logger        *logrus.Logger
}

// NewConfigureNetworkUseCase는 새로운 ConfigureNetworkUseCase를 생성합니다
func NewConfigureNetworkUseCase(
	repo interfaces.NetworkInterfaceRepository,
	configurer interfaces.NetworkConfigurer,
	rollbacker interfaces.NetworkRollbacker,
	naming *services.InterfaceNamingService,
	fs interfaces.FileSystem, // 파일 시스템 의존성 추가
	logger *logrus.Logger,
) *ConfigureNetworkUseCase {
	return &ConfigureNetworkUseCase{
		repository:    repo,
		configurer:    configurer,
		rollbacker:    rollbacker,
		namingService: naming,
		fileSystem:    fs,
		logger:        logger,
	}
}

// ConfigureNetworkInput은 유스케이스의 입력 파라미터입니다
type ConfigureNetworkInput struct {
	NodeName string
}

// ConfigureNetworkOutput은 유스케이스의 출력 결과입니다
type ConfigureNetworkOutput struct {
	ProcessedCount int
	FailedCount    int
	TotalCount     int
}

// Execute는 네트워크 설정 유스케이스를 실행합니다
func (uc *ConfigureNetworkUseCase) Execute(ctx context.Context, input ConfigureNetworkInput) (*ConfigureNetworkOutput, error) {
	// 1. 해당 노드의 모든 활성 인터페이스 조회 (netplan_success 상태 무관)
	allInterfaces, err := uc.repository.GetAllNodeInterfaces(ctx, input.NodeName)
	if err != nil {
		return nil, errors.NewSystemError("노드 인터페이스 조회 실패", err)
	}

	if len(allInterfaces) > 0 {
		uc.logger.WithFields(logrus.Fields{
			"node_name":     input.NodeName,
			"total_interfaces": len(allInterfaces),
		}).Info("처리할 인터페이스 발견")
	}

	var processedCount, failedCount int

	// 2. 각 인터페이스 처리 (생성/수정/동기화)
	for _, iface := range allInterfaces {
		// 인터페이스 이름 생성 (기존에 할당된 이름이 있다면 재사용)
		interfaceName, err := uc.namingService.GenerateNextNameForMAC(iface.MacAddress)
		if err != nil {
			uc.logger.WithError(err).WithField("mac_address", iface.MacAddress).Error("인터페이스 이름 생성 실패")
			failedCount++
			continue
		}

		// Netplan 설정 파일 경로
		configPath := filepath.Join(uc.configurer.GetConfigDir(), fmt.Sprintf("9%d-%s.yaml", extractInterfaceIndex(interfaceName.String()), interfaceName.String()))

		// 파일이 존재하지 않거나, 드리프트가 발생했거나, 아직 설정되지 않은 경우 처리
		if !uc.fileSystem.Exists(configPath) || uc.isDrifted(ctx, iface, configPath) || iface.Status == entities.StatusPending {
			if err := uc.processInterface(ctx, iface, interfaceName); err != nil {
				uc.logger.WithFields(logrus.Fields{
					"interface_id":   iface.ID,
					"interface_name": interfaceName.String(),
					"error":          err,
				}).Error("인터페이스 설정/동기화 실패")
				failedCount++

				// 실패 상태로 업데이트
				if updateErr := uc.repository.UpdateInterfaceStatus(ctx, iface.ID, entities.StatusFailed); updateErr != nil {
					uc.logger.WithError(updateErr).Error("인터페이스 상태 업데이트 실패")
				}
			} else {
				processedCount++
			}
		} else {
			uc.logger.WithFields(logrus.Fields{
				"interface_id":   iface.ID,
				"interface_name": interfaceName.String(),
			}).Debug("인터페이스 설정 최신 상태 유지")
		}
	}

	return &ConfigureNetworkOutput{
		ProcessedCount: processedCount,
		FailedCount:    failedCount,
		TotalCount:     len(allInterfaces),
	}, nil
}

// processInterface는 개별 인터페이스를 처리합니다
func (uc *ConfigureNetworkUseCase) processInterface(ctx context.Context, iface entities.NetworkInterface, interfaceName entities.InterfaceName) error {
	// 1. 유효성 검증
	if err := iface.Validate(); err != nil {
		return errors.NewValidationError("인터페이스 유효성 검증 실패", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   iface.ID,
		"interface_name": interfaceName.String(),
		"mac_address":    iface.MacAddress,
	}).Info("인터페이스 설정 시작")

	// 2. 네트워크 설정 적용
	if err := uc.configurer.Configure(ctx, iface, interfaceName); err != nil {
		// 롤백 시도
		if rollbackErr := uc.rollbacker.Rollback(ctx, interfaceName.String()); rollbackErr != nil {
			uc.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("네트워크 설정 적용 실패", err)
	}

	// 3. 설정 검증
	if err := uc.configurer.Validate(ctx, interfaceName); err != nil {
		// 검증 실패 시 롤백
		if rollbackErr := uc.rollbacker.Rollback(ctx, interfaceName.String()); rollbackErr != nil {
			uc.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("네트워크 설정 검증 실패", err)
	}

	// 4. 성공 상태로 업데이트
	if err := uc.repository.UpdateInterfaceStatus(ctx, iface.ID, entities.StatusConfigured); err != nil {
		return errors.NewSystemError("인터페이스 상태 업데이트 실패", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   iface.ID,
		"interface_name": interfaceName.String(),
	}).Info("인터페이스 설정 성공")

	return nil
}

// isDrifted는 Netplan 설정 파일과 DB 데이터 간의 드리프트를 감지합니다.
func (uc *ConfigureNetworkUseCase) isDrifted(ctx context.Context, dbIface entities.NetworkInterface, configPath string) bool {
	// 파일이 존재하지 않으면 드리프트로 간주 (새로 생성해야 함)
	if !uc.fileSystem.Exists(configPath) {
		uc.logger.WithFields(logrus.Fields{
			"interface_id":   dbIface.ID,
			"mac_address":    dbIface.MacAddress,
			"config_path":    configPath,
		}).Debug("설정 파일이 존재하지 않아 드리프트로 감지")
		return true
	}

	content, err := uc.fileSystem.ReadFile(configPath)
	if err != nil {
		uc.logger.WithError(err).WithField("file", configPath).Warn("Netplan 파일 읽기 실패, 드리프트로 간주")
		return true // 파일 읽기 실패 시 드리프트로 간주하여 재설정 시도
	}

	var netplanData NetplanYAML
	if err := yaml.Unmarshal(content, &netplanData); err != nil {
		uc.logger.WithError(err).WithField("file", configPath).Warn("Netplan YAML 파싱 실패, 드리프트로 간주")
		return true // YAML 파싱 실패 시 드리프트로 간주하여 재설정 시도
	}

	var fileMAC, fileAddress, fileCIDR string
	var fileMTU int

	for _, eth := range netplanData.Network.Ethernets {
		fileMAC = eth.Match.MACAddress
		if len(eth.Addresses) > 0 {
			// Parse the full CIDR from the file
			_, ipNet, err := net.ParseCIDR(eth.Addresses[0])
			if err == nil {
				fileAddress = ipNet.IP.String() // Get just the IP part
				fileCIDR = ipNet.String()       // Get the full CIDR string (e.g., "1.1.1.0/24")
			} else {
				// If parsing fails, use the raw address and an empty CIDR
				fileAddress = eth.Addresses[0]
				fileCIDR = "" // Or some default/error value
			}
		} else {
			// addresses 필드가 없는 경우 (구형 포맷)
			fileAddress = ""
			fileCIDR = ""
		}
		fileMTU = eth.MTU
		break // Assuming one ethernet per file
	}

	// MAC 주소가 일치하지 않으면 심각한 문제이므로 드리프트로 간주
	if fileMAC != dbIface.MacAddress {
		uc.logger.WithFields(logrus.Fields{
			"db_mac":   dbIface.MacAddress,
			"file_mac": fileMAC,
		}).Warn("MAC 주소 불일치, 드리프트로 간주")
		return true
	}

	// 드리프트 감지 조건
	isDrifted := (
		(len(eth.Addresses) == 0 && dbIface.Address != "") ||
		(dbIface.Address != fileAddress) ||
		(dbIface.CIDR != fileCIDR) ||
		(dbIface.MTU != fileMTU)
	)

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   dbIface.ID,
		"mac_address":    dbIface.MacAddress,
		"db_address":     dbIface.Address,
		"file_address":   fileAddress,
		"db_cidr":        dbIface.CIDR,
		"file_cidr":      fileCIDR,
		"db_mtu":         dbIface.MTU,
		"file_mtu":       fileMTU,
		"drift_condition_1": (len(eth.Addresses) == 0 && dbIface.Address != ""),
		"drift_condition_2": (dbIface.Address != fileAddress),
		"drift_condition_3": (dbIface.CIDR != fileCIDR),
		"drift_condition_4": (dbIface.MTU != fileMTU),
		"is_drifted_result": isDrifted,
	}).Debug("드리프트 감지 상세 정보")

	return isDrifted
}

// extractInterfaceIndex는 인터페이스 이름에서 인덱스를 추출합니다
func extractInterfaceIndex(name string) int {
	// multinic0 -> 0, multinic1 -> 1 등
	if strings.HasPrefix(name, "multinic") {
		indexStr := strings.TrimPrefix(name, "multinic")
		if index, err := strconv.Atoi(indexStr); err == nil {
			return index
		}
	}
	return 0
}

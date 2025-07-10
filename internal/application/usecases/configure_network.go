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
	// 1. 대기 중인 인터페이스 조회
	pendingInterfaces, err := uc.repository.GetPendingInterfaces(ctx, input.NodeName)
	if err != nil {
		return nil, errors.NewSystemError("대기 중인 인터페이스 조회 실패", err)
	}

	if len(pendingInterfaces) > 0 {
		uc.logger.WithFields(logrus.Fields{
			"node_name":     input.NodeName,
			"pending_count": len(pendingInterfaces),
		}).Info("대기 중인 인터페이스 발견")
	}

	var processedCount, failedCount int

	// 2. 각 인터페이스 처리
	for _, iface := range pendingInterfaces {
		if err := uc.processInterface(ctx, iface); err != nil {
			uc.logger.WithFields(logrus.Fields{
				"interface_id": iface.ID,
				"error":        err,
			}).Error("인터페이스 설정 실패")
			failedCount++

			// 실패 상태로 업데이트
			if updateErr := uc.repository.UpdateInterfaceStatus(ctx, iface.ID, entities.StatusFailed); updateErr != nil {
				uc.logger.WithError(updateErr).Error("인터페이스 상태 업데이트 실패")
			}
		} else {
			processedCount++
		}
	}

	// 3. 설정된 인터페이스 동기화
	uc.syncConfiguredInterfaces(ctx, input.NodeName)

	return &ConfigureNetworkOutput{
		ProcessedCount: processedCount,
		FailedCount:    failedCount,
		TotalCount:     len(pendingInterfaces),
	}, nil
}

// processInterface는 개별 인터페이스를 처리합니다
func (uc *ConfigureNetworkUseCase) processInterface(ctx context.Context, iface entities.NetworkInterface) error {
	// 1. 유효성 검증
	if err := iface.Validate(); err != nil {
		return errors.NewValidationError("인터페이스 유효성 검증 실패", err)
	}

	// 2. 인터페이스 이름 생성
	interfaceName, err := uc.namingService.GenerateNextName()
	if err != nil {
		return errors.NewSystemError("인터페이스 이름 생성 실패", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   iface.ID,
		"interface_name": interfaceName.String(),
		"mac_address":    iface.MacAddress,
	}).Info("인터페이스 설정 시작")

	// 3. 네트워크 설정 적용
	if err := uc.configurer.Configure(ctx, iface, interfaceName); err != nil {
		// 롤백 시도
		if rollbackErr := uc.rollbacker.Rollback(ctx, interfaceName.String()); rollbackErr != nil {
			uc.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("네트워크 설정 적용 실패", err)
	}

	// 4. 설정 검증
	if err := uc.configurer.Validate(ctx, interfaceName); err != nil {
		// 검증 실패 시 롤백
		if rollbackErr := uc.rollbacker.Rollback(ctx, interfaceName.String()); rollbackErr != nil {
			uc.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("네트워크 설정 검증 실패", err)
	}

	// 5. 성공 상태로 업데이트
	if err := uc.repository.UpdateInterfaceStatus(ctx, iface.ID, entities.StatusConfigured); err != nil {
		return errors.NewSystemError("인터페이스 상태 업데이트 실패", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   iface.ID,
		"interface_name": interfaceName.String(),
	}).Info("인터페이스 설정 성공")

	return nil
}

// NetplanYAML represents the structure of a netplan configuration file
type NetplanYAML struct {
	Network struct {
		Version   int `yaml:"version"`
		Ethernets map[string]struct {
			Match struct {
				MACAddress string `yaml:"macaddress"`
			} `yaml:"match"`
			Addresses []string `yaml:"addresses"`
			MTU       int      `yaml:"mtu"`
		} `yaml:"ethernets"`
	} `yaml:"network"`
}

func (uc *ConfigureNetworkUseCase) syncConfiguredInterfaces(ctx context.Context, nodeName string) {
	// 1. Get configured interfaces from DB
	configuredInterfaces, err := uc.repository.GetConfiguredInterfaces(ctx, nodeName)
	if err != nil {
		uc.logger.WithError(err).Error("설정 완료된 인터페이스 조회 실패")
		return
	}

	if len(configuredInterfaces) == 0 {
		return // No configured interfaces to sync
	}

	uc.logger.WithField("count", len(configuredInterfaces)).Debug("설정 동기화 시작")

	// Create a map for quick lookup
	macToDBIface := make(map[string]entities.NetworkInterface)
	for _, iface := range configuredInterfaces {
		macToDBIface[iface.MacAddress] = iface
	}

	// 2. List netplan files
	files, err := uc.fileSystem.ListFiles("/etc/netplan")
	if err != nil {
		uc.logger.WithError(err).Error("Netplan 설정 파일 목록 조회 실패")
		return
	}

	// 3. Iterate through files, parse, compare, and fix
	for _, file := range files {
		// Basic filter for multinic files
		if !strings.HasPrefix(filepath.Base(file), "9") || !strings.Contains(file, "multinic") {
			continue
		}

		// Read and parse the YAML file
		if !uc.fileSystem.Exists(file) {
			continue
		}
		content, err := uc.fileSystem.ReadFile(file)
		if err != nil {
			uc.logger.WithError(err).WithField("file", file).Warn("Netplan 파일 읽기 실패")
			continue
		}

		var netplanData NetplanYAML
		if err := yaml.Unmarshal(content, &netplanData); err != nil {
			uc.logger.WithError(err).WithField("file", file).Warn("Netplan YAML 파싱 실패")
			continue
		}

		// Extract info from parsed data
		var fileMAC, fileAddress, fileCIDR string
		var fileMTU int
		var interfaceName string

		for name, eth := range netplanData.Network.Ethernets {
			interfaceName = name
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
			}
			fileMTU = eth.MTU

			if fileMAC == "" {
				continue
			}

			// 4. Compare with DB data
			dbIface, ok := macToDBIface[fileMAC]
			if !ok {
				// This file's MAC is not in our DB for this node. It might be an orphan.
				// The orphan deletion logic will handle this.
				continue
			}

			// Check for drift
			// 1. The file is old (no addresses) but the DB expects an IP
			// 2. Or, the file values do not match the DB values
			isDrifted := (len(eth.Addresses) == 0 && dbIface.Address != "") ||
				(dbIface.Address != fileAddress) ||
				(dbIface.CIDR != fileCIDR) ||
				(dbIface.MTU != fileMTU)

			if isDrifted {
				uc.logger.WithFields(logrus.Fields{
					"interface":    interfaceName,
					"db_address":   dbIface.Address,
					"file_address": fileAddress,
					"db_cidr":      dbIface.CIDR,
					"file_cidr":    fileCIDR,
					"db_mtu":       dbIface.MTU,
					"file_mtu":     fileMTU,
				}).Info("설정 변경 감지. 동기화를 시작합니다.")

				ifaceName, err := entities.NewInterfaceName(interfaceName)
				if err != nil {
					uc.logger.WithError(err).Error("잘못된 인터페이스 이름")
					continue
				}

				// Re-apply the configuration from DB data
				if err := uc.configurer.Configure(ctx, dbIface, ifaceName); err != nil {
					uc.logger.WithError(err).WithField("interface", interfaceName).Error("설정 동기화 실패")
				} else {
					uc.logger.WithField("interface", interfaceName).Info("설정 동기화 성공")
				}
			}
			break // Assuming one ethernet per file
		}
	}
}
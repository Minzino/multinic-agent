package usecases

import (
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/interfaces"
	"multinic-agent-v2/internal/domain/services"
	"strings"

	"github.com/sirupsen/logrus"
)

// DeleteNetworkInput은 네트워크 삭제 유스케이스의 입력 데이터입니다
type DeleteNetworkInput struct {
	NodeName string
}

// DeleteNetworkOutput은 네트워크 삭제 유스케이스의 출력 데이터입니다
type DeleteNetworkOutput struct {
	DeletedInterfaces []string
	TotalDeleted      int
	Errors            []error
}

// DeleteNetworkUseCase는 고아 인터페이스를 감지하고 삭제하는 유스케이스입니다
type DeleteNetworkUseCase struct {
	osDetector    interfaces.OSDetector
	rollbacker    interfaces.NetworkRollbacker
	namingService *services.InterfaceNamingService
	logger        *logrus.Logger
}

// NewDeleteNetworkUseCase는 새로운 DeleteNetworkUseCase를 생성합니다
func NewDeleteNetworkUseCase(
	osDetector interfaces.OSDetector,
	rollbacker interfaces.NetworkRollbacker,
	namingService *services.InterfaceNamingService,
	logger *logrus.Logger,
) *DeleteNetworkUseCase {
	return &DeleteNetworkUseCase{
		osDetector:    osDetector,
		rollbacker:    rollbacker,
		namingService: namingService,
		logger:        logger,
	}
}

// Execute는 고아 인터페이스 삭제 유스케이스를 실행합니다
func (uc *DeleteNetworkUseCase) Execute(ctx context.Context, input DeleteNetworkInput) (*DeleteNetworkOutput, error) {
	uc.logger.WithFields(logrus.Fields{
		"node_name": input.NodeName,
	}).Debug("고아 인터페이스 삭제 프로세스 시작")

	osType, err := uc.osDetector.DetectOS()
	if err != nil {
		return nil, fmt.Errorf("OS 감지 실패: %w", err)
	}

	switch osType {
	case interfaces.OSTypeUbuntu:
		return uc.executeNetplanCleanup(ctx, input)
	case interfaces.OSTypeRHEL:
		return uc.executeNmcliCleanup(ctx, input)
	default:
		uc.logger.WithField("os_type", osType).Warn("지원하지 않는 OS 타입이므로 고아 인터페이스 삭제를 건너뜁니다")
		return &DeleteNetworkOutput{}, nil
	}
}

// executeNetplanCleanup은 Netplan (Ubuntu) 환경의 고아 인터페이스를 정리합니다
func (uc *DeleteNetworkUseCase) executeNetplanCleanup(ctx context.Context, input DeleteNetworkInput) (*DeleteNetworkOutput, error) {
	output := &DeleteNetworkOutput{
		DeletedInterfaces: []string{},
		Errors:            []error{},
	}

	orphanedFiles, err := uc.findOrphanedNetplanFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("고아 netplan 파일 조회 실패: %w", err)
	}

	if len(orphanedFiles) == 0 {
		uc.logger.Debug("삭제 대상 고아 netplan 파일이 없습니다")
		return output, nil
	}

	uc.logger.WithFields(logrus.Fields{
		"node_name":      input.NodeName,
		"orphaned_files": len(orphanedFiles),
	}).Info("고아 netplan 파일 감지 완료 - 삭제 프로세스 시작")

	for _, fileName := range orphanedFiles {
		interfaceName := uc.extractInterfaceNameFromFile(fileName)
		if err := uc.deleteNetplanFile(ctx, fileName, interfaceName); err != nil {
			uc.logger.WithFields(logrus.Fields{
				"file_name":      fileName,
				"interface_name": interfaceName,
				"error":          err.Error(),
			}).Error("netplan 파일 삭제 실패")
			output.Errors = append(output.Errors, fmt.Errorf("netplan 파일 %s 삭제 실패: %w", fileName, err))
		} else {
			output.DeletedInterfaces = append(output.DeletedInterfaces, interfaceName)
			output.TotalDeleted++
		}
	}
	return output, nil
}

// executeNmcliCleanup은 nmcli (RHEL) 환경의 고아 인터페이스를 정리합니다
func (uc *DeleteNetworkUseCase) executeNmcliCleanup(ctx context.Context, input DeleteNetworkInput) (*DeleteNetworkOutput, error) {
	output := &DeleteNetworkOutput{
		DeletedInterfaces: []string{},
		Errors:            []error{},
	}

	connections, err := uc.namingService.ListNmcliConnectionNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("nmcli 연결 목록 조회 실패: %w", err)
	}

	var orphanedConnections []string
	for _, connName := range connections {
		if !strings.HasPrefix(connName, "multinic") {
			continue
		}

		exists, err := uc.checkInterfaceExists(ctx, connName)
		if err != nil {
			uc.logger.WithFields(logrus.Fields{
				"connection_name": connName,
				"error":           err,
			}).Warn("인터페이스 존재 여부 확인 중 에러 발생")
			continue
		}

		if !exists {
			orphanedConnections = append(orphanedConnections, connName)
		}
	}

	if len(orphanedConnections) == 0 {
		uc.logger.Debug("삭제 대상 고아 nmcli 연결이 없습니다")
		return output, nil
	}

	uc.logger.WithFields(logrus.Fields{
		"node_name":            input.NodeName,
		"orphaned_connections": orphanedConnections,
	}).Info("고아 nmcli 연결 감지 완료 - 삭제 프로세스 시작")

	for _, connName := range orphanedConnections {
		if err := uc.rollbacker.Rollback(ctx, connName); err != nil {
			uc.logger.WithFields(logrus.Fields{
				"connection_name": connName,
				"error":           err,
			}).Error("nmcli 연결 삭제 실패")
			output.Errors = append(output.Errors, fmt.Errorf("nmcli 연결 %s 삭제 실패: %w", connName, err))
		} else {
			output.DeletedInterfaces = append(output.DeletedInterfaces, connName)
			output.TotalDeleted++
		}
	}
	return output, nil
}

// findOrphanedNetplanFiles는 시스템에 존재하지 않지만 netplan 파일이 남아있는 고아 파일들을 찾습니다
func (uc *DeleteNetworkUseCase) findOrphanedNetplanFiles(ctx context.Context) ([]string, error) {
	var orphanedFiles []string

	// /etc/netplan 디렉토리에서 multinic 관련 파일 스캔
	netplanDir := "/etc/netplan"
	files, err := uc.namingService.ListNetplanFiles(netplanDir)
	if err != nil {
		return nil, fmt.Errorf("netplan 디렉토리 스캔 실패: %w", err)
	}

	for _, fileName := range files {
		// multinic 파일만 처리 (9*-multinic*.yaml 패턴)
		if !uc.isMultinicNetplanFile(fileName) {
			continue
		}

		// 파일명에서 인터페이스 이름 추출
		interfaceName := uc.extractInterfaceNameFromFile(fileName)
		if interfaceName == "" {
			uc.logger.WithField("file_name", fileName).Warn("인터페이스 이름 추출 실패")
			continue
		}

		// 해당 인터페이스가 실제 시스템에 존재하는지 확인
		exists, err := uc.checkInterfaceExists(ctx, interfaceName)
		if err != nil {
			uc.logger.WithFields(logrus.Fields{
				"interface_name": interfaceName,
				"file_name":      fileName,
				"error":          err.Error(),
			}).Warn("인터페이스 존재 여부 확인 실패")
			continue
		}

		if !exists {
			uc.logger.WithFields(logrus.Fields{
				"file_name":      fileName,
				"interface_name": interfaceName,
			}).Info("고아 netplan 파일 발견")
			orphanedFiles = append(orphanedFiles, fileName)
		}
	}

	return orphanedFiles, nil
}

// isMultinicNetplanFile은 파일이 multinic 관련 netplan 파일인지 확인합니다
func (uc *DeleteNetworkUseCase) isMultinicNetplanFile(fileName string) bool {
	// 9*-multinic*.yaml 패턴 매칭
	return strings.Contains(fileName, "multinic") && strings.HasSuffix(fileName, ".yaml") &&
		strings.HasPrefix(fileName, "9") && strings.Contains(fileName, "-")
}

// extractInterfaceNameFromFile은 파일명에서 인터페이스 이름을 추출합니다
func (uc *DeleteNetworkUseCase) extractInterfaceNameFromFile(fileName string) string {
	// 예: "91-multinic1.yaml" -> "multinic1" 또는 "multinic1.yaml" -> "multinic1"
	if !strings.Contains(fileName, "multinic") {
		return ""
	}

	// .yaml 확장자 제거
	nameWithoutExt := strings.TrimSuffix(fileName, ".yaml")

	// "-"로 분할된 경우 (예: "91-multinic1")
	parts := strings.Split(nameWithoutExt, "-")
	for _, part := range parts {
		if strings.HasPrefix(part, "multinic") {
			return part
		}
	}

	// 분할되지 않은 경우 전체가 multinic로 시작하는지 확인 (예: "multinic1")
	if strings.HasPrefix(nameWithoutExt, "multinic") {
		return nameWithoutExt
	}

	return ""
}

// checkInterfaceExists는 해당 인터페이스가 실제 시스템에 존재하는지 확인합니다
func (uc *DeleteNetworkUseCase) checkInterfaceExists(ctx context.Context, interfaceName string) (bool, error) {
	// ip addr show {interface} 명령으로 인터페이스 존재 여부 확인
	_, err := uc.namingService.GetMacAddressForInterface(interfaceName)
	if err != nil {
		// 인터페이스가 존재하지 않으면 에러가 발생
		if strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "정보 조회 실패") {
			return false, nil
		}
		return false, err
	}

	// MAC 주소를 성공적으로 조회했다면 인터페이스가 존재함
	return true, nil
}

// deleteNetplanFile은 고아 netplan 파일을 삭제하고 netplan을 재적용합니다
func (uc *DeleteNetworkUseCase) deleteNetplanFile(ctx context.Context, fileName, interfaceName string) error {
	uc.logger.WithFields(logrus.Fields{
		"file_name":      fileName,
		"interface_name": interfaceName,
	}).Info("고아 netplan 파일 삭제 시작")

	// Rollback 호출로 파일 삭제 및 netplan 재적용
	if err := uc.rollbacker.Rollback(ctx, interfaceName); err != nil {
		return fmt.Errorf("netplan 파일 롤백 실패: %w", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"file_name":      fileName,
		"interface_name": interfaceName,
	}).Info("고아 netplan 파일 삭제 성공")

	return nil
}

package network

import (
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	
	"github.com/sirupsen/logrus"
)

// NetworkManagerFactory는 OS에 따라 적절한 네트워크 관리자를 생성하는 팩토리입니다
type NetworkManagerFactory struct {
	osDetector      interfaces.OSDetector
	commandExecutor interfaces.CommandExecutor
	fileSystem      interfaces.FileSystem
	backupService   interfaces.BackupService
	logger          *logrus.Logger
}

// NewNetworkManagerFactory는 새로운 NetworkManagerFactory를 생성합니다
func NewNetworkManagerFactory(
	osDetector interfaces.OSDetector,
	executor interfaces.CommandExecutor,
	fs interfaces.FileSystem,
	backup interfaces.BackupService,
	logger *logrus.Logger,
) *NetworkManagerFactory {
	return &NetworkManagerFactory{
		osDetector:      osDetector,
		commandExecutor: executor,
		fileSystem:      fs,
		backupService:   backup,
		logger:          logger,
	}
}

// CreateNetworkConfigurer는 OS에 따라 적절한 NetworkConfigurer를 생성합니다
func (f *NetworkManagerFactory) CreateNetworkConfigurer() (interfaces.NetworkConfigurer, error) {
	osType, err := f.osDetector.DetectOS()
	if err != nil {
		return nil, errors.NewSystemError("OS 감지 실패", err)
	}
	
	f.logger.WithField("os_type", osType).Info("OS 타입 감지 완료")
	
	switch osType {
	case interfaces.OSTypeUbuntu:
		return NewNetplanAdapter(
			f.commandExecutor,
			f.fileSystem,
			f.backupService,
			f.logger,
		), nil
		
	case interfaces.OSTypeSUSE:
		return NewWickedAdapter(
			f.commandExecutor,
			f.fileSystem,
			f.backupService,
			f.logger,
		), nil
		
	default:
		return nil, errors.NewSystemError("지원하지 않는 OS 타입", nil)
	}
}

// CreateNetworkRollbacker는 OS에 따라 적절한 NetworkRollbacker를 생성합니다
func (f *NetworkManagerFactory) CreateNetworkRollbacker() (interfaces.NetworkRollbacker, error) {
	// NetworkConfigurer와 동일한 인스턴스를 반환 (같은 구현체가 두 인터페이스를 모두 구현)
	configurer, err := f.CreateNetworkConfigurer()
	if err != nil {
		return nil, err
	}
	
	// 타입 어서션을 통해 NetworkRollbacker로 변환
	if rollbacker, ok := configurer.(interfaces.NetworkRollbacker); ok {
		return rollbacker, nil
	}
	
	return nil, errors.NewSystemError("네트워크 관리자가 롤백 기능을 지원하지 않음", nil)
}
package interfaces

import (
	"context"
	"multinic-agent-v2/internal/domain/entities"
)

// NetworkConfigurer는 네트워크 설정을 적용하는 인터페이스입니다
type NetworkConfigurer interface {
	// Configure는 네트워크 인터페이스를 설정합니다
	Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error
	
	// Validate는 설정된 인터페이스가 정상 작동하는지 검증합니다
	Validate(ctx context.Context, name entities.InterfaceName) error
}

// NetworkRollbacker는 네트워크 설정 롤백을 처리하는 인터페이스입니다
type NetworkRollbacker interface {
	// Rollback은 인터페이스 설정을 이전 상태로 되돌립니다
	Rollback(ctx context.Context, name entities.InterfaceName) error
}

// BackupService는 설정 백업을 관리하는 인터페이스입니다
type BackupService interface {
	// CreateBackup은 현재 설정의 백업을 생성합니다
	CreateBackup(ctx context.Context, interfaceName string, configPath string) error
	
	// RestoreLatestBackup은 가장 최근의 백업을 복원합니다
	RestoreLatestBackup(ctx context.Context, interfaceName string) error
	
	// HasBackup은 백업이 존재하는지 확인합니다
	HasBackup(ctx context.Context, interfaceName string) bool
}
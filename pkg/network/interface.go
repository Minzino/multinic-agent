package network

import "multinic-agent-v2/pkg/db"

// NetworkManager 인터페이스는 여러 네트워크 관리 시스템을 추상화
type NetworkManager interface {
	// ApplyConfiguration은 네트워크 설정을 적용
	ApplyConfiguration(configData []byte, interfaceName string) error
	
	// Rollback은 이전 설정으로 복원
	Rollback(interfaceName string) error
	
	// ValidateInterface는 인터페이스 이름이 유효한지 확인
	ValidateInterface(interfaceName string) bool
	
	// GetType은 네트워크 관리자 타입 반환
	GetType() string
	
	// ConfigureInterface는 DB의 인터페이스 정보로 설정을 생성하고 적용
	ConfigureInterface(iface db.MultiInterface, interfaceName string) error
}
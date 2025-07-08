package services

import (
	"fmt"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/interfaces"
)

// InterfaceNamingService는 네트워크 인터페이스 이름을 관리하는 도메인 서비스입니다
type InterfaceNamingService struct {
	fileSystem interfaces.FileSystem
}

// NewInterfaceNamingService는 새로운 InterfaceNamingService를 생성합니다
func NewInterfaceNamingService(fs interfaces.FileSystem) *InterfaceNamingService {
	return &InterfaceNamingService{
		fileSystem: fs,
	}
}

// GenerateNextName은 사용 가능한 다음 인터페이스 이름을 생성합니다
func (s *InterfaceNamingService) GenerateNextName() (entities.InterfaceName, error) {
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("multinic%d", i)
		
		// 이미 사용 중인지 확인
		if !s.isInterfaceInUse(name) {
			return entities.NewInterfaceName(name)
		}
	}
	
	return entities.InterfaceName{}, fmt.Errorf("사용 가능한 인터페이스 이름이 없습니다 (multinic0-9 모두 사용 중)")
}

// isInterfaceInUse는 인터페이스가 이미 사용 중인지 확인합니다
func (s *InterfaceNamingService) isInterfaceInUse(name string) bool {
	// /sys/class/net 디렉토리에서 인터페이스 확인
	return s.fileSystem.Exists(fmt.Sprintf("/sys/class/net/%s", name))
}
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

// GetCurrentMultinicInterfaces는 현재 시스템에 생성된 모든 multinic 인터페이스를 반환합니다
func (s *InterfaceNamingService) GetCurrentMultinicInterfaces() []entities.InterfaceName {
	var interfaces []entities.InterfaceName
	
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("multinic%d", i)
		if s.isInterfaceInUse(name) {
			if interfaceName, err := entities.NewInterfaceName(name); err == nil {
				interfaces = append(interfaces, interfaceName)
			}
		}
	}
	
	return interfaces
}

// GetMacAddressForInterface는 특정 인터페이스의 MAC 주소를 조회합니다
func (s *InterfaceNamingService) GetMacAddressForInterface(interfaceName string) (string, error) {
	macPath := fmt.Sprintf("/sys/class/net/%s/address", interfaceName)
	
	if !s.fileSystem.Exists(macPath) {
		return "", fmt.Errorf("인터페이스 %s의 MAC 주소 파일이 존재하지 않습니다", interfaceName)
	}
	
	macBytes, err := s.fileSystem.ReadFile(macPath)
	if err != nil {
		return "", fmt.Errorf("인터페이스 %s의 MAC 주소 읽기 실패: %w", interfaceName, err)
	}
	
	// 줄바꿈 문자 제거
	macAddress := string(macBytes)
	if len(macAddress) > 0 && macAddress[len(macAddress)-1] == '\n' {
		macAddress = macAddress[:len(macAddress)-1]
	}
	
	return macAddress, nil
}
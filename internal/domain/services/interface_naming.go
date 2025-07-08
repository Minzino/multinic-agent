package services

import (
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/interfaces"
	"regexp"
	"time"
)

// InterfaceNamingService는 네트워크 인터페이스 이름을 관리하는 도메인 서비스입니다
type InterfaceNamingService struct {
	fileSystem      interfaces.FileSystem
	commandExecutor interfaces.CommandExecutor
}

// NewInterfaceNamingService는 새로운 InterfaceNamingService를 생성합니다
func NewInterfaceNamingService(fs interfaces.FileSystem, executor interfaces.CommandExecutor) *InterfaceNamingService {
	return &InterfaceNamingService{
		fileSystem:      fs,
		commandExecutor: executor,
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

// GetCurrentMultinicInterfaces는 현재 시스템에 존재하는 모든 multinic 인터페이스를 반환합니다
// ip a 명령어를 통해 실제 네트워크 인터페이스를 확인합니다
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

// GetMacAddressForInterface는 특정 인터페이스의 MAC 주소를 ip 명령어로 조회합니다
func (s *InterfaceNamingService) GetMacAddressForInterface(interfaceName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// ip addr show 명령어로 특정 인터페이스 정보 조회
	output, err := s.commandExecutor.ExecuteWithTimeout(ctx, 10*time.Second, "ip", "addr", "show", interfaceName)
	if err != nil {
		return "", fmt.Errorf("인터페이스 %s 정보 조회 실패: %w", interfaceName, err)
	}
	
	// MAC 주소 추출 (예: "link/ether fa:16:3e:00:be:63 brd ff:ff:ff:ff:ff:ff")
	macRegex := regexp.MustCompile(`link/ether\s+([a-fA-F0-9:]{17})`)
	matches := macRegex.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("인터페이스 %s에서 MAC 주소를 찾을 수 없습니다", interfaceName)
	}
	
	return matches[1], nil
}
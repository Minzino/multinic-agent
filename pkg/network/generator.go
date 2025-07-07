package network

import (
	"fmt"
	"net"
	"strings"

	"multinic-agent-v2/pkg/db"
)

// InterfaceGenerator는 네트워크 인터페이스 설정을 생성
type InterfaceGenerator struct {
	usedNames map[string]bool
}

func NewInterfaceGenerator() *InterfaceGenerator {
	return &InterfaceGenerator{
		usedNames: make(map[string]bool),
	}
}

// GenerateInterfaceName은 multinic0~9 형식의 인터페이스 이름을 생성
func (g *InterfaceGenerator) GenerateInterfaceName() (string, error) {
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("multinic%d", i)
		if !g.usedNames[name] {
			g.usedNames[name] = true
			return name, nil
		}
	}
	return "", fmt.Errorf("사용 가능한 인터페이스 이름이 없음 (최대 10개)")
}

// GenerateNetplanConfig는 Netplan 설정을 생성 (인터페이스 활성화만)
func (g *InterfaceGenerator) GenerateNetplanConfig(iface db.MultiInterface, interfaceName string) ([]byte, error) {
	if err := validateNetworkConfig(iface); err != nil {
		return nil, fmt.Errorf("네트워크 설정 검증 실패: %w", err)
	}

	config := fmt.Sprintf(`# This is the network config written by 'multinic-agent'
network:
  ethernets:
    %s:
      dhcp4: false
      match:
        macaddress: %s
      set-name: %s
      mtu: 1500
  version: 2`, interfaceName, strings.ToLower(iface.MacAddress), interfaceName)

	return []byte(config), nil
}

// GenerateWickedConfig는 SUSE Wicked 설정을 생성 (인터페이스 활성화만)
func (g *InterfaceGenerator) GenerateWickedConfig(iface db.MultiInterface, interfaceName string) ([]byte, error) {
	if err := validateNetworkConfig(iface); err != nil {
		return nil, fmt.Errorf("네트워크 설정 검증 실패: %w", err)
	}

	config := fmt.Sprintf(`# Interface: %s
# MAC Address: %s
# This config written by 'multinic-agent'
STARTMODE='auto'
BOOTPROTO='none'
LLADDR='%s'
MTU='1500'`, interfaceName, strings.ToLower(iface.MacAddress), strings.ToLower(iface.MacAddress))

	return []byte(config), nil
}

// validateNetworkConfig는 네트워크 설정의 유효성을 검사 (기본 검증만)
func validateNetworkConfig(iface db.MultiInterface) error {
	// MAC 주소 검증
	if _, err := net.ParseMAC(iface.MacAddress); err != nil {
		return fmt.Errorf("잘못된 MAC 주소: %s", iface.MacAddress)
	}

	return nil
}
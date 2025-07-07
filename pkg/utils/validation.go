package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// 인터페이스 이름 패턴: multinic0 ~ multinic9
	interfacePattern = regexp.MustCompile(`^multinic[0-9]$`)
	
	// 호스트네임 패턴
	hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]*[a-zA-Z0-9]$`)
)

// ValidateInterfaceName은 인터페이스 이름이 유효한지 검증
func ValidateInterfaceName(name string) error {
	if name == "" {
		return fmt.Errorf("인터페이스 이름이 비어있음")
	}
	
	if !interfacePattern.MatchString(name) {
		return fmt.Errorf("잘못된 인터페이스 이름 형식: %s (multinic0~9 형식이어야 함)", name)
	}
	
	return nil
}

// ValidateHostname은 호스트네임이 유효한지 검증
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("호스트네임이 비어있음")
	}
	
	if len(hostname) > 253 {
		return fmt.Errorf("호스트네임이 너무 김: %d자 (최대 253자)", len(hostname))
	}
	
	if !hostnamePattern.MatchString(hostname) {
		return fmt.Errorf("잘못된 호스트네임 형식: %s", hostname)
	}
	
	return nil
}

// ValidateNetplanConfig은 Netplan 설정이 유효한지 기본 검증
func ValidateNetplanConfig(config []byte) error {
	if len(config) == 0 {
		return fmt.Errorf("빈 설정")
	}
	
	configStr := string(config)
	
	// 기본 YAML 구조 확인
	if !strings.Contains(configStr, "network:") {
		return fmt.Errorf("network 섹션이 없음")
	}
	
	// 위험한 설정 확인
	if strings.Contains(configStr, "eth0") || strings.Contains(configStr, "ens") {
		return fmt.Errorf("보호된 인터페이스 설정 포함")
	}
	
	return nil
}

// ValidateDatabaseConfig은 데이터베이스 설정이 유효한지 검증
func ValidateDatabaseConfig(host, port, user, password, database string) error {
	if host == "" {
		return fmt.Errorf("데이터베이스 호스트가 비어있음")
	}
	
	if port == "" {
		return fmt.Errorf("데이터베이스 포트가 비어있음")
	}
	
	if user == "" {
		return fmt.Errorf("데이터베이스 사용자가 비어있음")
	}
	
	if password == "" {
		return fmt.Errorf("데이터베이스 패스워드가 비어있음")
	}
	
	if database == "" {
		return fmt.Errorf("데이터베이스 이름이 비어있음")
	}
	
	return nil
}
package entities

import (
	"errors"
	"regexp"
)

// NetworkInterface는 네트워크 인터페이스의 도메인 엔티티입니다
type NetworkInterface struct {
	ID               int
	MacAddress       string
	AttachedNodeName string
	Status           InterfaceStatus
	Address          string // IP 주소 (e.g., "192.168.1.10/24")
	MTU              int    // MTU 값
}

// InterfaceStatus는 인터페이스의 상태를 나타냅니다
type InterfaceStatus int

const (
	StatusPending InterfaceStatus = iota
	StatusConfigured
	StatusFailed
)

// InterfaceName은 multinic 인터페이스 이름을 나타내는 값 객체입니다
type InterfaceName struct {
	value string
}

var (
	ErrInvalidMacAddress    = errors.New("유효하지 않은 MAC 주소 형식")
	ErrInvalidInterfaceName = errors.New("유효하지 않은 인터페이스 이름")
	ErrInvalidNodeName      = errors.New("유효하지 않은 노드 이름")
)

// NewInterfaceName은 새로운 인터페이스 이름을 생성합니다
func NewInterfaceName(name string) (InterfaceName, error) {
	if !isValidInterfaceName(name) {
		return InterfaceName{}, ErrInvalidInterfaceName
	}
	return InterfaceName{value: name}, nil
}

// String은 인터페이스 이름의 문자열 표현을 반환합니다
func (n InterfaceName) String() string {
	return n.value
}

// Validate는 NetworkInterface의 유효성을 검증합니다
func (ni *NetworkInterface) Validate() error {
	if !isValidMacAddress(ni.MacAddress) {
		return ErrInvalidMacAddress
	}
	if ni.AttachedNodeName == "" {
		return ErrInvalidNodeName
	}
	return nil
}

// IsPending은 인터페이스가 설정 대기 중인지 확인합니다
func (ni *NetworkInterface) IsPending() bool {
	return ni.Status == StatusPending
}

// MarkAsConfigured는 인터페이스를 설정 완료 상태로 변경합니다
func (ni *NetworkInterface) MarkAsConfigured() {
	ni.Status = StatusConfigured
}

// MarkAsFailed는 인터페이스를 설정 실패 상태로 변경합니다
func (ni *NetworkInterface) MarkAsFailed() {
	ni.Status = StatusFailed
}

// isValidMacAddress는 MAC 주소의 유효성을 검증합니다
func isValidMacAddress(mac string) bool {
	macRegex := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	return macRegex.MatchString(mac)
}

// isValidInterfaceName은 인터페이스 이름의 유효성을 검증합니다
func isValidInterfaceName(name string) bool {
	matched, _ := regexp.MatchString(`^multinic[0-9]$`, name)
	return matched
}

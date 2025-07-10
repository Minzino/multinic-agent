package entities

import (
	"errors"
	"regexp"
)

// NetworkInterface is a domain entity for network interface
type NetworkInterface struct {
	ID               int
	MacAddress       string
	AttachedNodeName string
	Status           InterfaceStatus
	Address          string // IP address (e.g., "192.168.1.10")
	CIDR             string // CIDR (e.g., "192.168.1.0/24")
	MTU              int    // MTU value
}

// InterfaceStatus represents the state of an interface
type InterfaceStatus int

const (
	StatusPending InterfaceStatus = iota
	StatusConfigured
	StatusFailed
)

// InterfaceName is a value object representing multinic interface name
type InterfaceName struct {
	value string
}

var (
	ErrInvalidMacAddress    = errors.New("invalid MAC address format")
	ErrInvalidInterfaceName = errors.New("invalid interface name")
	ErrInvalidNodeName      = errors.New("invalid node name")
)

// NewInterfaceName creates a new interface name
func NewInterfaceName(name string) (InterfaceName, error) {
	if !isValidInterfaceName(name) {
		return InterfaceName{}, ErrInvalidInterfaceName
	}
	return InterfaceName{value: name}, nil
}

// String returns the string representation of interface name
func (n InterfaceName) String() string {
	return n.value
}

// Validate verifies the validity of NetworkInterface
func (ni *NetworkInterface) Validate() error {
	if !isValidMacAddress(ni.MacAddress) {
		return ErrInvalidMacAddress
	}
	if ni.AttachedNodeName == "" {
		return ErrInvalidNodeName
	}
	return nil
}

// IsPending checks if the interface is pending configuration
func (ni *NetworkInterface) IsPending() bool {
	return ni.Status == StatusPending
}

// MarkAsConfigured changes the interface to configured state
func (ni *NetworkInterface) MarkAsConfigured() {
	ni.Status = StatusConfigured
}

// MarkAsFailed changes the interface to failed state
func (ni *NetworkInterface) MarkAsFailed() {
	ni.Status = StatusFailed
}

// isValidMacAddress validates MAC address format
func isValidMacAddress(mac string) bool {
	macRegex := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	return macRegex.MatchString(mac)
}

// isValidInterfaceName validates interface name format
func isValidInterfaceName(name string) bool {
	matched, _ := regexp.MatchString(`^multinic[0-9]$`, name)
	return matched
}

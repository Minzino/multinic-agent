package network

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewNetworkManager(t *testing.T) {
	logger := logrus.New()
	
	manager, err := NewNetworkManager(logger)
	if err != nil {
		t.Fatalf("NewNetworkManager() error = %v", err)
	}
	
	if manager == nil {
		t.Fatal("NewNetworkManager() returned nil")
	}
	
	// OS에 따라 다른 타입이 반환되는지 확인
	managerType := manager.GetType()
	if managerType != "netplan" && managerType != "wicked" {
		t.Errorf("Unexpected manager type: %s", managerType)
	}
}

func TestDetectOS(t *testing.T) {
	osType := detectOS()
	
	// OS 타입이 유효한지 확인
	if osType != Ubuntu && osType != SUSE {
		t.Errorf("detectOS() returned unexpected type: %v", osType)
	}
}
package network

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// OSType 정의
type OSType string

const (
	Ubuntu OSType = "ubuntu"
	SUSE   OSType = "suse"
)

// Factory는 OS 타입에 따라 적절한 NetworkManager를 생성
func NewNetworkManager(logger *logrus.Logger) (NetworkManager, error) {
	osType := detectOS()
	
	logger.WithField("os_type", osType).Info("OS 타입 감지")

	switch osType {
	case Ubuntu:
		return NewNetplanManager(logger), nil
	case SUSE:
		return NewWickedManager(logger), nil
	default:
		return nil, fmt.Errorf("지원하지 않는 OS 타입: %s", osType)
	}
}

// detectOS는 현재 시스템의 OS 타입을 감지
func detectOS() OSType {
	// Ubuntu 감지
	if _, err := os.Stat("/etc/lsb-release"); err == nil {
		content, err := os.ReadFile("/etc/lsb-release")
		if err == nil && strings.Contains(string(content), "Ubuntu") {
			return Ubuntu
		}
	}

	// SUSE 감지
	if _, err := os.Stat("/etc/suse-release"); err == nil {
		return SUSE
	}
	if _, err := os.Stat("/etc/SUSE-brand"); err == nil {
		return SUSE
	}
	if _, err := os.Stat("/etc/os-release"); err == nil {
		content, err := os.ReadFile("/etc/os-release")
		if err == nil && strings.Contains(string(content), "SUSE") {
			return SUSE
		}
	}

	// 기본값은 Ubuntu
	return Ubuntu
}
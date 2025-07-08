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
	// /etc/issue 파일로 OS 감지
	if content, err := os.ReadFile("/etc/issue"); err == nil {
		contentStr := strings.ToLower(string(content))
		
		if strings.Contains(contentStr, "ubuntu") {
			return Ubuntu
		}
		
		if strings.Contains(contentStr, "suse") {
			return SUSE
		}
	}

	// 기본값은 Ubuntu
	return Ubuntu
}
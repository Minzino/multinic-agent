package adapters

import (
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"strings"
)

// RealOSDetector는 실제 OS를 감지하는 OSDetector 구현체입니다
type RealOSDetector struct {
	fileSystem interfaces.FileSystem
}

// NewRealOSDetector는 새로운 RealOSDetector를 생성합니다
func NewRealOSDetector(fs interfaces.FileSystem) interfaces.OSDetector {
	return &RealOSDetector{
		fileSystem: fs,
	}
}

// DetectOS는 현재 운영체제 타입을 반환합니다
func (d *RealOSDetector) DetectOS() (interfaces.OSType, error) {
	// /etc/issue 파일로 OS 감지
	content, err := d.fileSystem.ReadFile("/etc/issue")
	if err != nil {
		return "", errors.NewSystemError("OS 감지 실패", err)
	}
	
	contentStr := strings.ToLower(string(content))
	
	if strings.Contains(contentStr, "ubuntu") {
		return interfaces.OSTypeUbuntu, nil
	}
	
	if strings.Contains(contentStr, "suse") {
		return interfaces.OSTypeSUSE, nil
	}
	
	// 기본값은 Ubuntu
	return interfaces.OSTypeUbuntu, nil
}
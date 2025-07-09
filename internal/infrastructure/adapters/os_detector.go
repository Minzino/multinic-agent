package adapters

import (
	"bufio"
	"fmt"
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
	// /etc/os-release 파일을 먼저 시도
	releaseInfo, err := d.parseOSRelease()
	if err != nil {
		return "", errors.NewSystemError("OS 감지 실패: /etc/os-release 파일을 읽을 수 없음", err)
	}

	id, ok := releaseInfo["ID"]
	if !ok {
		return "", errors.NewSystemError("OS 감지 실패: /etc/os-release 파일에 ID 필드가 없음", nil)
	}

	idLike, _ := releaseInfo["ID_LIKE"]

	// OS 타입 결정 로직
	if id == "ubuntu" {
		return interfaces.OSTypeUbuntu, nil
	} else if id == "sles" || id == "suse" || strings.Contains(idLike, "suse") {
		return interfaces.OSTypeSUSE, nil
	} else if id == "rhel" || id == "centos" || id == "rocky" || id == "almalinux" || strings.Contains(idLike, "fedora") {
		return interfaces.OSTypeRHEL, nil
	}

	// 알려진 ID와 일치하지 않으면 에러 반환
	return "", errors.NewSystemError(fmt.Sprintf("지원하지 않는 OS 타입. ID: '%s', ID_LIKE: '%s'", id, idLike), nil)
}

// parseOSRelease는 /etc/os-release 파일을 파싱하여 map으로 반환합니다.
func (d *RealOSDetector) parseOSRelease() (map[string]string, error) {
	content, err := d.fileSystem.ReadFile("/etc/os-release")
	if err != nil {
		return nil, err
	}

	releaseInfo := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
			releaseInfo[key] = value
		}
	}

	return releaseInfo, nil
}

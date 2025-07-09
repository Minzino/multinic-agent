package network

import (
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// SuseLegacyAdapter는 SUSE 9.4 (ifup/down)을 사용하는 NetworkConfigurer 및 NetworkRollbacker 구현체입니다
type SuseLegacyAdapter struct {
	commandExecutor interfaces.CommandExecutor
	fileSystem      interfaces.FileSystem
	logger          *logrus.Logger
	configDir       string
}

// NewSuseLegacyAdapter는 새로운 SuseLegacyAdapter를 생성합니다
func NewSuseLegacyAdapter(
	executor interfaces.CommandExecutor,
	fs interfaces.FileSystem,
	logger *logrus.Logger,
) *SuseLegacyAdapter {
	return &SuseLegacyAdapter{
		commandExecutor: executor,
		fileSystem:      fs,
		logger:          logger,
		configDir:       "/etc/sysconfig/network",
	}
}

// Configure는 네트워크 인터페이스를 설정합니다
func (a *SuseLegacyAdapter) Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error {
	// 설정 파일 경로 생성
	configPath := filepath.Join(a.configDir, fmt.Sprintf("ifcfg-%s", name.String()))

	// 백업 로직 제거 - 기존 설정 파일이 있으면 덮어쓰기

	// ifcfg 설정 생성
	configContent := a.generateIfcfgConfig(iface, name.String())

	// 설정 파일 저장
	if err := a.fileSystem.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return errors.NewSystemError("ifcfg 설정 파일 저장 실패", err)
	}

	a.logger.WithFields(logrus.Fields{
		"interface":   name.String(),
		"config_path": configPath,
	}).Info("ifcfg 설정 파일 생성 완료")

	// 인터페이스 활성화
	if err := a.activateInterface(ctx, name.String()); err != nil {
		// 실패 시 롤백
		if rollbackErr := a.Rollback(ctx, name.String()); rollbackErr != nil {
			a.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("인터페이스 활성화 실패", err)
	}

	return nil
}

// Validate는 설정된 인터페이스가 정상 작동하는지 검증합니다
func (a *SuseLegacyAdapter) Validate(ctx context.Context, name entities.InterfaceName) error {
	// 인터페이스가 존재하는지 확인
	interfacePath := fmt.Sprintf("/sys/class/net/%s", name.String())
	if !a.fileSystem.Exists(interfacePath) {
		return errors.NewValidationError("네트워크 인터페이스가 존재하지 않음", nil)
	}

	// 인터페이스가 UP 상태인지 확인
	_, err := a.commandExecutor.ExecuteWithTimeout(ctx, 10*time.Second, "ip", "link", "show", name.String(), "up")
	if err != nil {
		return errors.NewValidationError("네트워크 인터페이스가 UP 상태가 아님", err)
	}

	return nil
}

// Rollback은 인터페이스 설정을 이전 상태로 되돌립니다
func (a *SuseLegacyAdapter) Rollback(ctx context.Context, name string) error {
	configPath := filepath.Join(a.configDir, fmt.Sprintf("ifcfg-%s", name))

	// 인터페이스 비활성화
	if err := a.deactivateInterface(ctx, name); err != nil {
		a.logger.WithError(err).Warn("인터페이스 비활성화 실패")
	}

	// 설정 파일 제거
	if a.fileSystem.Exists(configPath) {
		if err := a.fileSystem.Remove(configPath); err != nil {
			return errors.NewSystemError("설정 파일 제거 실패", err)
		}
	}

	// 백업 복원 로직 제거 - 단순히 설정 파일만 제거

	a.logger.WithField("interface", name).Info("네트워크 설정 롤백 완료")
	return nil
}

// activateInterface는 ifup/down을 사용하여 인터페이스를 활성화합니다
func (a *SuseLegacyAdapter) activateInterface(ctx context.Context, interfaceName string) error {
	_, err := a.commandExecutor.ExecuteWithTimeout(
		ctx,
		30*time.Second,
		"ifup", interfaceName,
	)
	return err
}

// deactivateInterface는 ifdown을 사용하여 인터페이스를 비활성화합니다
func (a *SuseLegacyAdapter) deactivateInterface(ctx context.Context, interfaceName string) error {
	_, err := a.commandExecutor.ExecuteWithTimeout(
		ctx,
		30*time.Second,
		"ifdown", interfaceName,
	)
	return err
}

// generateIfcfgConfig는 ifcfg 설정 파일 내용을 생성합니다
func (a *SuseLegacyAdapter) generateIfcfgConfig(iface entities.NetworkInterface, interfaceName string) string {
	var config strings.Builder

	// 기본 설정
	config.WriteString("STARTMODE=auto\n")
	config.WriteString("BOOTPROTO=none\n")
	config.WriteString(fmt.Sprintf("LLADDR=%s\n", iface.MacAddress))
	config.WriteString("MTU=1500\n")

	return config.String()
}
package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"multinic-agent-v2/pkg/db"
	"github.com/sirupsen/logrus"
)

const (
	wickedConfigDir = "/etc/sysconfig/network"
	wickedBackupDir = "/var/lib/multinic/wicked-backups"
)

// WickedManager는 SUSE의 Wicked를 사용한 네트워크 관리
type WickedManager struct {
	logger *logrus.Logger
}

func NewWickedManager(logger *logrus.Logger) *WickedManager {
	return &WickedManager{
		logger: logger,
	}
}

func (m *WickedManager) GetType() string {
	return "wicked"
}

func (m *WickedManager) ValidateInterface(interfaceName string) bool {
	return strings.HasPrefix(interfaceName, "multinic") && 
		len(interfaceName) == 9 && 
		interfaceName[8] >= '0' && 
		interfaceName[8] <= '9'
}

func (m *WickedManager) ApplyConfiguration(configData []byte, interfaceName string) error {
	if !m.ValidateInterface(interfaceName) {
		return fmt.Errorf("잘못된 인터페이스 이름: %s", interfaceName)
	}

	// SUSE에서는 ifcfg-<interface> 형식 사용
	configPath := filepath.Join(wickedConfigDir, "ifcfg-"+interfaceName)

	// 백업 생성
	if err := m.createBackup(configPath); err != nil {
		m.logger.WithError(err).Warn("백업 생성 실패")
	}

	// 설정 파일 쓰기
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("설정 파일 쓰기 실패: %w", err)
	}

	// 네트워크 서비스 재시작
	if err := m.restartNetwork(interfaceName); err != nil {
		m.Rollback(interfaceName)
		return fmt.Errorf("네트워크 재시작 실패: %w", err)
	}

	m.logger.WithField("interface", interfaceName).Info("Wicked 설정 적용 성공")
	return nil
}

func (m *WickedManager) restartNetwork(interfaceName string) error {
	// 인터페이스만 재시작
	cmd := exec.Command("wicked", "ifup", interfaceName)
	
	done := make(chan error, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.WithField("output", string(output)).Error("wicked ifup 실패")
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("wicked ifup 시간 초과")
	}
}

func (m *WickedManager) createBackup(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(wickedBackupDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	backupPath := filepath.Join(wickedBackupDir, filepath.Base(configPath)+"."+timestamp)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, data, 0644)
}

func (m *WickedManager) Rollback(interfaceName string) error {
	if !m.ValidateInterface(interfaceName) {
		return fmt.Errorf("잘못된 인터페이스 이름: %s", interfaceName)
	}

	configPath := filepath.Join(wickedConfigDir, "ifcfg-"+interfaceName)

	// 가장 최근 백업 찾기
	pattern := filepath.Join(wickedBackupDir, filepath.Base(configPath)+".*")
	backupFiles, err := filepath.Glob(pattern)
	if err != nil || len(backupFiles) == 0 {
		// 백업이 없으면 인터페이스 다운
		cmd := exec.Command("wicked", "ifdown", interfaceName)
		cmd.Run()
		os.Remove(configPath)
		m.logger.Info("백업 없음, 인터페이스 제거")
		return nil
	}

	// 가장 최근 백업 선택
	latestBackup := backupFiles[len(backupFiles)-1]
	data, err := os.ReadFile(latestBackup)
	if err != nil {
		return fmt.Errorf("백업 파일 읽기 실패: %w", err)
	}

	// 설정 복원
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("설정 복원 실패: %w", err)
	}

	// 네트워크 재시작
	if err := m.restartNetwork(interfaceName); err != nil {
		return fmt.Errorf("백업 네트워크 재시작 실패: %w", err)
	}

	m.logger.WithField("interface", interfaceName).Info("설정 롤백 성공")
	return nil
}

func (m *WickedManager) ConfigureInterface(iface db.MultiInterface, interfaceName string) error {
	generator := NewInterfaceGenerator()
	
	// Wicked 설정 생성
	configData, err := generator.GenerateWickedConfig(iface, interfaceName)
	if err != nil {
		return fmt.Errorf("Wicked 설정 생성 실패: %w", err)
	}
	
	// 설정 적용
	return m.ApplyConfiguration(configData, interfaceName)
}
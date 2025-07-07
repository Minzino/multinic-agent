package netplan

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	netplanDir = "/etc/netplan"
	backupDir  = "/var/lib/multinic/backups"
	timeout    = 30 * time.Second
)

type Manager struct {
	logger *logrus.Logger
}

type NetplanConfig struct {
	Network Network `yaml:"network"`
}

type Network struct {
	Version   int                    `yaml:"version"`
	Renderer  string                 `yaml:"renderer,omitempty"`
	Ethernets map[string]interface{} `yaml:"ethernets"`
}

func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

func (m *Manager) ApplyConfiguration(configData []byte, interfaceName string) error {
	if !strings.HasPrefix(interfaceName, "multinic") {
		return fmt.Errorf("잘못된 인터페이스 이름: %s", interfaceName)
	}

	index := strings.TrimPrefix(interfaceName, "multinic")
	filename := fmt.Sprintf("9%s-%s.yaml", index, interfaceName)
	configPath := filepath.Join(netplanDir, filename)

	if err := m.createBackup(configPath); err != nil {
		m.logger.WithError(err).Warn("백업 생성 실패")
	}

	if err := ioutil.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("설정 파일 쓰기 실패: %w", err)
	}

	if err := m.applyNetplan(); err != nil {
		m.rollback(configPath)
		return fmt.Errorf("netplan apply 실패: %w", err)
	}

	return nil
}

func (m *Manager) applyNetplan() error {
	cmd := exec.Command("netplan", "apply")
	
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("netplan apply 시간 초과")
	}
}

func (m *Manager) createBackup(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	backupPath := filepath.Join(backupDir, filepath.Base(configPath)+"."+timestamp)

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(backupPath, data, 0644)
}

func (m *Manager) rollback(configPath string) {
	backupFiles, err := filepath.Glob(filepath.Join(backupDir, filepath.Base(configPath)+".*"))
	if err != nil || len(backupFiles) == 0 {
		m.logger.Error("백업 파일을 찾을 수 없음")
		return
	}

	latestBackup := backupFiles[len(backupFiles)-1]
	data, err := ioutil.ReadFile(latestBackup)
	if err != nil {
		m.logger.WithError(err).Error("백업 파일 읽기 실패")
		return
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		m.logger.WithError(err).Error("설정 복원 실패")
		return
	}

	if err := m.applyNetplan(); err != nil {
		m.logger.WithError(err).Error("백업 netplan apply 실패")
	}
}

func (m *Manager) IsRunningOnSUSE() bool {
	if _, err := os.Stat("/etc/suse-release"); err == nil {
		return true
	}
	if _, err := os.Stat("/etc/SUSE-brand"); err == nil {
		return true
	}
	return false
}

func (m *Manager) ApplyNetworkManager() error {
	cmd := exec.Command("systemctl", "restart", "NetworkManager")
	return cmd.Run()
}
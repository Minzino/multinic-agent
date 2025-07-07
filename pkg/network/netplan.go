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
	netplanDir = "/etc/netplan"
	backupDir  = "/var/lib/multinic/backups"
	timeout    = 30 * time.Second
)

// NetplanManager는 Ubuntu의 Netplan을 사용한 네트워크 관리
type NetplanManager struct {
	logger *logrus.Logger
}

func NewNetplanManager(logger *logrus.Logger) *NetplanManager {
	return &NetplanManager{
		logger: logger,
	}
}

func (m *NetplanManager) GetType() string {
	return "netplan"
}

func (m *NetplanManager) ValidateInterface(interfaceName string) bool {
	return strings.HasPrefix(interfaceName, "multinic") && 
		len(interfaceName) == 9 && 
		interfaceName[8] >= '0' && 
		interfaceName[8] <= '9'
}

func (m *NetplanManager) ApplyConfiguration(configData []byte, interfaceName string) error {
	if !m.ValidateInterface(interfaceName) {
		return fmt.Errorf("잘못된 인터페이스 이름: %s", interfaceName)
	}

	// multinic0 -> 90-multinic0.yaml
	index := interfaceName[8:] // '0' ~ '9'
	filename := fmt.Sprintf("9%s-%s.yaml", index, interfaceName)
	configPath := filepath.Join(netplanDir, filename)

	// 백업 생성
	if err := m.createBackup(configPath); err != nil {
		m.logger.WithError(err).Warn("백업 생성 실패")
	}

	// 설정 파일 쓰기
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("설정 파일 쓰기 실패: %w", err)
	}

	// netplan apply 실행
	if err := m.applyNetplan(); err != nil {
		m.Rollback(interfaceName)
		return fmt.Errorf("netplan apply 실패: %w", err)
	}

	m.logger.WithField("interface", interfaceName).Info("Netplan 설정 적용 성공")
	return nil
}

func (m *NetplanManager) applyNetplan() error {
	// 먼저 netplan try로 안전하게 테스트 (120초 타임아웃)
	m.logger.Info("netplan try로 설정 테스트 중...")
	cmd := exec.Command("/usr/sbin/netplan", "try", "--timeout=120")
	
	done := make(chan error, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.WithField("output", string(output)).Error("netplan try 실패")
		} else {
			m.logger.WithField("output", string(output)).Info("netplan try 성공")
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("netplan try 실패: %w", err)
		}
		
		// try가 성공하면 실제 적용
		m.logger.Info("netplan apply 실행 중...")
		applyCmd := exec.Command("/usr/sbin/netplan", "apply")
		if output, err := applyCmd.CombinedOutput(); err != nil {
			m.logger.WithField("output", string(output)).Error("netplan apply 실패")
			return fmt.Errorf("netplan apply 실패: %w", err)
		}
		
		return nil
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("netplan try 시간 초과")
	}
}

func (m *NetplanManager) createBackup(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	backupPath := filepath.Join(backupDir, filepath.Base(configPath)+"."+timestamp)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, data, 0644)
}

func (m *NetplanManager) Rollback(interfaceName string) error {
	if !m.ValidateInterface(interfaceName) {
		return fmt.Errorf("잘못된 인터페이스 이름: %s", interfaceName)
	}

	index := interfaceName[8:]
	filename := fmt.Sprintf("9%s-%s.yaml", index, interfaceName)
	configPath := filepath.Join(netplanDir, filename)

	// 가장 최근 백업 찾기
	pattern := filepath.Join(backupDir, filepath.Base(configPath)+".*")
	backupFiles, err := filepath.Glob(pattern)
	if err != nil || len(backupFiles) == 0 {
		m.logger.Error("백업 파일을 찾을 수 없음")
		return fmt.Errorf("백업 파일 없음")
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

	// netplan apply
	if err := m.applyNetplan(); err != nil {
		return fmt.Errorf("백업 netplan apply 실패: %w", err)
	}

	m.logger.WithField("interface", interfaceName).Info("설정 롤백 성공")
	return nil
}

func (m *NetplanManager) ConfigureInterface(iface db.MultiInterface, interfaceName string) error {
	generator := NewInterfaceGenerator()
	
	// Netplan 설정 생성
	configData, err := generator.GenerateNetplanConfig(iface, interfaceName)
	if err != nil {
		return fmt.Errorf("Netplan 설정 생성 실패: %w", err)
	}
	
	// 설정 적용
	return m.ApplyConfiguration(configData, interfaceName)
}
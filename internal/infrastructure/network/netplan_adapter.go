package network

import (
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// NetplanAdapter는 Ubuntu Netplan을 사용하는 NetworkConfigurer 및 NetworkRollbacker 구현체입니다
type NetplanAdapter struct {
	commandExecutor interfaces.CommandExecutor
	fileSystem      interfaces.FileSystem
	backupService   interfaces.BackupService
	logger          *logrus.Logger
	configDir       string
}

// NewNetplanAdapter는 새로운 NetplanAdapter를 생성합니다
func NewNetplanAdapter(
	executor interfaces.CommandExecutor,
	fs interfaces.FileSystem,
	backup interfaces.BackupService,
	logger *logrus.Logger,
) *NetplanAdapter {
	return &NetplanAdapter{
		commandExecutor: executor,
		fileSystem:      fs,
		backupService:   backup,
		logger:          logger,
		configDir:       "/etc/netplan",
	}
}

// Configure는 네트워크 인터페이스를 설정합니다
func (a *NetplanAdapter) Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error {
	// 설정 파일 경로 생성
	index := extractInterfaceIndex(name.String())
	configPath := filepath.Join(a.configDir, fmt.Sprintf("9%d-%s.yaml", index, name.String()))
	
	// 백업 생성 (기존 설정이 있는 경우)
	if err := a.backupService.CreateBackup(ctx, name.String(), configPath); err != nil {
		a.logger.WithError(err).Warn("백업 생성 실패, 계속 진행")
	}
	
	// Netplan 설정 생성
	config := a.generateNetplanConfig(iface, name.String())
	configData, err := yaml.Marshal(config)
	if err != nil {
		return errors.NewSystemError("Netplan 설정 마샬링 실패", err)
	}
	
	// 설정 파일 저장
	if err := a.fileSystem.WriteFile(configPath, configData, 0644); err != nil {
		return errors.NewSystemError("Netplan 설정 파일 저장 실패", err)
	}
	
	a.logger.WithFields(logrus.Fields{
		"interface": name.String(),
		"config_path": configPath,
	}).Info("Netplan 설정 파일 생성 완료")
	
	// Netplan 테스트 (try 명령)
	if err := a.testNetplan(ctx); err != nil {
		// 실패 시 설정 파일 제거
		a.fileSystem.Remove(configPath)
		return errors.NewNetworkError("Netplan 설정 테스트 실패", err)
	}
	
	// Netplan 적용
	if err := a.applyNetplan(ctx); err != nil {
		// 실패 시 롤백
		if rollbackErr := a.Rollback(ctx, name); rollbackErr != nil {
			a.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("Netplan 설정 적용 실패", err)
	}
	
	return nil
}

// Validate는 설정된 인터페이스가 정상 작동하는지 검증합니다
func (a *NetplanAdapter) Validate(ctx context.Context, name entities.InterfaceName) error {
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
func (a *NetplanAdapter) Rollback(ctx context.Context, name entities.InterfaceName) error {
	index := extractInterfaceIndex(name.String())
	configPath := filepath.Join(a.configDir, fmt.Sprintf("9%d-%s.yaml", index, name.String()))
	
	// 설정 파일 제거
	if a.fileSystem.Exists(configPath) {
		if err := a.fileSystem.Remove(configPath); err != nil {
			return errors.NewSystemError("설정 파일 제거 실패", err)
		}
	}
	
	// 백업이 있으면 복원
	if a.backupService.HasBackup(ctx, name.String()) {
		if err := a.backupService.RestoreLatestBackup(ctx, name.String()); err != nil {
			a.logger.WithError(err).Warn("백업 복원 실패")
		}
	}
	
	// Netplan 재적용
	if err := a.applyNetplan(ctx); err != nil {
		return errors.NewNetworkError("롤백 후 Netplan 적용 실패", err)
	}
	
	a.logger.WithField("interface", name.String()).Info("네트워크 설정 롤백 완료")
	return nil
}

// testNetplan은 netplan try 명령으로 설정을 테스트합니다
func (a *NetplanAdapter) testNetplan(ctx context.Context) error {
	// 컨테이너 환경에서는 nsenter를 사용하여 호스트 네임스페이스에서 실행
	_, err := a.commandExecutor.ExecuteWithTimeout(
		ctx, 
		120*time.Second,
		"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid",
		"netplan", "try", "--timeout=120",
	)
	return err
}

// applyNetplan은 netplan apply 명령으로 설정을 적용합니다
func (a *NetplanAdapter) applyNetplan(ctx context.Context) error {
	// 컨테이너 환경에서는 nsenter를 사용하여 호스트 네임스페이스에서 실행
	_, err := a.commandExecutor.ExecuteWithTimeout(
		ctx,
		30*time.Second,
		"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid",
		"netplan", "apply",
	)
	return err
}

// generateNetplanConfig는 Netplan 설정을 생성합니다
func (a *NetplanAdapter) generateNetplanConfig(iface entities.NetworkInterface, interfaceName string) map[string]interface{} {
	config := map[string]interface{}{
		"network": map[string]interface{}{
			"version": 2,
			"ethernets": map[string]interface{}{
				interfaceName: map[string]interface{}{
					"dhcp4": false,
					"dhcp6": false,
					"match": map[string]interface{}{
						"macaddress": iface.MacAddress,
					},
					"mtu": 1500,
				},
			},
		},
	}
	
	// IP 주소가 있으면 추가
	if iface.IPAddress != "" && iface.SubnetMask != "" {
		cidr := fmt.Sprintf("%s/%s", iface.IPAddress, iface.SubnetMask)
		ethernet := config["network"].(map[string]interface{})["ethernets"].(map[string]interface{})[interfaceName].(map[string]interface{})
		ethernet["addresses"] = []string{cidr}
		
		// 게이트웨이 설정
		if iface.Gateway != "" {
			ethernet["gateway4"] = iface.Gateway
		}
		
		// DNS 설정
		if iface.DNS != "" {
			dnsServers := strings.Split(iface.DNS, ",")
			for i, dns := range dnsServers {
				dnsServers[i] = strings.TrimSpace(dns)
			}
			ethernet["nameservers"] = map[string]interface{}{
				"addresses": dnsServers,
			}
		}
	}
	
	return config
}

// extractInterfaceIndex는 인터페이스 이름에서 인덱스를 추출합니다
func extractInterfaceIndex(name string) int {
	// multinic0 -> 0, multinic1 -> 1 등
	if strings.HasPrefix(name, "multinic") {
		indexStr := strings.TrimPrefix(name, "multinic")
		if index, err := strconv.Atoi(indexStr); err == nil {
			return index
		}
	}
	return 0
}
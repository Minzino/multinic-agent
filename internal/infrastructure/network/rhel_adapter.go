package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"

	"github.com/sirupsen/logrus"
)

// RHELAdapter는 nmcli를 사용하여 RHEL 계열 OS의 네트워크를 설정합니다.
type RHELAdapter struct {
	commandExecutor interfaces.CommandExecutor
	logger          *logrus.Logger
}

// NewRHELAdapter는 새로운 RHELAdapter를 생성합니다.
func NewRHELAdapter(
	executor interfaces.CommandExecutor,
	logger *logrus.Logger,
) *RHELAdapter {
	return &RHELAdapter{
		commandExecutor: executor,
		logger:          logger,
	}
}

// Configure는 nmcli를 사용하여 네트워크 인터페이스를 설정합니다.
func (a *RHELAdapter) Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error {
	ifaceName := name.String()
	macAddress := iface.MacAddress

	a.logger.WithFields(logrus.Fields{
		"interface": ifaceName,
		"mac":       macAddress,
	}).Info("nmcli를 사용하여 RHEL 인터페이스 설정 시작")

	// 1. 기존 연결이 있다면 삭제
	_ = a.Rollback(ctx, ifaceName)

	// 2. 새로운 연결 추가
	addCmd := []string{
		"connection", "add", "type", "ethernet", "con-name", ifaceName, "ifname", ifaceName, "mac", macAddress,
	}
	if _, err := a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", addCmd...); err != nil {
		return errors.NewNetworkError(fmt.Sprintf("nmcli connection add 실패: %s", ifaceName), err)
	}

	// 3. IP 주소 할당 비활성화 (인터페이스만 연결)
	disableIPv4Cmd := []string{"connection", "modify", ifaceName, "ipv4.method", "disabled"}
	if _, err := a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", disableIPv4Cmd...); err != nil {
		return errors.NewNetworkError(fmt.Sprintf("nmcli ipv4.method disabled 실패: %s", ifaceName), err)
	}

	disableIPv6Cmd := []string{"connection", "modify", ifaceName, "ipv6.method", "disabled"}
	if _, err := a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", disableIPv6Cmd...); err != nil {
		return errors.NewNetworkError(fmt.Sprintf("nmcli ipv6.method disabled 실패: %s", ifaceName), err)
	}

	// 4. 연결 활성화
	upCmd := []string{"connection", "up", ifaceName}
	if _, err := a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", upCmd...); err != nil {
		// 활성화 실패 시 롤백
		if rollbackErr := a.Rollback(ctx, ifaceName); rollbackErr != nil {
			a.logger.WithError(rollbackErr).Warn("nmcli connection up 실패 후 롤백 중 에러 발생")
		}
		return errors.NewNetworkError(fmt.Sprintf("nmcli connection up 실패: %s", ifaceName), err)
	}

	a.logger.WithField("interface", ifaceName).Info("nmcli 인터페이스 설정 완료")
	return nil
}

// Validate는 설정된 인터페이스가 정상적으로 활성화되었는지 검증합니다.
func (a *RHELAdapter) Validate(ctx context.Context, name entities.InterfaceName) error {
	ifaceName := name.String()
	a.logger.WithField("interface", ifaceName).Info("nmcli 인터페이스 검증 시작")

	// `nmcli device status`를 사용하여 상태 확인
	// 출력 예시: DEVICE  TYPE      STATE      CONNECTION
	//           eth0    ethernet  connected  eth0
	//           multinic0 ethernet  connected  multinic0
	output, err := a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", "device", "status")
	if err != nil {
		return errors.NewNetworkError(fmt.Sprintf("nmcli device status 실행 실패: %s", ifaceName), err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ifaceName) {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[2] == "connected" {
				a.logger.WithField("interface", ifaceName).Info("nmcli 인터페이스 검증 성공")
				return nil
			}
			return errors.NewNetworkError(fmt.Sprintf("인터페이스 %s의 상태가 'connected'가 아님: %s", ifaceName, line), nil)
		}
	}

	return errors.NewNetworkError(fmt.Sprintf("nmcli device status 출력에서 인터페이스 %s를 찾을 수 없음", ifaceName), nil)
}

// Rollback은 nmcli를 사용하여 인터페이스 설정을 제거합니다.
func (a *RHELAdapter) Rollback(ctx context.Context, name string) error {
	a.logger.WithField("interface", name).Info("nmcli 인터페이스 롤백/삭제 시작")

	// 연결 비활성화
	downCmd := []string{"connection", "down", name}
	_, err := a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", downCmd...)
	if err != nil {
		// 연결이 존재하지 않거나 이미 내려가 있는 경우 등은 에러로 간주하지 않음
		a.logger.WithError(err).WithField("interface", name).Debug("nmcli connection down 중 에러 발생 (무시 가능)")
	}

	// 연결 삭제
	deleteCmd := []string{"connection", "delete", name}
	_, err = a.commandExecutor.ExecuteWithTimeout(ctx, 30*time.Second, "nmcli", deleteCmd...)
	if err != nil {
		// 연결이 존재하지 않는 경우 등은 에러로 간주하지 않음
		a.logger.WithError(err).WithField("interface", name).Debug("nmcli connection delete 중 에러 발생 (무시 가능)")
		return nil // 롤백의 목적은 제거이므로, 이미 없어도 성공으로 간주
	}

	a.logger.WithField("interface", name).Info("nmcli 인터페이스 롤백/삭제 완료")
	return nil
}

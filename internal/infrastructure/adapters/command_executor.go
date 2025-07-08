package adapters

import (
	"bytes"
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"os/exec"
	"time"
)

// RealCommandExecutor는 실제 시스템 명령을 실행하는 CommandExecutor 구현체입니다
type RealCommandExecutor struct{}

// NewRealCommandExecutor는 새로운 RealCommandExecutor를 생성합니다
func NewRealCommandExecutor() interfaces.CommandExecutor {
	return &RealCommandExecutor{}
}

// Execute는 명령을 실행하고 결과를 반환합니다
func (e *RealCommandExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return nil, errors.NewSystemError(
			fmt.Sprintf("명령 실행 실패: %s %v", command, args),
			fmt.Errorf("%w, stderr: %s", err, stderr.String()),
		)
	}
	
	return stdout.Bytes(), nil
}

// ExecuteWithTimeout은 타임아웃을 적용하여 명령을 실행합니다
func (e *RealCommandExecutor) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, command string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	output, err := e.Execute(ctx, command, args...)
	if err != nil {
		// 컨텍스트 데드라인 초과 시 타임아웃 에러로 변환
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.NewTimeoutError(
				fmt.Sprintf("명령 실행 타임아웃: %s %v (제한시간: %v)", command, args, timeout),
			)
		}
		return nil, err
	}
	
	return output, nil
}
package utils

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig는 재시도 설정
type RetryConfig struct {
	MaxAttempts int
	InitialDelay time.Duration
	MaxDelay time.Duration
	Multiplier float64
}

// DefaultRetryConfig는 기본 재시도 설정
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 1 * time.Second,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
}

// RetryWithBackoff는 지수 백오프를 사용한 재시도
func RetryWithBackoff(ctx context.Context, config RetryConfig, operation func() error) error {
	delay := config.InitialDelay
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		
		if attempt == config.MaxAttempts {
			return fmt.Errorf("최대 재시도 횟수 초과 (%d회): %w", config.MaxAttempts, err)
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 다음 재시도를 위한 지연 시간 계산
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}
	
	return fmt.Errorf("재시도 실패")
}
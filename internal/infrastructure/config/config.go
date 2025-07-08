package config

import (
	"multinic-agent-v2/internal/domain/errors"
	"os"
	"strconv"
	"time"
)

// Config는 애플리케이션 설정을 담는 구조체입니다
type Config struct {
	Database DatabaseConfig
	Agent    AgentConfig
	Health   HealthConfig
}

// DatabaseConfig는 데이터베이스 설정을 담는 구조체입니다
type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// AgentConfig는 에이전트 설정을 담는 구조체입니다
type AgentConfig struct {
	PollInterval    time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	CommandTimeout  time.Duration
	BackupDirectory string
}

// HealthConfig는 헬스체크 설정을 담는 구조체입니다
type HealthConfig struct {
	Port string
}

// ConfigLoader는 설정을 로드하는 인터페이스입니다
type ConfigLoader interface {
	Load() (*Config, error)
}

// EnvironmentConfigLoader는 환경 변수에서 설정을 로드하는 구현체입니다
type EnvironmentConfigLoader struct{}

// NewEnvironmentConfigLoader는 새로운 EnvironmentConfigLoader를 생성합니다
func NewEnvironmentConfigLoader() ConfigLoader {
	return &EnvironmentConfigLoader{}
}

// Load는 환경 변수에서 설정을 로드합니다
func (l *EnvironmentConfigLoader) Load() (*Config, error) {
	config := &Config{
		Database: DatabaseConfig{
			Host:         getEnvOrDefault("DB_HOST", "192.168.34.79"),
			Port:         getEnvOrDefault("DB_PORT", "30305"),
			User:         getEnvOrDefault("DB_USER", "root"),
			Password:     getEnvOrDefault("DB_PASSWORD", "cloud1234"),
			Database:     getEnvOrDefault("DB_NAME", "multinic"),
			MaxOpenConns: getEnvIntOrDefault("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns: getEnvIntOrDefault("DB_MAX_IDLE_CONNS", 5),
			MaxLifetime:  getEnvDurationOrDefault("DB_MAX_LIFETIME", 5*time.Minute),
		},
		Agent: AgentConfig{
			PollInterval:    getEnvDurationOrDefault("POLL_INTERVAL", 30*time.Second),
			MaxRetries:      getEnvIntOrDefault("MAX_RETRIES", 3),
			RetryDelay:      getEnvDurationOrDefault("RETRY_DELAY", 2*time.Second),
			CommandTimeout:  getEnvDurationOrDefault("COMMAND_TIMEOUT", 30*time.Second),
			BackupDirectory: getEnvOrDefault("BACKUP_DIR", "/var/lib/multinic/backups"),
		},
		Health: HealthConfig{
			Port: getEnvOrDefault("HEALTH_PORT", "8080"),
		},
	}
	
	// 설정 유효성 검증
	if err := l.validate(config); err != nil {
		return nil, err
	}
	
	return config, nil
}

// validate는 설정의 유효성을 검증합니다
func (l *EnvironmentConfigLoader) validate(config *Config) error {
	// 데이터베이스 설정 검증
	if config.Database.Host == "" {
		return errors.NewValidationError("데이터베이스 호스트가 설정되지 않음", nil)
	}
	if config.Database.Port == "" {
		return errors.NewValidationError("데이터베이스 포트가 설정되지 않음", nil)
	}
	if config.Database.User == "" {
		return errors.NewValidationError("데이터베이스 사용자가 설정되지 않음", nil)
	}
	if config.Database.Database == "" {
		return errors.NewValidationError("데이터베이스 이름이 설정되지 않음", nil)
	}
	
	// 에이전트 설정 검증
	if config.Agent.PollInterval <= 0 {
		return errors.NewValidationError("폴링 간격이 유효하지 않음", nil)
	}
	if config.Agent.MaxRetries < 0 {
		return errors.NewValidationError("최대 재시도 횟수가 유효하지 않음", nil)
	}
	
	// 헬스체크 설정 검증
	if config.Health.Port == "" {
		return errors.NewValidationError("헬스체크 포트가 설정되지 않음", nil)
	}
	
	return nil
}

// 환경 변수 헬퍼 함수들

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
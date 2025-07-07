package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"multinic-agent-v2/pkg/db"
	"multinic-agent-v2/pkg/health"
	"multinic-agent-v2/pkg/network"
	"multinic-agent-v2/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Config struct {
	DB struct {
		Host     string
		Port     string
		User     string
		Password string
		Database string
	}
	PollInterval time.Duration
	HealthPort   string
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	config := loadConfig()
	
	// 데이터베이스 설정 유효성 검사
	if err := utils.ValidateDatabaseConfig(
		config.DB.Host,
		config.DB.Port,
		config.DB.User,
		config.DB.Password,
		config.DB.Database,
	); err != nil {
		logger.WithError(err).Fatal("잘못된 데이터베이스 설정")
	}
	
	// 데이터베이스 연결 재시도
	var dbClient *db.Client
	err := utils.RetryWithBackoff(context.Background(), utils.DefaultRetryConfig, func() error {
		var err error
		dbClient, err = db.NewClient(db.Config{
			Host:     config.DB.Host,
			Port:     config.DB.Port,
			User:     config.DB.User,
			Password: config.DB.Password,
			Database: config.DB.Database,
		}, logger)
		return err
	})
	if err != nil {
		logger.WithError(err).Fatal("데이터베이스 연결 실패")
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			logger.WithError(err).Error("데이터베이스 연결 종료 실패")
		}
	}()

	networkManager, err := network.NewNetworkManager(logger)
	if err != nil {
		logger.WithError(err).Fatal("네트워크 매니저 생성 실패")
	}
	
	logger.WithField("network_manager", networkManager.GetType()).Info("네트워크 매니저 초기화")
	
	// 헬스체크 서버 시작
	healthChecker := health.NewHealthChecker(logger)
	healthServer := &http.Server{
		Addr:    ":" + config.HealthPort,
		Handler: healthChecker,
	}
	
	go func() {
		logger.WithField("port", config.HealthPort).Info("헬스체크 서버 시작")
		if err := healthServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.WithError(err).Error("헬스체크 서버 실패")
		}
	}()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	logger.Info("MultiNIC 에이전트 시작")

	for {
		select {
		case <-ctx.Done():
			logger.Info("에이전트 종료")
			// 헬스체크 서버 정리
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			healthServer.Shutdown(shutdownCtx)
			return
		case <-sigChan:
			logger.Info("종료 신호 수신")
			cancel()
		case <-ticker.C:
			if err := processConfigurations(dbClient, networkManager, healthChecker, logger); err != nil {
				logger.WithError(err).Error("설정 처리 실패")
				healthChecker.UpdateDBHealth(false, err)
			} else {
				healthChecker.UpdateDBHealth(true, nil)
			}
		}
	}
}

func loadConfig() Config {
	config := Config{
		PollInterval: 30 * time.Second,
		HealthPort:   "8080",
	}

	config.DB.Host = getEnv("DB_HOST", "192.168.34.79")
	config.DB.Port = getEnv("DB_PORT", "30305")
	config.DB.User = getEnv("DB_USER", "root")
	config.DB.Password = getEnv("DB_PASSWORD", "cloud1234")
	config.DB.Database = getEnv("DB_NAME", "multinic")
	config.HealthPort = getEnv("HEALTH_PORT", "8080")

	if interval := os.Getenv("POLL_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			config.PollInterval = d
		}
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func processConfigurations(dbClient *db.Client, networkManager network.NetworkManager, healthChecker *health.HealthChecker, logger *logrus.Logger) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("호스트네임 가져오기 실패: %w", err)
	}

	// 호스트네임 유효성 검사
	if err := utils.ValidateHostname(hostname); err != nil {
		return fmt.Errorf("잘못된 호스트네임: %w", err)
	}

	// multi_interface 테이블에서 netplan_success=0이고 attached_node_name이 본인인 레코드 조회
	pendingInterfaces, err := dbClient.GetPendingInterfaces(hostname)
	if err != nil {
		return fmt.Errorf("대기 중인 인터페이스 조회 실패: %w", err)
	}

	if len(pendingInterfaces) == 0 {
		return nil // 처리할 인터페이스가 없음
	}

	logger.WithFields(logrus.Fields{
		"node_name":        hostname,
		"pending_count":    len(pendingInterfaces),
	}).Info("대기 중인 인터페이스 발견")

	// 인터페이스 이름 생성기 초기화
	generator := network.NewInterfaceGenerator()
	
	var processedCount int
	var failedCount int

	for _, iface := range pendingInterfaces {
		logger.WithFields(logrus.Fields{
			"interface_id": iface.ID,
			"mac_address":  iface.MacAddress,
			"ip_address":   iface.IPAddress,
		}).Info("인터페이스 설정 처리 시작")

		// 인터페이스 이름 생성 (multinic0~9)
		interfaceName, err := generator.GenerateInterfaceName()
		if err != nil {
			logger.WithError(err).Error("인터페이스 이름 생성 실패")
			failedCount++
			healthChecker.IncrementFailedConfigs()
			continue
		}

		logger.WithFields(logrus.Fields{
			"interface_id":   iface.ID,
			"interface_name": interfaceName,
		}).Info("인터페이스 설정 적용 중")

		// 네트워크 설정 적용 (재시도 포함)
		err = utils.RetryWithBackoff(context.Background(), utils.RetryConfig{
			MaxAttempts:  2,
			InitialDelay: 2 * time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
		}, func() error {
			return networkManager.ConfigureInterface(iface, interfaceName)
		})

		if err != nil {
			logger.WithFields(logrus.Fields{
				"interface_id":   iface.ID,
				"interface_name": interfaceName,
				"error":          err,
			}).Error("인터페이스 설정 적용 실패")
			failedCount++
			healthChecker.IncrementFailedConfigs()
			continue
		}

		// 성공한 경우 DB 상태 업데이트
		if err := dbClient.UpdateInterfaceStatus(iface.ID, true); err != nil {
			logger.WithFields(logrus.Fields{
				"interface_id": iface.ID,
				"error":        err,
			}).Error("인터페이스 상태 업데이트 실패")
		} else {
			logger.WithFields(logrus.Fields{
				"interface_id":   iface.ID,
				"interface_name": interfaceName,
			}).Info("인터페이스 설정 성공")
			processedCount++
			healthChecker.IncrementProcessedVMs()
		}
	}

	logger.WithFields(logrus.Fields{
		"processed": processedCount,
		"failed":    failedCount,
		"total":     len(pendingInterfaces),
	}).Info("인터페이스 처리 완료")

	return nil
}
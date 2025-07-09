package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"multinic-agent-v2/internal/application/usecases"
	"multinic-agent-v2/internal/infrastructure/config"
	"multinic-agent-v2/internal/infrastructure/container"

	"github.com/sirupsen/logrus"
)

func main() {
	// 로거 초기화
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// LOG_LEVEL 환경 변수 설정
	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr != "" {
		logLevel, err := logrus.ParseLevel(logLevelStr)
		if err != nil {
			logger.WithError(err).Warnf("알 수 없는 LOG_LEVEL 값: %s. 기본 Info 레벨을 사용합니다.", logLevelStr)
			logger.SetLevel(logrus.InfoLevel) // Fallback to Info
		} else {
			logger.SetLevel(logLevel)
		}
	} else {
		logger.SetLevel(logrus.InfoLevel) // Default to Info if not set
	}

	// 설정 로드
	configLoader := config.NewEnvironmentConfigLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		logger.WithError(err).Fatal("설정 로드 실패")
	}
	defer func() {
		if err := appContainer.Close(); err != nil {
			logger.WithError(err).Error("컨테이너 정리 실패")
		}
	}()

	// 애플리케이션 시작
	app := NewApplication(appContainer, logger)
	if err := app.Run(); err != nil {
		logger.WithError(err).Fatal("애플리케이션 실행 실패")
	}
}

// Application은 메인 애플리케이션 구조체입니다
type Application struct {
	container        *container.Container
	logger           *logrus.Logger
	configureUseCase *usecases.ConfigureNetworkUseCase
	deleteUseCase    *usecases.DeleteNetworkUseCase
	healthServer     *http.Server
}

// NewApplication은 새로운 Application을 생성합니다
func NewApplication(container *container.Container, logger *logrus.Logger) *Application {
	return &Application{
		container:        container,
		logger:           logger,
		configureUseCase: container.GetConfigureNetworkUseCase(),
		deleteUseCase:    container.GetDeleteNetworkUseCase(),
	}
}

// Run은 애플리케이션을 실행합니다
func (a *Application) Run() error {
	cfg := a.container.GetConfig()

	// 헬스체크 서버 시작
	if err := a.startHealthServer(cfg.Health.Port); err != nil {
		return err
	}

	// 컨텍스트 및 시그널 핸들링 설정
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 메인 루프 시작
	ticker := time.NewTicker(cfg.Agent.PollInterval)
	defer ticker.Stop()

	a.logger.Info("MultiNIC 에이전트 시작")

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("에이전트 종료")
			return a.shutdown()

		case <-sigChan:
			a.logger.Info("종료 신호 수신")
			cancel()

		case <-ticker.C:
			if err := a.processNetworkConfigurations(ctx); err != nil {
				a.logger.WithError(err).Error("네트워크 설정 처리 실패")
				a.container.GetHealthService().UpdateDBHealth(false, err)
			} else {
				a.container.GetHealthService().UpdateDBHealth(true, nil)
			}
		}
	}
}

// startHealthServer는 헬스체크 서버를 시작합니다
func (a *Application) startHealthServer(port string) error {
	healthService := a.container.GetHealthService()

	a.healthServer = &http.Server{
		Addr:    ":" + port,
		Handler: healthService,
	}

	go func() {
		a.logger.WithField("port", port).Info("헬스체크 서버 시작")
		if err := a.healthServer.ListenAndServe(); err != http.ErrServerClosed {
			a.logger.WithError(err).Error("헬스체크 서버 실패")
		}
	}()

	return nil
}

// processNetworkConfigurations는 네트워크 설정을 처리합니다
func (a *Application) processNetworkConfigurations(ctx context.Context) error {
	// 호스트네임 가져오기
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	// 1. 네트워크 설정 유스케이스 실행 (생성/수정)
	configInput := usecases.ConfigureNetworkInput{
		NodeName: hostname,
	}

	configOutput, err := a.configureUseCase.Execute(ctx, configInput)
	if err != nil {
		return err
	}

	// 2. 네트워크 삭제 유스케이스 실행 (고아 인터페이스 정리)
	deleteInput := usecases.DeleteNetworkInput{
		NodeName: hostname,
	}

	deleteOutput, err := a.deleteUseCase.Execute(ctx, deleteInput)
	if err != nil {
		a.logger.WithError(err).Error("고아 인터페이스 삭제 처리 실패")
		// 삭제 실패는 치명적이지 않으므로 계속 진행
	}

	// 헬스체크 통계 업데이트 (설정 관련)
	healthService := a.container.GetHealthService()
	for i := 0; i < configOutput.ProcessedCount; i++ {
		healthService.IncrementProcessedVMs()
	}
	for i := 0; i < configOutput.FailedCount; i++ {
		healthService.IncrementFailedConfigs()
	}

	// 처리 결과 로깅
	totalProcessed := configOutput.TotalCount + deleteOutput.TotalDeleted
	if totalProcessed > 0 {
		a.logger.WithFields(logrus.Fields{
			"config_processed": configOutput.ProcessedCount,
			"config_failed":    configOutput.FailedCount,
			"config_total":     configOutput.TotalCount,
			"deleted_total":    deleteOutput.TotalDeleted,
			"delete_errors":    len(deleteOutput.Errors),
		}).Info("네트워크 처리 완료")
	}

	// 삭제 에러가 있다면 별도로 로깅
	if len(deleteOutput.Errors) > 0 {
		for _, delErr := range deleteOutput.Errors {
			a.logger.WithError(delErr).Warn("인터페이스 삭제 중 에러 발생")
		}
	}

	return nil
}

// shutdown은 애플리케이션을 정리하고 종료합니다
func (a *Application) shutdown() error {
	// 헬스체크 서버 정리
	if a.healthServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := a.healthServer.Shutdown(shutdownCtx); err != nil {
			a.logger.WithError(err).Error("헬스체크 서버 종료 실패")
		}
	}

	return nil
}

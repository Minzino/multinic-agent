// +build integration

package integration

import (
	"context"
	"testing"
	"time"
	
	"multinic-agent-v2/internal/application/usecases"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/interfaces"
	"multinic-agent-v2/internal/infrastructure/adapters"
	"multinic-agent-v2/internal/infrastructure/config"
	"multinic-agent-v2/internal/infrastructure/container"
	"multinic-agent-v2/internal/infrastructure/services"
	
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanArchitectureIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트는 -short 플래그와 함께 실행시 스킵됩니다")
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // 테스트 중 로그 출력 억제
	
	t.Run("설정 로드 통합 테스트", func(t *testing.T) {
		configLoader := config.NewEnvironmentConfigLoader()
		cfg, err := configLoader.Load()
		
		assert.NoError(t, err)
		require.NotNil(t, cfg)
		
		// 기본값 확인
		assert.Equal(t, "192.168.34.79", cfg.Database.Host)
		assert.Equal(t, "30305", cfg.Database.Port)
		assert.Equal(t, 30*time.Second, cfg.Agent.PollInterval)
	})
	
	t.Run("OS 감지 통합 테스트", func(t *testing.T) {
		fs := adapters.NewRealFileSystem()
		detector := adapters.NewRealOSDetector(fs)
		
		osType, err := detector.DetectOS()
		assert.NoError(t, err)
		assert.Contains(t, []string{string(interfaces.OSTypeUbuntu), string(interfaces.OSTypeSUSE)}, string(osType))
		
		t.Logf("감지된 OS: %s", osType)
	})
	
	t.Run("인터페이스 네이밍 서비스 통합 테스트", func(t *testing.T) {
		fs := adapters.NewRealFileSystem()
		namingService := services.NewInterfaceNamingService(fs)
		
		// 실제 시스템에서 사용 가능한 인터페이스 이름 생성
		interfaceName, err := namingService.GenerateNextName()
		
		if err != nil {
			// 모든 인터페이스가 사용 중일 수 있음
			assert.Contains(t, err.Error(), "사용 가능한 인터페이스 이름이 없습니다")
			t.Log("모든 multinic 인터페이스가 사용 중")
		} else {
			assert.Regexp(t, `^multinic[0-9]$`, interfaceName.String())
			t.Logf("생성된 인터페이스 이름: %s", interfaceName.String())
		}
	})
	
	t.Run("백업 서비스 통합 테스트", func(t *testing.T) {
		fs := adapters.NewRealFileSystem()
		clock := adapters.NewRealClock()
		backupDir := "/tmp/multinic-test-backup"
		
		backupService := services.NewBackupService(fs, clock, logger, backupDir)
		
		ctx := context.Background()
		testInterface := "multinic0"
		testConfigPath := "/tmp/test-config.yaml"
		
		// 테스트 설정 파일 생성
		testContent := []byte("test config content")
		err := fs.WriteFile(testConfigPath, testContent, 0644)
		require.NoError(t, err)
		defer fs.Remove(testConfigPath)
		
		// 백업 생성
		err = backupService.CreateBackup(ctx, testInterface, testConfigPath)
		assert.NoError(t, err)
		
		// 백업 존재 확인
		hasBackup := backupService.HasBackup(ctx, testInterface)
		assert.True(t, hasBackup)
		
		// 백업 복원 테스트
		err = backupService.RestoreLatestBackup(ctx, testInterface)
		assert.NoError(t, err)
		
		t.Log("백업 서비스 통합 테스트 성공")
	})
}

func TestUseCaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트는 -short 플래그와 함께 실행시 스킵됩니다")
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	
	t.Run("ConfigureNetworkUseCase 모킹된 의존성과 함께", func(t *testing.T) {
		// 모킹된 레포지토리와 서비스들을 사용하여 실제 유스케이스 로직 테스트
		mockRepo := &MockRepository{}
		mockConfigurer := &MockConfigurer{}
		mockRollbacker := &MockRollbacker{}
		
		fs := adapters.NewRealFileSystem()
		clock := adapters.NewRealClock()
		backupService := services.NewBackupService(fs, clock, logger, "/tmp/test-backup")
		namingService := services.NewInterfaceNamingService(fs)
		
		useCase := usecases.NewConfigureNetworkUseCase(
			mockRepo,
			mockConfigurer,
			mockRollbacker,
			backupService,
			namingService,
			logger,
		)
		
		// 빈 결과 테스트
		mockRepo.pendingInterfaces = []entities.NetworkInterface{}
		
		input := usecases.ConfigureNetworkInput{NodeName: "test-node"}
		output, err := useCase.Execute(context.Background(), input)
		
		assert.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, 0, output.TotalCount)
		
		t.Log("유스케이스 통합 테스트 성공")
	})
}

func TestContainerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트는 -short 플래그와 함께 실행시 스킵됩니다")
	}
	
	t.Run("의존성 컨테이너 초기화", func(t *testing.T) {
		// 테스트용 설정
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "3306",
				User:     "test",
				Password: "test",
				Database: "test",
			},
			Agent: config.AgentConfig{
				PollInterval:    30 * time.Second,
				BackupDirectory: "/tmp/test-backup",
			},
			Health: config.HealthConfig{
				Port: "8080",
			},
		}
		
		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)
		
		// 컨테이너는 실제 DB 연결이 필요하므로 DB가 없으면 스킵
		appContainer, err := container.NewContainer(cfg, logger)
		if err != nil {
			t.Skipf("컨테이너 초기화 실패 (테스트 DB가 없을 수 있음): %v", err)
		}
		defer appContainer.Close()
		
		// 컨테이너에서 서비스들 가져오기
		healthService := appContainer.GetHealthService()
		assert.NotNil(t, healthService)
		
		useCase := appContainer.GetConfigureNetworkUseCase()
		assert.NotNil(t, useCase)
		
		t.Log("의존성 컨테이너 초기화 성공")
	})
}

// 테스트용 Mock 구현들
type MockRepository struct {
	pendingInterfaces []entities.NetworkInterface
}

func (m *MockRepository) GetPendingInterfaces(ctx context.Context, nodeName string) ([]entities.NetworkInterface, error) {
	return m.pendingInterfaces, nil
}

func (m *MockRepository) UpdateInterfaceStatus(ctx context.Context, interfaceID int, status entities.InterfaceStatus) error {
	return nil
}

func (m *MockRepository) GetInterfaceByID(ctx context.Context, id int) (*entities.NetworkInterface, error) {
	return nil, nil
}

type MockConfigurer struct{}

func (m *MockConfigurer) Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error {
	return nil
}

func (m *MockConfigurer) Validate(ctx context.Context, name entities.InterfaceName) error {
	return nil
}

type MockRollbacker struct{}

func (m *MockRollbacker) Rollback(ctx context.Context, name entities.InterfaceName) error {
	return nil
}
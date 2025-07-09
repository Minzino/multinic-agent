package usecases

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"multinic-agent-v2/internal/domain/entities"
	domainErrors "multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/services"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock 구현체들
type MockNetworkInterfaceRepository struct {
	mock.Mock
}

func (m *MockNetworkInterfaceRepository) GetPendingInterfaces(ctx context.Context, nodeName string) ([]entities.NetworkInterface, error) {
	args := m.Called(ctx, nodeName)
	return args.Get(0).([]entities.NetworkInterface), args.Error(1)
}

func (m *MockNetworkInterfaceRepository) GetConfiguredInterfaces(ctx context.Context, nodeName string) ([]entities.NetworkInterface, error) {
	args := m.Called(ctx, nodeName)
	return args.Get(0).([]entities.NetworkInterface), args.Error(1)
}

func (m *MockNetworkInterfaceRepository) UpdateInterfaceStatus(ctx context.Context, interfaceID int, status entities.InterfaceStatus) error {
	args := m.Called(ctx, interfaceID, status)
	return args.Error(0)
}

func (m *MockNetworkInterfaceRepository) GetInterfaceByID(ctx context.Context, id int) (*entities.NetworkInterface, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*entities.NetworkInterface), args.Error(1)
}

func (m *MockNetworkInterfaceRepository) GetActiveInterfaces(ctx context.Context, nodeName string) ([]entities.NetworkInterface, error) {
	args := m.Called(ctx, nodeName)
	return args.Get(0).([]entities.NetworkInterface), args.Error(1)
}

type MockNetworkConfigurer struct {
	mock.Mock
}

func (m *MockNetworkConfigurer) Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error {
	args := m.Called(ctx, iface, name)
	return args.Error(0)
}

func (m *MockNetworkConfigurer) Validate(ctx context.Context, name entities.InterfaceName) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

type MockNetworkRollbacker struct {
	mock.Mock
}

func (m *MockNetworkRollbacker) Rollback(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	args := m.Called(path, data, perm)
	return args.Error(0)
}

func (m *MockFileSystem) Exists(path string) bool {
	args := m.Called(path)
	return args.Bool(0)
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

func (m *MockFileSystem) Remove(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockFileSystem) ListFiles(path string) ([]string, error) {
	args := m.Called(path)
	return args.Get(0).([]string), args.Error(1)
}

// MockCommandExecutor는 CommandExecutor 인터페이스의 목 구현체입니다
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, error) {
	mockArgs := m.Called(ctx, command, args)
	return mockArgs.Get(0).([]byte), mockArgs.Error(1)
}

func (m *MockCommandExecutor) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, command string, args ...string) ([]byte, error) {
	mockArgs := m.Called(ctx, timeout, command, args)
	return mockArgs.Get(0).([]byte), mockArgs.Error(1)
}

func TestConfigureNetworkUseCase_Execute(t *testing.T) {
	tests := []struct {
		name           string
		input          ConfigureNetworkInput
		setupMocks     func(*MockNetworkInterfaceRepository, *MockNetworkConfigurer, *MockNetworkRollbacker, *MockFileSystem)
		expectedOutput *ConfigureNetworkOutput
		wantError      bool
	}{
		{
			name: "처리할 인터페이스가 없는 경우",
			input: ConfigureNetworkInput{
				NodeName: "test-node",
			},
			setupMocks: func(repo *MockNetworkInterfaceRepository, configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				repo.On("GetPendingInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, nil)
				repo.On("GetConfiguredInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, nil)
			},
			expectedOutput: &ConfigureNetworkOutput{
				ProcessedCount: 0,
				FailedCount:    0,
				TotalCount:     0,
			},
			wantError: false,
		},
		{
			name: "단일 인터페이스 성공적으로 처리",
			input: ConfigureNetworkInput{
				NodeName: "test-node",
			},
			setupMocks: func(repo *MockNetworkInterfaceRepository, configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				testInterface := entities.NetworkInterface{
					ID:               1,
					MacAddress:       "00:11:22:33:44:55",
					AttachedNodeName: "test-node",
					Status:           entities.StatusPending,
					Address:          "10.10.10.10",
					CIDR:             "10.10.10.0/24",
					MTU:              1500,
				}

				repo.On("GetPendingInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{testInterface}, nil)
				repo.On("GetConfiguredInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, nil)

				// 인터페이스 이름 생성을 위한 파일 시스템 mock
				fs.On("Exists", "/sys/class/net/multinic0").Return(false)

				// 네트워크 설정 성공
				configurer.On("Configure", mock.Anything, testInterface, mock.MatchedBy(func(name entities.InterfaceName) bool {
					return name.String() == "multinic0"
				})).Return(nil)

				// 검증 성공
				configurer.On("Validate", mock.Anything, mock.MatchedBy(func(name entities.InterfaceName) bool {
					return name.String() == "multinic0"
				})).Return(nil)

				// 상태 업데이트 성공
				repo.On("UpdateInterfaceStatus", mock.Anything, 1, entities.StatusConfigured).Return(nil)
			},
			expectedOutput: &ConfigureNetworkOutput{
				ProcessedCount: 1,
				FailedCount:    0,
				TotalCount:     1,
			},
			wantError: false,
		},
		{
			name: "네트워크 설정 실패 시 롤백 수행",
			input: ConfigureNetworkInput{
				NodeName: "test-node",
			},
			setupMocks: func(repo *MockNetworkInterfaceRepository, configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				testInterface := entities.NetworkInterface{
					ID:               1,
					MacAddress:       "00:11:22:33:44:55",
					AttachedNodeName: "test-node",
					Status:           entities.StatusPending,
					Address:          "10.10.10.10",
					CIDR:             "10.10.10.0/24",
					MTU:              1500,
				}

				repo.On("GetPendingInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{testInterface}, nil)
				repo.On("GetConfiguredInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, nil)

				// 인터페이스 이름 생성을 위한 파일 시스템 mock
				fs.On("Exists", "/sys/class/net/multinic0").Return(false)

				// 네트워크 설정 실패
				configurer.On("Configure", mock.Anything, testInterface, mock.MatchedBy(func(name entities.InterfaceName) bool {
					return name.String() == "multinic0"
				})).Return(errors.New("설정 실패"))

				// 롤백 수행
				rollbacker.On("Rollback", mock.Anything, "multinic0").Return(nil)

				// 실패 상태로 업데이트
				repo.On("UpdateInterfaceStatus", mock.Anything, 1, entities.StatusFailed).Return(nil)
			},
			expectedOutput: &ConfigureNetworkOutput{
				ProcessedCount: 0,
				FailedCount:    1,
				TotalCount:     1,
			},
			wantError: false,
		},
		{
			name: "검증 실패 시 롤백 수행",
			input: ConfigureNetworkInput{
				NodeName: "test-node",
			},
			setupMocks: func(repo *MockNetworkInterfaceRepository, configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				testInterface := entities.NetworkInterface{
					ID:               1,
					MacAddress:       "00:11:22:33:44:55",
					AttachedNodeName: "test-node",
					Status:           entities.StatusPending,
					Address:          "10.10.10.10",
					CIDR:             "10.10.10.0/24",
					MTU:              1500,
				}

				repo.On("GetPendingInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{testInterface}, nil)
				repo.On("GetConfiguredInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, nil)

				// 인터페이스 이름 생성을 위한 파일 시스템 mock
				fs.On("Exists", "/sys/class/net/multinic0").Return(false)

				// 네트워크 설정 성공
				configurer.On("Configure", mock.Anything, testInterface, mock.MatchedBy(func(name entities.InterfaceName) bool {
					return name.String() == "multinic0"
				})).Return(nil)

				// 검증 실패
				configurer.On("Validate", mock.Anything, mock.MatchedBy(func(name entities.InterfaceName) bool {
					return name.String() == "multinic0"
				})).Return(errors.New("검증 실패"))

				// 롤백 수행
				rollbacker.On("Rollback", mock.Anything, "multinic0").Return(nil)

				// 실패 상태로 업데이트
				repo.On("UpdateInterfaceStatus", mock.Anything, 1, entities.StatusFailed).Return(nil)
			},
			expectedOutput: &ConfigureNetworkOutput{
				ProcessedCount: 0,
				FailedCount:    1,
				TotalCount:     1,
			},
			wantError: false,
		},
		{
			name: "데이터베이스 조회 실패",
			input: ConfigureNetworkInput{
				NodeName: "test-node",
			},
			setupMocks: func(repo *MockNetworkInterfaceRepository, configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				repo.On("GetPendingInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, errors.New("DB 연결 실패"))
			},
			expectedOutput: nil,
			wantError:      true,
		},
		{
			name: "설정 동기화 - 변경된 IP와 MTU를 감지하고 수정",
			input: ConfigureNetworkInput{
				NodeName: "test-node",
			},
			setupMocks: func(repo *MockNetworkInterfaceRepository, configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				// 1. No pending interfaces
				repo.On("GetPendingInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{}, nil)

				// 2. One configured interface in DB with correct data
				dbIface := entities.NetworkInterface{
					ID:         1,
					MacAddress: "00:11:22:33:44:55",
					Address:    "1.1.1.1",
					CIDR:       "1.1.1.0/24",
					MTU:        1500,
				}
				repo.On("GetConfiguredInterfaces", mock.Anything, "test-node").Return([]entities.NetworkInterface{dbIface}, nil)

				// 3. A netplan file on disk with drifted data
				fileName := "/etc/netplan/90-multinic0.yaml"
				// Note: The address in YAML has the prefix, but the DB Address field does not.
				driftedYAML := `network:
  version: 2
  ethernets:
    multinic0:
      match:
        macaddress: "00:11:22:33:44:55"
      addresses: ["1.1.1.2/24"] # Drifted IP
      mtu: 1400`               // Drifted MTU
				fs.On("ListFiles", "/etc/netplan").Return([]string{fileName}, nil)
				fs.On("ReadFile", fileName).Return([]byte(driftedYAML), nil)
				fs.On("Exists", fileName).Return(true)

				// 4. Expect Configure to be called with the correct DB data to fix the drift
				configurer.On("Configure", mock.Anything, dbIface, mock.MatchedBy(func(name entities.InterfaceName) bool {
					return name.String() == "multinic0"
				})).Return(nil)
			},
			expectedOutput: &ConfigureNetworkOutput{
				ProcessedCount: 0, // No pending interfaces processed
				FailedCount:    0,
				TotalCount:     0,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 객체들 생성
			mockRepo := new(MockNetworkInterfaceRepository)
			mockConfigurer := new(MockNetworkConfigurer)
			mockRollbacker := new(MockNetworkRollbacker)
			mockFS := new(MockFileSystem)

			// Mock 설정
			tt.setupMocks(mockRepo, mockConfigurer, mockRollbacker, mockFS)

			// Mock CommandExecutor 생성
			mockExecutor := new(MockCommandExecutor)

			// 네이밍 서비스 생성
			namingService := services.NewInterfaceNamingService(mockFS, mockExecutor)

			// 로거 생성
			logger := logrus.New()
			logger.SetLevel(logrus.FatalLevel) // 테스트 중 로그 출력 억제

			// 유스케이스 생성
			useCase := NewConfigureNetworkUseCase(
				mockRepo,
				mockConfigurer,
				mockRollbacker,
				namingService,
				mockFS, // Add mockFS here
				logger,
			)

			// 실행
			result, err := useCase.Execute(context.Background(), tt.input)

			// 검증
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedOutput.ProcessedCount, result.ProcessedCount)
				assert.Equal(t, tt.expectedOutput.FailedCount, result.FailedCount)
				assert.Equal(t, tt.expectedOutput.TotalCount, result.TotalCount)
			}

			// Mock 호출 검증
			mockRepo.AssertExpectations(t)
			mockConfigurer.AssertExpectations(t)
			mockRollbacker.AssertExpectations(t)
			mockFS.AssertExpectations(t)
		})
	}
}

func TestConfigureNetworkUseCase_processInterface(t *testing.T) {
	tests := []struct {
		name       string
		iface      entities.NetworkInterface
		setupMocks func(*MockNetworkConfigurer, *MockNetworkRollbacker, *MockFileSystem)
		wantError  bool
		errorType  string
	}{
		{
			name: "잘못된 MAC 주소로 인한 유효성 검증 실패",
			iface: entities.NetworkInterface{
				ID:               1,
				MacAddress:       "invalid-mac",
				AttachedNodeName: "test-node",
			},
			setupMocks: func(configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				// 유효성 검증 실패로 다른 메서드들은 호출되지 않음
			},
			wantError: true,
			errorType: "VALIDATION",
		},
		{
			name: "인터페이스 이름 생성 실패 (모든 인터페이스 사용 중)",
			iface: entities.NetworkInterface{
				ID:               1,
				MacAddress:       "00:11:22:33:44:55",
				AttachedNodeName: "test-node",
			},
			setupMocks: func(configurer *MockNetworkConfigurer, rollbacker *MockNetworkRollbacker, fs *MockFileSystem) {
				// 모든 인터페이스가 사용 중이라고 설정
				for i := 0; i < 10; i++ {
					fs.On("Exists", fmt.Sprintf("/sys/class/net/multinic%d", i)).Return(true)
				}
			},
			wantError: true,
			errorType: "SYSTEM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 객체들 생성
			mockConfigurer := new(MockNetworkConfigurer)
			mockRollbacker := new(MockNetworkRollbacker)
			mockFS := new(MockFileSystem)
			mockRepo := new(MockNetworkInterfaceRepository)
			mockExecutor := new(MockCommandExecutor)

			// Mock 설정
			tt.setupMocks(mockConfigurer, mockRollbacker, mockFS)

			// 네이밍 서비스 생성
			namingService := services.NewInterfaceNamingService(mockFS, mockExecutor)

			// 로거 생성
			logger := logrus.New()
			logger.SetLevel(logrus.FatalLevel)

			// 유스케이스 생성
			useCase := NewConfigureNetworkUseCase(
				mockRepo,
				mockConfigurer,
				mockRollbacker,
				namingService,
				mockFS, // Add mockFS here
				logger,
			)

			// processInterface 메서드 테스트
			err := useCase.processInterface(context.Background(), tt.iface)

			// 검증
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorType != "" {
					var domainErr *domainErrors.DomainError
					if assert.ErrorAs(t, err, &domainErr) {
						assert.Equal(t, domainErrors.ErrorType(tt.errorType), domainErr.Type)
					}
				}
			} else {
				assert.NoError(t, err)
			}

			// Mock 호출 검증
			mockConfigurer.AssertExpectations(t)
			mockRollbacker.AssertExpectations(t)
			mockFS.AssertExpectations(t)
		})
	}
}

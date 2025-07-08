package usecases

import (
	"context"
	"errors"
	"testing"

	"multinic-agent-v2/internal/domain/services"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteNetworkUseCase_Execute_NetplanFileCleanup_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	mockExecutor := new(MockCommandExecutor)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // 테스트 중 로그 출력 억제

	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 1. /etc/netplan 디렉토리에 고아 파일들이 있음
	netplanFiles := []string{
		"50-cloud-init.yaml",    // 무시됨 (multinic 관련 아님)
		"91-multinic1.yaml",     // multinic1 - 시스템에 존재
		"92-multinic2.yaml",     // multinic2 - 시스템에 존재하지 않음 (고아 파일)
	}
	mockFileSystem.On("ListFiles", "/etc/netplan").Return(netplanFiles, nil)

	// 2. multinic1은 시스템에 존재 (MAC 주소 조회 성공)
	mockExecutor.On("ExecuteWithTimeout", 
		mock.AnythingOfType("*context.timerCtx"), 
		mock.AnythingOfType("time.Duration"), 
		"ip", 
		[]string{"addr", "show", "multinic1"}).Return([]byte(`
2: multinic1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc fq_codel state UP group default qlen 1000
    link/ether fa:16:3e:12:2a:03 brd ff:ff:ff:ff:ff:ff
    inet6 fe80::f816:3eff:fe12:2a03/64 scope link
       valid_lft forever preferred_lft forever`), nil)

	// 3. multinic2는 시스템에 존재하지 않음 (에러 발생)
	mockExecutor.On("ExecuteWithTimeout", 
		mock.AnythingOfType("*context.timerCtx"), 
		mock.AnythingOfType("time.Duration"), 
		"ip", 
		[]string{"addr", "show", "multinic2"}).Return([]byte(""), errors.New("Device \"multinic2\" does not exist"))

	// 4. multinic2의 고아 파일 삭제 (롤백 호출)
	mockRollbacker.On("Rollback", ctx, "multinic2").Return(nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, output.TotalDeleted)
	assert.Equal(t, []string{"multinic2"}, output.DeletedInterfaces)
	assert.Empty(t, output.Errors)

	// Mock 검증
	mockFileSystem.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockRollbacker.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_NoOrphanedFiles(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	mockExecutor := new(MockCommandExecutor)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 1. /etc/netplan 디렉토리에 multinic 파일이 없음
	netplanFiles := []string{
		"50-cloud-init.yaml",    // multinic 관련 아님
	}
	mockFileSystem.On("ListFiles", "/etc/netplan").Return(netplanFiles, nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 0, output.TotalDeleted)
	assert.Empty(t, output.DeletedInterfaces)
	assert.Empty(t, output.Errors)

	// Mock 검증
	mockFileSystem.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_RollbackFailure(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	mockExecutor := new(MockCommandExecutor)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 1. 고아 netplan 파일 존재
	netplanFiles := []string{"92-multinic2.yaml"}
	mockFileSystem.On("ListFiles", "/etc/netplan").Return(netplanFiles, nil)

	// 2. multinic2는 시스템에 존재하지 않음
	mockExecutor.On("ExecuteWithTimeout", 
		mock.AnythingOfType("*context.timerCtx"), 
		mock.AnythingOfType("time.Duration"), 
		"ip", 
		[]string{"addr", "show", "multinic2"}).Return([]byte(""), errors.New("Device \"multinic2\" does not exist"))

	// 3. 롤백 실패
	mockRollbacker.On("Rollback", ctx, "multinic2").Return(errors.New("rollback failed"))

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err) // 유스케이스 자체는 성공 (에러는 output.Errors에 포함)
	assert.NotNil(t, output)
	assert.Equal(t, 0, output.TotalDeleted)
	assert.Empty(t, output.DeletedInterfaces)
	assert.Len(t, output.Errors, 1)
	assert.Contains(t, output.Errors[0].Error(), "rollback failed")

	// Mock 검증
	mockFileSystem.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockRollbacker.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_ExtractInterfaceNameFromFile(t *testing.T) {
	useCase := &DeleteNetworkUseCase{}

	tests := []struct {
		fileName     string
		expectedName string
	}{
		{"91-multinic1.yaml", "multinic1"},
		{"92-multinic2.yaml", "multinic2"},
		{"90-multinic0.yaml", "multinic0"},
		{"99-multinic9.yaml", "multinic9"},
		{"50-cloud-init.yaml", ""},
		{"invalid.yaml", ""},
		{"multinic1.yaml", "multinic1"},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			result := useCase.extractInterfaceNameFromFile(tt.fileName)
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestDeleteNetworkUseCase_IsMultinicNetplanFile(t *testing.T) {
	useCase := &DeleteNetworkUseCase{}

	tests := []struct {
		fileName string
		expected bool
	}{
		{"91-multinic1.yaml", true},
		{"92-multinic2.yaml", true},
		{"90-multinic0.yaml", true},
		{"99-multinic9.yaml", true},
		{"50-cloud-init.yaml", false},
		{"multinic1.yaml", false}, // 9로 시작하지 않음
		{"91-multinic1.txt", false}, // yaml이 아님
		{"invalid.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			result := useCase.isMultinicNetplanFile(tt.fileName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
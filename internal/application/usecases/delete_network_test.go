package usecases

import (
	"context"
	"errors"
	"testing"

	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/services"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDeleteNetworkUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	logger := logrus.New()

	namingService := services.NewInterfaceNamingService(mockFileSystem)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 현재 시스템의 multinic 인터페이스 시뮬레이션
	mockFileSystem.On("Exists", "/sys/class/net/multinic0").Return(true)
	mockFileSystem.On("Exists", "/sys/class/net/multinic1").Return(true)
	mockFileSystem.On("Exists", "/sys/class/net/multinic2").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic3").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic4").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic5").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic6").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic7").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic8").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic9").Return(false)

	// DB에서 활성 인터페이스 조회 (multinic1만 DB에 있음)
	activeInterfaces := []entities.NetworkInterface{
		{
			ID:               1,
			MacAddress:       "00:11:22:33:44:55",
			AttachedNodeName: "test-node",
			Status:           entities.StatusConfigured,
		},
	}
	mockRepo.On("GetActiveInterfaces", ctx, "test-node").Return(activeInterfaces, nil)

	// MAC 주소 조회
	mockFileSystem.On("Exists", "/sys/class/net/multinic0/address").Return(true)
	mockFileSystem.On("ReadFile", "/sys/class/net/multinic0/address").Return([]byte("aa:bb:cc:dd:ee:ff\n"), nil)
	mockFileSystem.On("Exists", "/sys/class/net/multinic1/address").Return(true)
	mockFileSystem.On("ReadFile", "/sys/class/net/multinic1/address").Return([]byte("00:11:22:33:44:55\n"), nil)

	// 롤백 실행 (multinic0만 삭제됨)
	mockRollbacker.On("Rollback", ctx, "multinic0").Return(nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, output.TotalDeleted)
	assert.Equal(t, []string{"multinic0"}, output.DeletedInterfaces)
	assert.Empty(t, output.Errors)

	mockRepo.AssertExpectations(t)
	mockRollbacker.AssertExpectations(t)
	mockFileSystem.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_NoOrphanedInterfaces(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	logger := logrus.New()

	namingService := services.NewInterfaceNamingService(mockFileSystem)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 현재 시스템에 multinic 인터페이스 없음
	mockFileSystem.On("Exists", "/sys/class/net/multinic0").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic1").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic2").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic3").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic4").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic5").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic6").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic7").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic8").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic9").Return(false)

	// DB에서 활성 인터페이스 조회
	activeInterfaces := []entities.NetworkInterface{}
	mockRepo.On("GetActiveInterfaces", ctx, "test-node").Return(activeInterfaces, nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 0, output.TotalDeleted)
	assert.Empty(t, output.DeletedInterfaces)
	assert.Empty(t, output.Errors)

	mockRepo.AssertExpectations(t)
	mockFileSystem.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	logger := logrus.New()

	namingService := services.NewInterfaceNamingService(mockFileSystem)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 현재 시스템의 multinic 인터페이스 시뮬레이션
	mockFileSystem.On("Exists", "/sys/class/net/multinic0").Return(true)
	mockFileSystem.On("Exists", "/sys/class/net/multinic1").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic2").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic3").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic4").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic5").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic6").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic7").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic8").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic9").Return(false)

	// DB 조회 실패
	mockRepo.On("GetActiveInterfaces", ctx, "test-node").Return([]entities.NetworkInterface(nil), errors.New("database error"))

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "활성 인터페이스 조회 실패")

	mockRepo.AssertExpectations(t)
	mockFileSystem.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_RollbackError(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	logger := logrus.New()

	namingService := services.NewInterfaceNamingService(mockFileSystem)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 현재 시스템의 multinic 인터페이스 시뮬레이션
	mockFileSystem.On("Exists", "/sys/class/net/multinic0").Return(true)
	mockFileSystem.On("Exists", "/sys/class/net/multinic1").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic2").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic3").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic4").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic5").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic6").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic7").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic8").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic9").Return(false)

	// DB에서 활성 인터페이스 조회 (빈 결과 - multinic0는 고아)
	activeInterfaces := []entities.NetworkInterface{}
	mockRepo.On("GetActiveInterfaces", ctx, "test-node").Return(activeInterfaces, nil)

	// MAC 주소 조회
	mockFileSystem.On("Exists", "/sys/class/net/multinic0/address").Return(true)
	mockFileSystem.On("ReadFile", "/sys/class/net/multinic0/address").Return([]byte("aa:bb:cc:dd:ee:ff\n"), nil)

	// 롤백 실패
	mockRollbacker.On("Rollback", ctx, "multinic0").Return(errors.New("rollback failed"))

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err) // 유스케이스 자체는 성공 (에러는 output.Errors에 포함)
	assert.NotNil(t, output)
	assert.Equal(t, 0, output.TotalDeleted)
	assert.Empty(t, output.DeletedInterfaces)
	assert.Len(t, output.Errors, 1)
	assert.Contains(t, output.Errors[0].Error(), "multinic0 삭제 실패")

	mockRepo.AssertExpectations(t)
	mockRollbacker.AssertExpectations(t)
	mockFileSystem.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_MacAddressReadError(t *testing.T) {
	// Arrange
	mockRepo := new(MockNetworkInterfaceRepository)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	logger := logrus.New()

	namingService := services.NewInterfaceNamingService(mockFileSystem)
	useCase := NewDeleteNetworkUseCase(mockRepo, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	// 현재 시스템의 multinic 인터페이스 시뮬레이션
	mockFileSystem.On("Exists", "/sys/class/net/multinic0").Return(true)
	mockFileSystem.On("Exists", "/sys/class/net/multinic1").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic2").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic3").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic4").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic5").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic6").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic7").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic8").Return(false)
	mockFileSystem.On("Exists", "/sys/class/net/multinic9").Return(false)

	// DB에서 활성 인터페이스 조회
	activeInterfaces := []entities.NetworkInterface{}
	mockRepo.On("GetActiveInterfaces", ctx, "test-node").Return(activeInterfaces, nil)

	// MAC 주소 파일이 존재하지 않음
	mockFileSystem.On("Exists", "/sys/class/net/multinic0/address").Return(false)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 0, output.TotalDeleted) // MAC 주소 읽기 실패로 삭제되지 않음
	assert.Empty(t, output.DeletedInterfaces)
	assert.Empty(t, output.Errors)

	mockRepo.AssertExpectations(t)
	mockFileSystem.AssertExpectations(t)
}
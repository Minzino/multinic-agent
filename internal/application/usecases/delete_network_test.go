package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"multinic-agent-v2/internal/domain/interfaces"
	"multinic-agent-v2/internal/domain/services"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOSDetector is a mock for the OSDetector interface.
type MockOSDetector struct {
	mock.Mock
}

func (m *MockOSDetector) DetectOS() (interfaces.OSType, error) {
	args := m.Called()
	return args.Get(0).(interfaces.OSType), args.Error(1)
}

func TestDeleteNetworkUseCase_Execute_NetplanFileCleanup_Success(t *testing.T) {
	// Arrange
	mockOSDetector := new(MockOSDetector)
	mockRollbacker := new(MockNetworkRollbacker) // Assumes this mock is defined in another test file in the package
	mockFileSystem := new(MockFileSystem)       // Assumes this mock is defined in another test file in the package
	mockExecutor := new(MockCommandExecutor)   // Assumes this mock is defined in another test file in the package
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockOSDetector, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	mockOSDetector.On("DetectOS").Return(interfaces.OSTypeUbuntu, nil)

	netplanFiles := []string{"91-multinic1.yaml", "92-multinic2.yaml"}
	mockFileSystem.On("ListFiles", "/etc/netplan").Return(netplanFiles, nil)

	// For multinic1, the device exists
	mockExecutor.On("ExecuteWithTimeout", mock.Anything, mock.Anything, "ip", []string{"addr", "show", "multinic1"}).Return([]byte("link/ether"), nil)

	// For multinic2, the device does not exist (it's an orphan)
	mockExecutor.On("ExecuteWithTimeout", mock.Anything, mock.Anything, "ip", []string{"addr", "show", "multinic2"}).Return([]byte(""), errors.New("does not exist"))

	mockRollbacker.On("Rollback", ctx, "multinic2").Return(nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, output.TotalDeleted)
	assert.Equal(t, []string{"multinic2"}, output.DeletedInterfaces)
	mockExecutor.AssertExpectations(t)
	mockRollbacker.AssertExpectations(t)
}

func TestDeleteNetworkUseCase_Execute_NmcliCleanup_Success(t *testing.T) {
	// Arrange
	mockOSDetector := new(MockOSDetector)
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	mockExecutor := new(MockCommandExecutor)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockOSDetector, mockRollbacker, namingService, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "rhel-node"}

	mockOSDetector.On("DetectOS").Return(interfaces.OSTypeRHEL, nil)

	nmcliConnections := []string{"multinic0", "multinic1"}
	mockExecutor.On("ExecuteWithTimeout", mock.Anything, mock.Anything, "nmcli", []string{"-t", "-f", "NAME", "c", "show"}).Return([]byte(strings.Join(nmcliConnections, "\n")), nil)

	// For multinic0, the device exists
	mockExecutor.On("ExecuteWithTimeout", mock.Anything, mock.Anything, "ip", []string{"addr", "show", "multinic0"}).Return([]byte("link/ether"), nil)

	// For multinic1, the device does not exist (it's an orphan)
	mockExecutor.On("ExecuteWithTimeout", mock.Anything, mock.Anything, "ip", []string{"addr", "show", "multinic1"}).Return([]byte(""), errors.New("does not exist"))

	mockRollbacker.On("Rollback", ctx, "multinic1").Return(nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, output.TotalDeleted)
	assert.Equal(t, []string{"multinic1"}, output.DeletedInterfaces)
	mockExecutor.AssertExpectations(t)
	mockRollbacker.AssertExpectations(t)
}
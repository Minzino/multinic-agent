package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"multinic-agent/internal/domain/entities"
	"multinic-agent/internal/domain/interfaces"
	"multinic-agent/internal/domain/services"

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
	mockRollbacker := new(MockNetworkRollbacker)
	mockFileSystem := new(MockFileSystem)
	mockExecutor := new(MockCommandExecutor)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockRepository := new(MockNetworkInterfaceRepository)
	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockOSDetector, mockRollbacker, namingService, mockRepository, mockFileSystem, logger)

	ctx := context.Background()
	input := DeleteNetworkInput{NodeName: "test-node"}

	mockOSDetector.On("DetectOS").Return(interfaces.OSTypeUbuntu, nil)

	// Setup hostname
	mockExecutor.On("ExecuteWithTimeout", mock.Anything, mock.Anything, "hostname", mock.Anything).Return([]byte("test-node\n"), nil)

	// Setup netplan files
	netplanFiles := []string{"91-multinic1.yaml", "92-multinic2.yaml"}
	mockFileSystem.On("ListFiles", "/etc/netplan").Return(netplanFiles, nil)

	// Setup active interfaces from DB (only multinic1 is active)
	activeInterfaces := []entities.NetworkInterface{
		{ID: 1, MacAddress: "fa:16:3e:11:11:11", AttachedNodeName: "test-node"},
	}
	mockRepository.On("GetAllNodeInterfaces", ctx, "test-node").Return(activeInterfaces, nil)

	// Setup netplan file content for multinic1 (active)
	multinic1Content := `network:
  ethernets:
    multinic1:
      match:
        macaddress: fa:16:3e:11:11:11
      dhcp4: false
  version: 2`
	mockFileSystem.On("ReadFile", "/etc/netplan/91-multinic1.yaml").Return([]byte(multinic1Content), nil)

	// Setup netplan file content for multinic2 (orphan)
	multinic2Content := `network:
  ethernets:
    multinic2:
      match:
        macaddress: fa:16:3e:22:22:22
      dhcp4: false
  version: 2`
	mockFileSystem.On("ReadFile", "/etc/netplan/92-multinic2.yaml").Return([]byte(multinic2Content), nil)

	mockRollbacker.On("Rollback", ctx, "multinic2").Return(nil)

	// Act
	output, err := useCase.Execute(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, output.TotalDeleted)
	assert.Equal(t, []string{"multinic2"}, output.DeletedInterfaces)
	mockRepository.AssertExpectations(t)
	mockFileSystem.AssertExpectations(t)
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

	mockRepository := new(MockNetworkInterfaceRepository)
	namingService := services.NewInterfaceNamingService(mockFileSystem, mockExecutor)
	useCase := NewDeleteNetworkUseCase(mockOSDetector, mockRollbacker, namingService, mockRepository, mockFileSystem, logger)

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
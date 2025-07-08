package services

import (
	"fmt"
	"os"
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileSystem은 FileSystem 인터페이스의 목 구현체입니다
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

func TestInterfaceNamingService_GenerateNextName(t *testing.T) {
	tests := []struct {
		name           string
		existingIfaces []string
		expectedName   string
		wantError      bool
	}{
		{
			name:           "모든 인터페이스가 비어있음 - multinic0 반환",
			existingIfaces: []string{},
			expectedName:   "multinic0",
			wantError:      false,
		},
		{
			name:           "multinic0이 사용 중 - multinic1 반환",
			existingIfaces: []string{"multinic0"},
			expectedName:   "multinic1",
			wantError:      false,
		},
		{
			name:           "multinic0, multinic1이 사용 중 - multinic2 반환",
			existingIfaces: []string{"multinic0", "multinic1"},
			expectedName:   "multinic2",
			wantError:      false,
		},
		{
			name:           "일부 인터페이스가 건너뛰어짐 - 가장 작은 번호 반환",
			existingIfaces: []string{"multinic0", "multinic2", "multinic4"},
			expectedName:   "multinic1",
			wantError:      false,
		},
		{
			name: "모든 인터페이스가 사용 중 - 에러 반환",
			existingIfaces: []string{
				"multinic0", "multinic1", "multinic2", "multinic3", "multinic4",
				"multinic5", "multinic6", "multinic7", "multinic8", "multinic9",
			},
			expectedName: "",
			wantError:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := new(MockFileSystem)
			
			// 순차적으로 호출될 것으로 예상되는 인터페이스들만 Mock 설정
			if tt.wantError {
				// 모든 인터페이스가 사용 중인 경우 - 모든 호출이 true
				for i := 0; i < 10; i++ {
					interfacePath := fmt.Sprintf("/sys/class/net/multinic%d", i)
					mockFS.On("Exists", interfacePath).Return(true).Once()
				}
			} else {
				// 첫 번째 사용 가능한 인터페이스까지만 호출됨
				expectedIndex := 0
				if tt.expectedName == "multinic1" {
					expectedIndex = 1
				} else if tt.expectedName == "multinic2" {
					expectedIndex = 2
				}
				
				// 0부터 expectedIndex까지 순차적으로 호출
				for i := 0; i <= expectedIndex; i++ {
					interfacePath := fmt.Sprintf("/sys/class/net/multinic%d", i)
					interfaceName := fmt.Sprintf("multinic%d", i)
					
					// 기존 인터페이스 목록에 있으면 true, 없으면 false
					exists := false
					for _, existing := range tt.existingIfaces {
						if existing == interfaceName {
							exists = true
							break
						}
					}
					mockFS.On("Exists", interfacePath).Return(exists).Once()
				}
			}
			
			service := NewInterfaceNamingService(mockFS)
			result, err := service.GenerateNextName()
			
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "사용 가능한 인터페이스 이름이 없습니다")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, result.String())
			}
			
			mockFS.AssertExpectations(t)
		})
	}
}

func TestInterfaceNamingService_isInterfaceInUse(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		exists        bool
		expected      bool
	}{
		{
			name:          "인터페이스가 존재함",
			interfaceName: "multinic0",
			exists:        true,
			expected:      true,
		},
		{
			name:          "인터페이스가 존재하지 않음",
			interfaceName: "multinic1",
			exists:        false,
			expected:      false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := new(MockFileSystem)
			expectedPath := fmt.Sprintf("/sys/class/net/%s", tt.interfaceName)
			mockFS.On("Exists", expectedPath).Return(tt.exists)
			
			service := NewInterfaceNamingService(mockFS)
			result := service.isInterfaceInUse(tt.interfaceName)
			
			assert.Equal(t, tt.expected, result)
			mockFS.AssertExpectations(t)
		})
	}
}
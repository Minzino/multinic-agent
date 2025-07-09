package adapters

import (
	"os"
	"testing"

	"multinic-agent-v2/internal/domain/interfaces"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileSystemForOSDetector는 OS 감지용 Mock FileSystem입니다
type MockFileSystemForOSDetector struct {
	mock.Mock
}

func (m *MockFileSystemForOSDetector) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileSystemForOSDetector) WriteFile(path string, data []byte, perm os.FileMode) error {
	args := m.Called(path, data, perm)
	return args.Error(0)
}

func (m *MockFileSystemForOSDetector) Exists(path string) bool {
	args := m.Called(path)
	return args.Bool(0)
}

func (m *MockFileSystemForOSDetector) MkdirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

func (m *MockFileSystemForOSDetector) Remove(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockFileSystemForOSDetector) ListFiles(path string) ([]string, error) {
	args := m.Called(path)
	return args.Get(0).([]string), args.Error(1)
}

func TestRealOSDetector_DetectOS(t *testing.T) {
	tests := []struct {
		name         string
		issueContent string
		readError    error
		expectedOS   interfaces.OSType
		expectError  bool
	}{
		{
			name:         "Ubuntu 시스템 감지",
			issueContent: "Ubuntu 22.04.3 LTS \\n \\l",
			readError:    nil,
			expectedOS:   interfaces.OSTypeUbuntu,
			expectError:  false,
		},
		{
			name:         "SUSE 시스템 감지",
			issueContent: "SUSE Liberty Linux 9.4 (Plow)\nKernel \\r on an \\m",
			readError:    nil,
			expectedOS:   interfaces.OSTypeSUSE,
			expectError:  false,
		},
		{
			name:         "대소문자 구분 없이 Ubuntu 감지",
			issueContent: "UBUNTU 20.04 LTS",
			readError:    nil,
			expectedOS:   interfaces.OSTypeUbuntu,
			expectError:  false,
		},
		{
			name:         "대소문자 구분 없이 SUSE 감지",
			issueContent: "suse enterprise server 15",
			readError:    nil,
			expectedOS:   interfaces.OSTypeSUSE,
			expectError:  false,
		},
		{
			name:         "알 수 없는 OS - 기본값 Ubuntu",
			issueContent: "Red Hat Enterprise Linux 8",
			readError:    nil,
			expectedOS:   interfaces.OSTypeUbuntu,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := new(MockFileSystemForOSDetector)

			if tt.readError != nil {
				mockFS.On("ReadFile", "/etc/issue").Return([]byte{}, tt.readError)
			} else {
				mockFS.On("ReadFile", "/etc/issue").Return([]byte(tt.issueContent), nil)
			}

			detector := NewRealOSDetector(mockFS)
			result, err := detector.DetectOS()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOS, result)
			}

			mockFS.AssertExpectations(t)
		})
	}
}

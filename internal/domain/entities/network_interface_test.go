package entities

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkInterface_Validate(t *testing.T) {
	tests := []struct {
		name      string
		iface     NetworkInterface
		wantError bool
		errorType error
	}{
		{
			name: "유효한 인터페이스",
			iface: NetworkInterface{
				ID:               1,
				MacAddress:       "00:11:22:33:44:55",
				AttachedNodeName: "test-node",
				IPAddress:        "192.168.1.100",
				SubnetMask:       "255.255.255.0",
				Gateway:          "192.168.1.1",
				DNS:              "8.8.8.8",
				VLAN:             0,
				Status:           StatusPending,
			},
			wantError: false,
		},
		{
			name: "잘못된 MAC 주소 형식",
			iface: NetworkInterface{
				ID:               1,
				MacAddress:       "invalid-mac",
				AttachedNodeName: "test-node",
				Status:           StatusPending,
			},
			wantError: true,
			errorType: ErrInvalidMacAddress,
		},
		{
			name: "빈 노드 이름",
			iface: NetworkInterface{
				ID:         1,
				MacAddress: "00:11:22:33:44:55",
				Status:     StatusPending,
			},
			wantError: true,
			errorType: ErrInvalidNodeName,
		},
		{
			name: "다양한 MAC 주소 형식 - 콜론",
			iface: NetworkInterface{
				MacAddress:       "aa:bb:cc:dd:ee:ff",
				AttachedNodeName: "test-node",
			},
			wantError: false,
		},
		{
			name: "다양한 MAC 주소 형식 - 대시",
			iface: NetworkInterface{
				MacAddress:       "AA-BB-CC-DD-EE-FF",
				AttachedNodeName: "test-node",
			},
			wantError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.iface.Validate()
			
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNetworkInterface_StatusMethods(t *testing.T) {
	t.Run("IsPending", func(t *testing.T) {
		iface := NetworkInterface{Status: StatusPending}
		assert.True(t, iface.IsPending())
		
		iface.Status = StatusConfigured
		assert.False(t, iface.IsPending())
	})
	
	t.Run("MarkAsConfigured", func(t *testing.T) {
		iface := NetworkInterface{Status: StatusPending}
		iface.MarkAsConfigured()
		assert.Equal(t, StatusConfigured, iface.Status)
	})
	
	t.Run("MarkAsFailed", func(t *testing.T) {
		iface := NetworkInterface{Status: StatusPending}
		iface.MarkAsFailed()
		assert.Equal(t, StatusFailed, iface.Status)
	})
}

func TestInterfaceName_NewInterfaceName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "유효한 인터페이스 이름 - multinic0",
			input:     "multinic0",
			wantError: false,
		},
		{
			name:      "유효한 인터페이스 이름 - multinic9",
			input:     "multinic9",
			wantError: false,
		},
		{
			name:      "잘못된 인터페이스 이름 - multinic10",
			input:     "multinic10",
			wantError: true,
		},
		{
			name:      "잘못된 인터페이스 이름 - eth0",
			input:     "eth0",
			wantError: true,
		},
		{
			name:      "잘못된 인터페이스 이름 - 빈 문자열",
			input:     "",
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewInterfaceName(tt.input)
			
			if tt.wantError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidInterfaceName)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, result.String())
			}
		})
	}
}

func TestInterfaceName_String(t *testing.T) {
	name, err := NewInterfaceName("multinic5")
	require.NoError(t, err)
	
	assert.Equal(t, "multinic5", name.String())
}

func TestMacAddressValidation(t *testing.T) {
	tests := []struct {
		name      string
		macAddr   string
		wantValid bool
	}{
		{"유효한 MAC - 소문자 콜론", "00:11:22:33:44:55", true},
		{"유효한 MAC - 대문자 콜론", "AA:BB:CC:DD:EE:FF", true},
		{"유효한 MAC - 대시", "00-11-22-33-44-55", true},
		{"유효한 MAC - 혼합", "aA:bB:cC:dD:eE:fF", true},
		{"잘못된 MAC - 짧음", "00:11:22:33:44", false},
		{"잘못된 MAC - 길음", "00:11:22:33:44:55:66", false},
		{"잘못된 MAC - 잘못된 문자", "00:11:22:33:44:GG", false},
		{"잘못된 MAC - 형식 오류", "00112233445", false},
		{"빈 문자열", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidMacAddress(tt.macAddr)
			assert.Equal(t, tt.wantValid, result)
		})
	}
}

func TestInterfaceNameValidation(t *testing.T) {
	tests := []struct {
		name      string
		ifaceName string
		wantValid bool
	}{
		{"유효한 이름 - multinic0", "multinic0", true},
		{"유효한 이름 - multinic1", "multinic1", true},
		{"유효한 이름 - multinic9", "multinic9", true},
		{"잘못된 이름 - multinic10", "multinic10", false},
		{"잘못된 이름 - eth0", "eth0", false},
		{"잘못된 이름 - ens33", "ens33", false},
		{"잘못된 이름 - multinica", "multinica", false},
		{"빈 문자열", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidInterfaceName(tt.ifaceName)
			assert.Equal(t, tt.wantValid, result)
		})
	}
}
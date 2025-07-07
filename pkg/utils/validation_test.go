package utils

import (
	"testing"
)

func TestValidateInterfaceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"유효한 인터페이스 - multinic0", "multinic0", false},
		{"유효한 인터페이스 - multinic9", "multinic9", false},
		{"빈 문자열", "", true},
		{"잘못된 형식 - eth0", "eth0", true},
		{"잘못된 형식 - multinic10", "multinic10", true},
		{"잘못된 형식 - multinic", "multinic", true},
		{"잘못된 형식 - multinica", "multinica", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInterfaceName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"유효한 호스트네임", "my-host", false},
		{"유효한 호스트네임 with 점", "my.host.example", false},
		{"빈 문자열", "", true},
		{"너무 긴 호스트네임", string(make([]byte, 254)), true},
		{"잘못된 문자 포함", "my_host", true},
		{"특수문자 시작", "-myhost", true},
		{"특수문자 끝", "myhost-", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNetplanConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  []byte
		wantErr bool
	}{
		{
			"유효한 설정",
			[]byte(`network:
  version: 2
  ethernets:
    multinic0:
      dhcp4: true`),
			false,
		},
		{
			"빈 설정",
			[]byte{},
			true,
		},
		{
			"network 섹션 없음",
			[]byte(`version: 2`),
			true,
		},
		{
			"보호된 인터페이스 포함 - eth0",
			[]byte(`network:
  ethernets:
    eth0:
      dhcp4: true`),
			true,
		},
		{
			"보호된 인터페이스 포함 - ens",
			[]byte(`network:
  ethernets:
    ens33:
      dhcp4: true`),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetplanConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNetplanConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDatabaseConfig(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		user     string
		password string
		database string
		wantErr  bool
	}{
		{"모든 필드 유효", "localhost", "3306", "root", "password", "testdb", false},
		{"호스트 비어있음", "", "3306", "root", "password", "testdb", true},
		{"포트 비어있음", "localhost", "", "root", "password", "testdb", true},
		{"사용자 비어있음", "localhost", "3306", "", "password", "testdb", true},
		{"패스워드 비어있음", "localhost", "3306", "root", "", "testdb", true},
		{"데이터베이스 비어있음", "localhost", "3306", "root", "password", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDatabaseConfig(tt.host, tt.port, tt.user, tt.password, tt.database)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDatabaseConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
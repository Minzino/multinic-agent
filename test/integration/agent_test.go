// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"multinic-agent-v2/pkg/db"
	"multinic-agent-v2/pkg/network"
	"github.com/sirupsen/logrus"
)

func TestAgentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트는 -short 플래그와 함께 실행시 스킵됩니다")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// 테스트용 DB 설정
	dbConfig := db.Config{
		Host:     "localhost",
		Port:     "3306",
		User:     "test",
		Password: "test",
		Database: "multinic_test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("데이터베이스 연결 테스트", func(t *testing.T) {
		client, err := db.NewClient(dbConfig, logger)
		if err != nil {
			t.Skipf("데이터베이스 연결 실패 (테스트 DB가 없을 수 있음): %v", err)
		}
		defer client.Close()

		// 연결 확인
		configs, err := client.GetVMConfigs()
		if err != nil {
			t.Errorf("VM 설정 조회 실패: %v", err)
		}
		t.Logf("조회된 VM 설정 수: %d", len(configs))
	})

	t.Run("네트워크 매니저 생성 테스트", func(t *testing.T) {
		manager, err := network.NewNetworkManager(logger)
		if err != nil {
			t.Fatalf("네트워크 매니저 생성 실패: %v", err)
		}

		t.Logf("네트워크 매니저 타입: %s", manager.GetType())

		// 인터페이스 검증 테스트
		validNames := []string{"multinic0", "multinic5", "multinic9"}
		for _, name := range validNames {
			if !manager.ValidateInterface(name) {
				t.Errorf("%s는 유효한 인터페이스여야 함", name)
			}
		}

		invalidNames := []string{"eth0", "multinic10", "ens33"}
		for _, name := range invalidNames {
			if manager.ValidateInterface(name) {
				t.Errorf("%s는 유효하지 않은 인터페이스여야 함", name)
			}
		}
	})
}

func TestNetworkConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("통합 테스트는 -short 플래그와 함께 실행시 스킵됩니다")
	}

	logger := logrus.New()
	manager, err := network.NewNetworkManager(logger)
	if err != nil {
		t.Fatalf("네트워크 매니저 생성 실패: %v", err)
	}

	// 테스트 설정
	testConfig := []byte(`network:
  version: 2
  renderer: networkd
  ethernets:
    multinic0:
      dhcp4: true
      dhcp6: false`)

	t.Run("설정 적용 시뮬레이션", func(t *testing.T) {
		// 실제 시스템에 영향을 주지 않기 위해 dry-run 모드가 있다면 사용
		// 여기서는 유효성 검사만 수행
		if !manager.ValidateInterface("multinic0") {
			t.Error("multinic0 인터페이스 검증 실패")
		}
		
		t.Log("설정 적용 시뮬레이션 성공")
	})
}
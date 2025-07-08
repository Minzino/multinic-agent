package usecases

import (
	"context"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"multinic-agent-v2/internal/domain/services"

	"github.com/sirupsen/logrus"
)

// ConfigureNetworkUseCase는 네트워크 설정을 처리하는 유스케이스입니다
type ConfigureNetworkUseCase struct {
	repository    interfaces.NetworkInterfaceRepository
	configurer    interfaces.NetworkConfigurer
	rollbacker    interfaces.NetworkRollbacker
	namingService *services.InterfaceNamingService
	logger        *logrus.Logger
}

// NewConfigureNetworkUseCase는 새로운 ConfigureNetworkUseCase를 생성합니다
func NewConfigureNetworkUseCase(
	repo interfaces.NetworkInterfaceRepository,
	configurer interfaces.NetworkConfigurer,
	rollbacker interfaces.NetworkRollbacker,
	naming *services.InterfaceNamingService,
	logger *logrus.Logger,
) *ConfigureNetworkUseCase {
	return &ConfigureNetworkUseCase{
		repository:    repo,
		configurer:    configurer,
		rollbacker:    rollbacker,
		namingService: naming,
		logger:        logger,
	}
}

// ConfigureNetworkInput은 유스케이스의 입력 파라미터입니다
type ConfigureNetworkInput struct {
	NodeName string
}

// ConfigureNetworkOutput은 유스케이스의 출력 결과입니다
type ConfigureNetworkOutput struct {
	ProcessedCount int
	FailedCount    int
	TotalCount     int
}

// Execute는 네트워크 설정 유스케이스를 실행합니다
func (uc *ConfigureNetworkUseCase) Execute(ctx context.Context, input ConfigureNetworkInput) (*ConfigureNetworkOutput, error) {
	// 1. 대기 중인 인터페이스 조회
	pendingInterfaces, err := uc.repository.GetPendingInterfaces(ctx, input.NodeName)
	if err != nil {
		return nil, errors.NewSystemError("대기 중인 인터페이스 조회 실패", err)
	}

	if len(pendingInterfaces) == 0 {
		return &ConfigureNetworkOutput{
			ProcessedCount: 0,
			FailedCount:    0,
			TotalCount:     0,
		}, nil
	}

	uc.logger.WithFields(logrus.Fields{
		"node_name":     input.NodeName,
		"pending_count": len(pendingInterfaces),
	}).Info("대기 중인 인터페이스 발견")

	var processedCount, failedCount int

	// 2. 각 인터페이스 처리
	for _, iface := range pendingInterfaces {
		if err := uc.processInterface(ctx, iface); err != nil {
			uc.logger.WithFields(logrus.Fields{
				"interface_id": iface.ID,
				"error":        err,
			}).Error("인터페이스 설정 실패")
			failedCount++

			// 실패 상태로 업데이트
			if updateErr := uc.repository.UpdateInterfaceStatus(ctx, iface.ID, entities.StatusFailed); updateErr != nil {
				uc.logger.WithError(updateErr).Error("인터페이스 상태 업데이트 실패")
			}
		} else {
			processedCount++
		}
	}

	return &ConfigureNetworkOutput{
		ProcessedCount: processedCount,
		FailedCount:    failedCount,
		TotalCount:     len(pendingInterfaces),
	}, nil
}

// processInterface는 개별 인터페이스를 처리합니다
func (uc *ConfigureNetworkUseCase) processInterface(ctx context.Context, iface entities.NetworkInterface) error {
	// 1. 유효성 검증
	if err := iface.Validate(); err != nil {
		return errors.NewValidationError("인터페이스 유효성 검증 실패", err)
	}

	// 2. 인터페이스 이름 생성
	interfaceName, err := uc.namingService.GenerateNextName()
	if err != nil {
		return errors.NewSystemError("인터페이스 이름 생성 실패", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   iface.ID,
		"interface_name": interfaceName.String(),
		"mac_address":    iface.MacAddress,
	}).Info("인터페이스 설정 시작")

	// 3. 네트워크 설정 적용
	if err := uc.configurer.Configure(ctx, iface, interfaceName); err != nil {
		// 롤백 시도
		if rollbackErr := uc.rollbacker.Rollback(ctx, interfaceName.String()); rollbackErr != nil {
			uc.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("네트워크 설정 적용 실패", err)
	}

	// 4. 설정 검증
	if err := uc.configurer.Validate(ctx, interfaceName); err != nil {
		// 검증 실패 시 롤백
		if rollbackErr := uc.rollbacker.Rollback(ctx, interfaceName.String()); rollbackErr != nil {
			uc.logger.WithError(rollbackErr).Error("롤백 실패")
		}
		return errors.NewNetworkError("네트워크 설정 검증 실패", err)
	}

	// 5. 성공 상태로 업데이트
	if err := uc.repository.UpdateInterfaceStatus(ctx, iface.ID, entities.StatusConfigured); err != nil {
		return errors.NewSystemError("인터페이스 상태 업데이트 실패", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_id":   iface.ID,
		"interface_name": interfaceName.String(),
	}).Info("인터페이스 설정 성공")

	return nil
}

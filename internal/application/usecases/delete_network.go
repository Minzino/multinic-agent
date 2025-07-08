package usecases

import (
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/interfaces"
	"multinic-agent-v2/internal/domain/services"

	"github.com/sirupsen/logrus"
)

// DeleteNetworkInput은 네트워크 삭제 유스케이스의 입력 데이터입니다
type DeleteNetworkInput struct {
	NodeName string
}

// DeleteNetworkOutput은 네트워크 삭제 유스케이스의 출력 데이터입니다
type DeleteNetworkOutput struct {
	DeletedInterfaces []string
	TotalDeleted      int
	Errors            []error
}

// DeleteNetworkUseCase는 고아 인터페이스를 감지하고 삭제하는 유스케이스입니다
type DeleteNetworkUseCase struct {
	repository    interfaces.NetworkInterfaceRepository
	rollbacker    interfaces.NetworkRollbacker
	namingService *services.InterfaceNamingService
	logger        *logrus.Logger
}

// NewDeleteNetworkUseCase는 새로운 DeleteNetworkUseCase를 생성합니다
func NewDeleteNetworkUseCase(
	repository interfaces.NetworkInterfaceRepository,
	rollbacker interfaces.NetworkRollbacker,
	namingService *services.InterfaceNamingService,
	logger *logrus.Logger,
) *DeleteNetworkUseCase {
	return &DeleteNetworkUseCase{
		repository:    repository,
		rollbacker:    rollbacker,
		namingService: namingService,
		logger:        logger,
	}
}

// Execute는 고아 인터페이스 삭제 유스케이스를 실행합니다
func (uc *DeleteNetworkUseCase) Execute(ctx context.Context, input DeleteNetworkInput) (*DeleteNetworkOutput, error) {
	uc.logger.WithFields(logrus.Fields{
		"node_name": input.NodeName,
	}).Info("고아 인터페이스 삭제 프로세스 시작")

	output := &DeleteNetworkOutput{
		DeletedInterfaces: []string{},
		TotalDeleted:      0,
		Errors:            []error{},
	}

	// 1. 현재 시스템에 생성된 multinic 인터페이스 목록 조회
	currentInterfaces := uc.namingService.GetCurrentMultinicInterfaces()
	if len(currentInterfaces) == 0 {
		uc.logger.Info("현재 시스템에 multinic 인터페이스가 없습니다")
		return output, nil
	}

	uc.logger.WithFields(logrus.Fields{
		"current_interfaces": len(currentInterfaces),
	}).Info("현재 시스템의 multinic 인터페이스 확인 완료")

	// 2. 데이터베이스에서 현재 노드의 활성 인터페이스 목록 조회
	activeInterfaces, err := uc.repository.GetActiveInterfaces(ctx, input.NodeName)
	if err != nil {
		return nil, fmt.Errorf("활성 인터페이스 조회 실패: %w", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"active_interfaces": len(activeInterfaces),
	}).Info("데이터베이스에서 활성 인터페이스 조회 완료")

	// 3. 현재 인터페이스와 DB 인터페이스 비교하여 고아 인터페이스 식별
	orphanedInterfaces := uc.findOrphanedInterfaces(currentInterfaces, activeInterfaces)
	if len(orphanedInterfaces) == 0 {
		uc.logger.Info("삭제 대상 고아 인터페이스가 없습니다")
		return output, nil
	}

	uc.logger.WithFields(logrus.Fields{
		"orphaned_interfaces": len(orphanedInterfaces),
	}).Info("고아 인터페이스 감지 완료")

	// 4. 각 고아 인터페이스 삭제 처리
	for _, interfaceName := range orphanedInterfaces {
		if err := uc.deleteInterface(ctx, interfaceName); err != nil {
			uc.logger.WithFields(logrus.Fields{
				"interface_name": interfaceName.String(),
				"error":          err.Error(),
			}).Error("인터페이스 삭제 실패")
			output.Errors = append(output.Errors, fmt.Errorf("인터페이스 %s 삭제 실패: %w", interfaceName.String(), err))
		} else {
			output.DeletedInterfaces = append(output.DeletedInterfaces, interfaceName.String())
			output.TotalDeleted++
			uc.logger.WithFields(logrus.Fields{
				"interface_name": interfaceName.String(),
			}).Info("인터페이스 삭제 완료")
		}
	}

	uc.logger.WithFields(logrus.Fields{
		"total_deleted": output.TotalDeleted,
		"total_errors":  len(output.Errors),
	}).Info("고아 인터페이스 삭제 프로세스 완료")

	return output, nil
}

// findOrphanedInterfaces는 현재 인터페이스와 DB 인터페이스를 비교하여 고아 인터페이스를 찾습니다
func (uc *DeleteNetworkUseCase) findOrphanedInterfaces(
	currentInterfaces []entities.InterfaceName,
	activeInterfaces []entities.NetworkInterface,
) []entities.InterfaceName {
	// DB 인터페이스의 MAC 주소 맵 생성
	activeMacs := make(map[string]bool)
	for _, iface := range activeInterfaces {
		activeMacs[iface.MacAddress] = true
	}

	var orphaned []entities.InterfaceName

	// 현재 인터페이스 중 DB에 없는 MAC 주소를 가진 인터페이스 찾기
	for _, currentInterface := range currentInterfaces {
		macAddress, err := uc.namingService.GetMacAddressForInterface(currentInterface.String())
		if err != nil {
			uc.logger.WithFields(logrus.Fields{
				"interface_name": currentInterface.String(),
				"error":          err.Error(),
			}).Warn("인터페이스 MAC 주소 조회 실패")
			continue
		}

		if !activeMacs[macAddress] {
			uc.logger.WithFields(logrus.Fields{
				"interface_name": currentInterface.String(),
				"mac_address":    macAddress,
			}).Info("고아 인터페이스 발견")
			orphaned = append(orphaned, currentInterface)
		}
	}

	return orphaned
}

// deleteInterface는 특정 인터페이스를 삭제합니다
func (uc *DeleteNetworkUseCase) deleteInterface(ctx context.Context, interfaceName entities.InterfaceName) error {
	uc.logger.WithFields(logrus.Fields{
		"interface_name": interfaceName.String(),
	}).Info("인터페이스 삭제 시작")

	// 네트워크 인터페이스 롤백 (설정 제거)
	if err := uc.rollbacker.Rollback(ctx, interfaceName.String()); err != nil {
		return fmt.Errorf("네트워크 인터페이스 롤백 실패: %w", err)
	}

	uc.logger.WithFields(logrus.Fields{
		"interface_name": interfaceName.String(),
	}).Info("인터페이스 삭제 성공")

	return nil
}
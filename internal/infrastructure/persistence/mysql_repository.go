package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"multinic-agent-v2/internal/domain/entities"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
)

// MySQLRepository는 MySQL 기반의 NetworkInterfaceRepository 구현체입니다
type MySQLRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

// NewMySQLRepository는 새로운 MySQLRepository를 생성합니다
func NewMySQLRepository(db *sql.DB, logger *logrus.Logger) interfaces.NetworkInterfaceRepository {
	return &MySQLRepository{
		db:     db,
		logger: logger,
	}
}

// GetPendingInterfaces는 특정 노드의 설정 대기 중인 인터페이스들을 조회합니다
func (r *MySQLRepository) GetPendingInterfaces(ctx context.Context, nodeName string) ([]entities.NetworkInterface, error) {
	query := `
		SELECT id, macaddress, attached_node_name, ip_address, 
		       subnet_mask, gateway, dns, vlan
		FROM multi_interface
		WHERE netplan_success = 0 
		AND attached_node_name = ?
		AND deleted_at IS NULL
		LIMIT 10
	`
	
	rows, err := r.db.QueryContext(ctx, query, nodeName)
	if err != nil {
		return nil, errors.NewSystemError("데이터베이스 조회 실패", err)
	}
	defer rows.Close()
	
	var interfaces []entities.NetworkInterface
	
	for rows.Next() {
		var iface entities.NetworkInterface
		var ipAddress, subnetMask, gateway, dns sql.NullString
		
		err := rows.Scan(
			&iface.ID,
			&iface.MacAddress,
			&iface.AttachedNodeName,
			&ipAddress,
			&subnetMask,
			&gateway,
			&dns,
			&iface.VLAN,
		)
		if err != nil {
			r.logger.WithError(err).Error("행 스캔 실패")
			continue
		}
		
		// NULL 값 처리
		if ipAddress.Valid {
			iface.IPAddress = ipAddress.String
		}
		if subnetMask.Valid {
			iface.SubnetMask = subnetMask.String
		}
		if gateway.Valid {
			iface.Gateway = gateway.String
		}
		if dns.Valid {
			iface.DNS = dns.String
		}
		
		iface.Status = entities.StatusPending
		interfaces = append(interfaces, iface)
	}
	
	if err = rows.Err(); err != nil {
		return nil, errors.NewSystemError("결과 처리 중 오류", err)
	}
	
	return interfaces, nil
}

// UpdateInterfaceStatus는 인터페이스의 설정 상태를 업데이트합니다
func (r *MySQLRepository) UpdateInterfaceStatus(ctx context.Context, interfaceID int, status entities.InterfaceStatus) error {
	var netplanSuccess int
	switch status {
	case entities.StatusConfigured:
		netplanSuccess = 1
	case entities.StatusFailed:
		netplanSuccess = 0
	default:
		netplanSuccess = 0
	}
	
	query := `
		UPDATE multi_interface 
		SET netplan_success = ?, modified_at = NOW()
		WHERE id = ?
	`
	
	result, err := r.db.ExecContext(ctx, query, netplanSuccess, interfaceID)
	if err != nil {
		return errors.NewSystemError("상태 업데이트 실패", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.NewSystemError("영향받은 행 확인 실패", err)
	}
	
	if rowsAffected == 0 {
		return errors.NewNotFoundError(fmt.Sprintf("인터페이스를 찾을 수 없음: ID=%d", interfaceID))
	}
	
	r.logger.WithFields(logrus.Fields{
		"interface_id": interfaceID,
		"status":       status,
	}).Info("인터페이스 상태 업데이트 완료")
	
	return nil
}

// GetInterfaceByID는 ID로 인터페이스를 조회합니다
func (r *MySQLRepository) GetInterfaceByID(ctx context.Context, id int) (*entities.NetworkInterface, error) {
	query := `
		SELECT id, macaddress, attached_node_name, ip_address,
		       subnet_mask, gateway, dns, vlan, netplan_success
		FROM multi_interface
		WHERE id = ? AND deleted_at IS NULL
	`
	
	var iface entities.NetworkInterface
	var ipAddress, subnetMask, gateway, dns sql.NullString
	var netplanSuccess int
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&iface.ID,
		&iface.MacAddress,
		&iface.AttachedNodeName,
		&ipAddress,
		&subnetMask,
		&gateway,
		&dns,
		&iface.VLAN,
		&netplanSuccess,
	)
	
	if err == sql.ErrNoRows {
		return nil, errors.NewNotFoundError(fmt.Sprintf("인터페이스를 찾을 수 없음: ID=%d", id))
	}
	if err != nil {
		return nil, errors.NewSystemError("데이터베이스 조회 실패", err)
	}
	
	// NULL 값 처리
	if ipAddress.Valid {
		iface.IPAddress = ipAddress.String
	}
	if subnetMask.Valid {
		iface.SubnetMask = subnetMask.String
	}
	if gateway.Valid {
		iface.Gateway = gateway.String
	}
	if dns.Valid {
		iface.DNS = dns.String
	}
	
	// 상태 매핑
	switch netplanSuccess {
	case 1:
		iface.Status = entities.StatusConfigured
	default:
		iface.Status = entities.StatusPending
	}
	
	return &iface, nil
}
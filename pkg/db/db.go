package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type Client struct {
	db     *sql.DB
	logger *logrus.Logger
}

type MultiInterface struct {
	ID               int
	AttachedNodeName string
	MacAddress       string
	NetplanSuccess   int
	IPAddress        string
	SubnetMask       string
	Gateway          string
	DNS              string
	VLAN             int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewClient(cfg Config, logger *logrus.Logger) (*Client, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("데이터베이스 연결 실패: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("데이터베이스 핑 실패: %w", err)
	}

	return &Client{
		db:     db,
		logger: logger,
	}, nil
}

func (c *Client) GetPendingInterfaces(nodeName string) ([]MultiInterface, error) {
	query := `
		SELECT id, attached_node_name, mac_address, netplan_success, 
			   ip_address, subnet_mask, gateway, dns, vlan, created_at, updated_at
		FROM multi_interface 
		WHERE netplan_success = 0 AND attached_node_name = ?
		ORDER BY id
		LIMIT 10
	`
	
	rows, err := c.db.Query(query, nodeName)
	if err != nil {
		return nil, fmt.Errorf("다중 인터페이스 쿼리 실패: %w", err)
	}
	defer rows.Close()

	var interfaces []MultiInterface
	for rows.Next() {
		var iface MultiInterface
		var dns sql.NullString
		
		if err := rows.Scan(&iface.ID, &iface.AttachedNodeName, &iface.MacAddress, 
			&iface.NetplanSuccess, &iface.IPAddress, &iface.SubnetMask, 
			&iface.Gateway, &dns, &iface.VLAN, &iface.CreatedAt, &iface.UpdatedAt); err != nil {
			c.logger.WithError(err).Error("다중 인터페이스 스캔 실패")
			continue
		}
		
		if dns.Valid {
			iface.DNS = dns.String
		}
		
		interfaces = append(interfaces, iface)
	}

	return interfaces, rows.Err()
}

func (c *Client) UpdateInterfaceStatus(interfaceID int, success bool) error {
	status := 0
	if success {
		status = 1
	}

	query := `
		UPDATE multi_interface 
		SET netplan_success = ?, updated_at = NOW() 
		WHERE id = ?
	`
	
	_, err := c.db.Exec(query, status, interfaceID)
	if err != nil {
		return fmt.Errorf("인터페이스 상태 업데이트 실패: %w", err)
	}

	return nil
}

func (c *Client) Close() error {
	return c.db.Close()
}
package health

import (
	"encoding/json"
	"fmt"
	"multinic-agent-v2/internal/domain/interfaces"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthService는 헬스체크 기능을 제공하는 서비스입니다
type HealthService struct {
	mu             sync.RWMutex
	clock          interfaces.Clock
	logger         *logrus.Logger
	startTime      time.Time
	dbHealthy      bool
	dbError        error
	processedVMs   int64
	failedConfigs  int64
	networkManager string
}

// HealthStatus는 헬스체크 상태를 나타냅니다
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
)

// HealthResponse는 헬스체크 응답 구조체입니다
type HealthResponse struct {
	Status     HealthStatus           `json:"status"`
	Timestamp  string                 `json:"timestamp"`
	LastCheck  string                 `json:"last_check"`
	Components map[string]interface{} `json:"components"`
	Statistics map[string]interface{} `json:"statistics"`
}

// NewHealthService는 새로운 HealthService를 생성합니다
func NewHealthService(clock interfaces.Clock, logger *logrus.Logger) *HealthService {
	return &HealthService{
		clock:     clock,
		logger:    logger,
		startTime: clock.Now(),
		dbHealthy: false,
	}
}

// UpdateDBHealth는 데이터베이스 상태를 업데이트합니다
func (h *HealthService) UpdateDBHealth(healthy bool, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.dbHealthy = healthy
	h.dbError = err
}

// IncrementProcessedVMs는 처리된 VM 수를 증가시킵니다
func (h *HealthService) IncrementProcessedVMs() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.processedVMs++
}

// IncrementFailedConfigs는 실패한 설정 수를 증가시킵니다
func (h *HealthService) IncrementFailedConfigs() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failedConfigs++
}

// SetNetworkManager는 사용 중인 네트워크 관리자 타입을 설정합니다
func (h *HealthService) SetNetworkManager(managerType string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.networkManager = managerType
}

// ServeHTTP는 HTTP 헬스체크 엔드포인트를 처리합니다
func (h *HealthService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := h.buildHealthResponse()

	// 상태에 따라 HTTP 상태 코드 설정
	statusCode := http.StatusOK
	if response.Status == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("헬스체크 응답 인코딩 실패")
	}
}

// buildHealthResponse는 헬스체크 응답을 구성합니다
func (h *HealthService) buildHealthResponse() HealthResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := h.clock.Now()

	// 전체 상태 결정
	status := h.determineOverallStatus()

	// 컴포넌트 상태
	components := map[string]interface{}{
		"database": map[string]interface{}{
			"healthy": h.dbHealthy,
			"error":   h.formatError(h.dbError),
		},
		"network_manager": map[string]interface{}{
			"type": h.networkManager,
		},
	}

	// 통계 정보
	statistics := map[string]interface{}{
		"processed_vms":  h.processedVMs,
		"failed_configs": h.failedConfigs,
		"uptime":         h.formatUptime(now.Sub(h.startTime)),
	}

	return HealthResponse{
		Status:     status,
		Timestamp:  now.Format(time.RFC3339),
		LastCheck:  now.Format(time.RFC3339),
		Components: components,
		Statistics: statistics,
	}
}

// determineOverallStatus는 전체 상태를 결정합니다
func (h *HealthService) determineOverallStatus() HealthStatus {
	// 데이터베이스가 비정상이면 unhealthy
	if !h.dbHealthy {
		return StatusUnhealthy
	}

	// 실패한 설정이 전체의 50% 이상이면 degraded
	if h.processedVMs > 0 && h.failedConfigs > 0 {
		failureRate := float64(h.failedConfigs) / float64(h.processedVMs+h.failedConfigs)
		if failureRate >= 0.5 {
			return StatusDegraded
		}
	}

	return StatusHealthy
}

// formatError는 에러를 문자열로 포맷팅합니다
func (h *HealthService) formatError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// formatUptime은 가동 시간을 사람이 읽기 쉬운 형태로 포맷팅합니다
func (h *HealthService) formatUptime(duration time.Duration) string {
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

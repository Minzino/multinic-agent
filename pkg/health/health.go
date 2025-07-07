package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

type HealthChecker struct {
	mu            sync.RWMutex
	status        Status
	lastCheck     time.Time
	dbHealthy     bool
	lastDBError   error
	processedVMs  int
	failedConfigs int
	logger        *logrus.Logger
}

type HealthStatus struct {
	Status        Status    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	LastCheck     time.Time `json:"last_check"`
	Components    Components `json:"components"`
	Statistics    Stats     `json:"statistics"`
}

type Components struct {
	Database struct {
		Healthy bool   `json:"healthy"`
		Error   string `json:"error,omitempty"`
	} `json:"database"`
	NetworkManager struct {
		Type string `json:"type"`
	} `json:"network_manager"`
}

type Stats struct {
	ProcessedVMs  int `json:"processed_vms"`
	FailedConfigs int `json:"failed_configs"`
	Uptime        string `json:"uptime"`
}

var startTime = time.Now()

func NewHealthChecker(logger *logrus.Logger) *HealthChecker {
	return &HealthChecker{
		status:    StatusHealthy,
		lastCheck: time.Now(),
		dbHealthy: true,
		logger:    logger,
	}
}

func (h *HealthChecker) UpdateDBHealth(healthy bool, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.dbHealthy = healthy
	h.lastDBError = err
	h.lastCheck = time.Now()
	
	h.updateOverallStatus()
}

func (h *HealthChecker) IncrementProcessedVMs() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.processedVMs++
}

func (h *HealthChecker) IncrementFailedConfigs() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failedConfigs++
	h.updateOverallStatus()
}

func (h *HealthChecker) updateOverallStatus() {
	if !h.dbHealthy {
		h.status = StatusUnhealthy
	} else if h.failedConfigs > 0 {
		h.status = StatusDegraded
	} else {
		h.status = StatusHealthy
	}
}

func (h *HealthChecker) GetStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	status := HealthStatus{
		Status:    h.status,
		Timestamp: time.Now(),
		LastCheck: h.lastCheck,
	}
	
	status.Components.Database.Healthy = h.dbHealthy
	if h.lastDBError != nil {
		status.Components.Database.Error = h.lastDBError.Error()
	}
	
	status.Statistics.ProcessedVMs = h.processedVMs
	status.Statistics.FailedConfigs = h.failedConfigs
	status.Statistics.Uptime = time.Since(startTime).String()
	
	return status
}

func (h *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	status := h.GetStatus()
	
	// HTTP 상태 코드 설정
	switch status.Status {
	case StatusHealthy:
		w.WriteHeader(http.StatusOK)
	case StatusDegraded:
		w.WriteHeader(http.StatusOK) // 서비스는 동작하지만 일부 문제 있음
	case StatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.WithError(err).Error("헬스 상태 인코딩 실패")
	}
}
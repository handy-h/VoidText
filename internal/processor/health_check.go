package processor

import (
	"fmt"
	"sync"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/external"
	"voidtext/internal/logging"
)

// HealthStatus 健康状态
type HealthStatus struct {
	Service      string    `json:"service"`
	Healthy      bool      `json:"healthy"`
	LastCheck    time.Time `json:"last_check"`
	LastError    string    `json:"last_error,omitempty"`
	ErrorCount   int       `json:"error_count"`
	SuccessCount int       `json:"success_count"`
	TotalChecks  int       `json:"total_checks"`
	AvgResponseMs int64    `json:"avg_response_ms"`
	Enabled      bool      `json:"enabled"`
}

// HealthCheckManager 健康检查管理器
type HealthCheckManager struct {
	statuses      map[string]*HealthStatus
	ollamaClient  *external.OllamaClient
	checkInterval time.Duration // 统一命名：checkInterval
	mu            sync.RWMutex
	stopChan      chan bool
	running       bool
}

var (
	globalHealthManager *HealthCheckManager
	healthManagerOnce   sync.Once
)

// GetHealthManager 获取全局健康管理器（线程安全单例）
func GetHealthManager() *HealthCheckManager {
	healthManagerOnce.Do(func() {
		globalHealthManager = &HealthCheckManager{
			statuses:      make(map[string]*HealthStatus),
			checkInterval: getDefaultCheckInterval(),
			stopChan:      make(chan bool),
		}
		
		// 初始化Ollama客户端（如果启用本地模型）
		if config.AppConfigInstance.EnableLocalModel {
			globalHealthManager.ollamaClient = external.NewOllamaClient(
				config.AppConfigInstance.LocalModelURL,
				config.AppConfigInstance.LocalModelName,
				time.Duration(config.AppConfigInstance.LocalModelTimeout)*time.Second,
			)
			
			// 注册本地模型健康检查
			globalHealthManager.RegisterService("ollama", true)
		}
		
		// 注册远程API健康检查
		globalHealthManager.RegisterService("remote_api", true)
	})
	return globalHealthManager
}

// RegisterService 注册服务进行健康检查
func (hcm *HealthCheckManager) RegisterService(service string, enabled bool) {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()
	
	hcm.statuses[service] = &HealthStatus{
		Service:   service,
		Healthy:   false, // 初始状态为不健康，等待第一次检查
		LastCheck: time.Now(),
		Enabled:   enabled,
	}
}

// Start 启动健康检查
func (hcm *HealthCheckManager) Start() error {
	if hcm.running {
		return fmt.Errorf("健康检查已在运行")
	}

	hcm.running = true

	// 同步执行第一次健康检查，确保初始状态正确
	// 避免goroutine调度延迟导致processChunk在健康检查完成前运行
	hcm.performHealthChecks()

	// 启动后台定期检查
	go hcm.runHealthChecksLoop()

	logging.Info("health_check_started", map[string]interface{}{
		"interval_seconds": hcm.checkInterval.Seconds(),
		"services":         len(hcm.statuses),
	})

	return nil
}

// Stop 停止健康检查
func (hcm *HealthCheckManager) Stop() {
	if !hcm.running {
		return
	}
	
	hcm.stopChan <- true
	hcm.running = false
	
	logging.Info("health_check_stopped", nil)
}

// runHealthChecksLoop 定期执行健康检查的循环
func (hcm *HealthCheckManager) runHealthChecksLoop() {
	ticker := time.NewTicker(hcm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hcm.performHealthChecks()
		case <-hcm.stopChan:
			return
		}
	}
}

// performHealthChecks 执行所有健康检查
func (hcm *HealthCheckManager) performHealthChecks() {
	hcm.mu.RLock()
	services := make([]string, 0, len(hcm.statuses))
	for service := range hcm.statuses {
		services = append(services, service)
	}
	hcm.mu.RUnlock()
	
	for _, service := range services {
		hcm.checkService(service)
	}
}

// checkService 检查单个服务
func (hcm *HealthCheckManager) checkService(service string) {
	hcm.mu.Lock()
	status, exists := hcm.statuses[service]
	if !exists || !status.Enabled {
		hcm.mu.Unlock()
		return
	}
	hcm.mu.Unlock()
	
	startTime := time.Now()
	var healthy bool
	var errMsg string
	
	switch service {
	case "ollama":
		healthy, errMsg = hcm.checkOllama()
	case "remote_api":
		healthy, errMsg = hcm.checkRemoteAPI()
	default:
		healthy = false
		errMsg = fmt.Sprintf("未知服务类型: %s", service)
	}
	
	duration := time.Since(startTime).Milliseconds()
	
	hcm.updateServiceStatus(service, healthy, duration, errMsg)
	
	// 记录健康检查结果
	logging.HealthCheckResult(service, healthy, duration, errMsg)
}

// checkOllama 检查Ollama服务
func (hcm *HealthCheckManager) checkOllama() (bool, string) {
	if hcm.ollamaClient == nil {
		return false, "Ollama客户端未初始化"
	}
	
	healthy := hcm.ollamaClient.HealthCheck()
	if !healthy {
		return false, "Ollama服务不可用"
	}
	
	return true, ""
}

// checkRemoteAPI 检查远程API
func (hcm *HealthCheckManager) checkRemoteAPI() (bool, string) {
	// 简化检查：如果配置了API URL和Key，则认为可用
	cfg := config.AppConfigInstance
	if cfg.LLMApiURL == "" || cfg.LLMApiKey == "" {
		return false, "远程API未配置"
	}
	
	// 可以添加更复杂的检查，如发送测试请求
	return true, ""
}

// updateServiceStatus 更新服务状态
func (hcm *HealthCheckManager) updateServiceStatus(service string, healthy bool, durationMs int64, errorMsg string) {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()
	
	status, exists := hcm.statuses[service]
	if !exists {
		return
	}
	
	status.TotalChecks++
	status.LastCheck = time.Now()
	
	if healthy {
		status.SuccessCount++
		status.ErrorCount = 0 // 重置错误计数
		status.LastError = ""
		
		// 更新平均响应时间
		if status.AvgResponseMs == 0 {
			status.AvgResponseMs = durationMs
		} else {
			// 简单移动平均
			status.AvgResponseMs = (status.AvgResponseMs*9 + durationMs) / 10
		}
	} else {
		status.ErrorCount++
		status.LastError = errorMsg
	}
	
	// 更新健康状态
	oldHealthy := status.Healthy
	status.Healthy = healthy
	
	// 如果状态发生变化，记录日志
	if oldHealthy != healthy {
		event := "service_healthy"
		if !healthy {
			event = "service_unhealthy"
		}
		
		logging.Warn(event, map[string]interface{}{
			"service":   service,
			"healthy":   healthy,
			"error":     errorMsg,
			"duration":  durationMs,
			"error_count": status.ErrorCount,
		})
		
		// 如果本地模型连续失败达到阈值，自动禁用
		if service == "ollama" && status.ErrorCount >= getLocalModelErrorThreshold() {
			status.Enabled = false
			logging.Warn("local_model_auto_disabled", map[string]interface{}{
				"service":     service,
				"error_count": status.ErrorCount,
				"threshold":   getLocalModelErrorThreshold(),
			})
		}
	}
}

// GetStatus 获取服务状态
func (hcm *HealthCheckManager) GetStatus(service string) (*HealthStatus, bool) {
	hcm.mu.RLock()
	defer hcm.mu.RUnlock()
	
	status, exists := hcm.statuses[service]
	if !exists {
		return nil, false
	}
	
	// 返回副本以避免并发修改
	copyStatus := *status
	return &copyStatus, true
}

// GetAllStatuses 获取所有服务状态
func (hcm *HealthCheckManager) GetAllStatuses() map[string]HealthStatus {
	hcm.mu.RLock()
	defer hcm.mu.RUnlock()
	
	result := make(map[string]HealthStatus)
	for service, status := range hcm.statuses {
		result[service] = *status
	}
	
	return result
}

// IsServiceHealthy 检查服务是否健康
func (hcm *HealthCheckManager) IsServiceHealthy(service string) bool {
	hcm.mu.RLock()
	defer hcm.mu.RUnlock()
	
	status, exists := hcm.statuses[service]
	if !exists {
		return false
	}
	
	return status.Healthy && status.Enabled
}

// EnableService 启用服务
func (hcm *HealthCheckManager) EnableService(service string, forceCheck bool) bool {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()
	
	status, exists := hcm.statuses[service]
	if !exists {
		return false
	}
	
	status.Enabled = true
	
	logging.Info("service_enabled", map[string]interface{}{
		"service":     service,
		"force_check": forceCheck,
	})
	
	// 如果需要强制检查，立即执行一次健康检查
	if forceCheck {
		go func() {
			hcm.checkService(service)
		}()
	}
	
	return true
}

// DisableService 禁用服务
func (hcm *HealthCheckManager) DisableService(service string) bool {
	hcm.mu.Lock()
	defer hcm.mu.Unlock()
	
	status, exists := hcm.statuses[service]
	if !exists {
		return false
	}
	
	status.Enabled = false
	
	logging.Info("service_disabled", map[string]interface{}{
		"service": service,
	})
	
	return true
}

// GetFallbackRecommendation 获取降级建议
func (hcm *HealthCheckManager) GetFallbackRecommendation() string {
	hcm.mu.RLock()
	defer hcm.mu.RUnlock()
	
	// 检查本地模型状态
	ollamaStatus, ollamaExists := hcm.statuses["ollama"]
	remoteStatus, remoteExists := hcm.statuses["remote_api"]
	
	if !ollamaExists || !remoteExists {
		return "unknown"
	}
	
	// 推荐策略
	if ollamaStatus.Healthy && ollamaStatus.Enabled {
		return "local_first" // 本地优先
	}
	
	if remoteStatus.Healthy {
		return "remote_only" // 仅远程
	}
	
	return "degraded" // 降级模式
}

// ShouldUseLocalModel 是否应该使用本地模型
func (hcm *HealthCheckManager) ShouldUseLocalModel() bool {
	return hcm.IsServiceHealthy("ollama")
}

// ShouldFallbackToRemote 是否应该降级到远程API
func (hcm *HealthCheckManager) ShouldFallbackToRemote() bool {
	hcm.mu.RLock()
	defer hcm.mu.RUnlock()

	localStatus, localExists := hcm.statuses["ollama"]
	remoteStatus, remoteExists := hcm.statuses["remote_api"]

	// 如果本地模型不健康但远程API健康，建议降级
	if localExists && remoteExists {
		return !localStatus.Healthy && remoteStatus.Healthy
	}

	return false
}



// GetServiceMetrics 获取服务指标
func (hcm *HealthCheckManager) GetServiceMetrics(service string) map[string]interface{} {
	status, exists := hcm.GetStatus(service)
	if !exists {
		return nil
	}
	
	metrics := map[string]interface{}{
		"service":        status.Service,
		"healthy":        status.Healthy,
		"enabled":        status.Enabled,
		"last_check":     status.LastCheck.Format(time.RFC3339),
		"total_checks":   status.TotalChecks,
		"success_count":  status.SuccessCount,
		"error_count":    status.ErrorCount,
		"avg_response_ms": status.AvgResponseMs,
		"success_rate":   0.0,
	}
	
	if status.TotalChecks > 0 {
		metrics["success_rate"] = float64(status.SuccessCount) / float64(status.TotalChecks) * 100
	}
	
	if status.LastError != "" {
		metrics["last_error"] = status.LastError
	}
	
	return metrics
}

// getDefaultCheckInterval 获取默认检查间隔
func getDefaultCheckInterval() time.Duration {
	return 30 * time.Second
}

// getLocalModelErrorThreshold 获取本地模型错误阈值
func getLocalModelErrorThreshold() int {
	return 3
}
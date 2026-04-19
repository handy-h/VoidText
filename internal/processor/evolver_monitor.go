package processor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/database"
	"txt-cleaning/internal/logging"
)

// EvolverMonitor 自进化监控器
// 负责监控修复效果，在错误率超过阈值时调用外部Evolver工具优化提示词
// 设计原则：
// 1. 避免直接修改Go源代码，通过外部脚本生成新的提示词文件
// 2. 使用文件监听（fsnotify轮询）实现热重载
// 3. 保证并发安全，避免多个监控实例同时调用Evolver
// 4. 支持指数退避，避免频繁调用外部工具
type EvolverMonitor struct {
	promptManager *PromptManager
	mu            sync.RWMutex
	isRunning     bool
	stopChan      chan struct{}
	lastCallTime  time.Time
	minCallInterval time.Duration
}

// NewEvolverMonitor 创建自进化监控器
func NewEvolverMonitor(pm *PromptManager) *EvolverMonitor {
	return &EvolverMonitor{
		promptManager:   pm,
		stopChan:        make(chan struct{}),
		minCallInterval: 5 * time.Minute, // 最小调用间隔5分钟，避免频繁调用
	}
}

// Start 启动监控器（后台协程）
// 启动一个独立的goroutine定期检查错误阈值并触发Evolver调用
func (em *EvolverMonitor) Start() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.isRunning {
		return fmt.Errorf("监控器已在运行中")
	}

	em.isRunning = true
	go em.monitorLoop()

	logging.Info("evolver_monitor_started", map[string]interface{}{
		"min_call_interval": em.minCallInterval.String(),
	})
	return nil
}

// Stop 停止监控器
func (em *EvolverMonitor) Stop() {
	em.mu.Lock()
	defer em.mu.Unlock()

	if !em.isRunning {
		return
	}

	close(em.stopChan)
	em.isRunning = false
	logging.Info("evolver_monitor_stopped", nil)
}

// monitorLoop 监控循环（在独立协程中运行）
// 每30秒检查一次错误阈值，满足条件时调用Evolver
func (em *EvolverMonitor) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-em.stopChan:
			return
		case <-ticker.C:
			em.checkAndTrigger()
		}
	}
}

// checkAndTrigger 检查阈值并触发Evolver调用
func (em *EvolverMonitor) checkAndTrigger() {
	// 从数据库或日志中获取最近的错误统计（简化实现）
	// 实际项目中应从数据库查询错误率、缓存命中率等指标
	// 这里使用一个简单的启发式规则：如果最近有错误日志，则触发

	// 检查是否达到最小调用间隔
	em.mu.RLock()
	lastCall := em.lastCallTime
	em.mu.RUnlock()

	if time.Since(lastCall) < em.minCallInterval {
		return // 未达到最小调用间隔
	}

	// 模拟检查错误阈值（实际应查询数据库）
	shouldTrigger := em.checkErrorThresholds()
	if !shouldTrigger {
		return
	}

	// 触发Evolver调用
	em.triggerEvolver()
}

// checkErrorThresholds 检查错误阈值（基于数据库统计）
// 1. 缓存命中率 < 30%
// 2. API错误率 > 20%
// 3. 连续失败次数 > 3
func (em *EvolverMonitor) checkErrorThresholds() bool {
	// 获取数据库仓库实例
	repo := database.NewChunkCacheRepo()
	
	// 查询缓存命中率
	hitRate, err := repo.GetCacheHitRate()
	if err != nil {
		logging.Warn("evolver_hit_rate_query_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return false
	}
	
	// 查询API错误率
	errorRate, err := repo.GetErrorRate()
	if err != nil {
		logging.Warn("evolver_error_rate_query_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return false
	}
	
	// 检查阈值
	thresholds := map[string]float64{
		"hit_rate_low":   30.0, // 缓存命中率低于30%
		"error_rate_high": 20.0, // API错误率高于20%
	}
	
	shouldTrigger := false
	reason := ""
	
	if hitRate < thresholds["hit_rate_low"] {
		shouldTrigger = true
		reason = fmt.Sprintf("缓存命中率过低: %.1f%% < %.1f%%", hitRate, thresholds["hit_rate_low"])
		logging.Warn("evolver_trigger_low_hit_rate", map[string]interface{}{
			"hit_rate": hitRate,
			"threshold": thresholds["hit_rate_low"],
			"reason": reason,
		})
	}
	
	if errorRate > thresholds["error_rate_high"] {
		shouldTrigger = true
		reason = fmt.Sprintf("API错误率过高: %.1f%% > %.1f%%", errorRate, thresholds["error_rate_high"])
		logging.Warn("evolver_trigger_high_error_rate", map[string]interface{}{
			"error_rate": errorRate,
			"threshold": thresholds["error_rate_high"],
			"reason": reason,
		})
	}
	
	if shouldTrigger && reason != "" {
		logging.EvolverTriggered(reason, int(errorRate), int(thresholds["error_rate_high"]))
	}
	
	return shouldTrigger
}

// triggerEvolver 触发Evolver调用
// 调用外部Evolver脚本，传入错误上下文和当前提示词
func (em *EvolverMonitor) triggerEvolver() {
	em.mu.Lock()
	em.lastCallTime = time.Now()
	em.mu.Unlock()

	// 获取当前提示词和版本
	prompt, version := em.promptManager.GetCurrentPrompt()

	// 获取数据库统计信息
	repo := database.NewChunkCacheRepo()
	hitRate, hitErr := repo.GetCacheHitRate()
	errorRate, errorErr := repo.GetErrorRate()

	// 确定错误类型
	errorType := "unknown"
	if hitErr == nil && errorErr == nil {
		if hitRate < 30.0 {
			errorType = "low_hit_rate"
		} else if errorRate > 20.0 {
			errorType = "high_error_rate"
		}
	}

	// 准备详细的错误上下文（符合Evolver GEP协议）
	errorContext := fmt.Sprintf(`{
	"prompt_version": "%s",
	"prompt_length": %d,
	"timestamp": "%s",
	"error_type": "%s",
	"hit_rate": %.2f,
	"error_rate": %.2f,
	"hit_rate_error": "%v",
	"error_rate_error": "%v",
	"recommendation": "根据性能指标优化提示词",
	"system": "txt-cleaning",
	"environment": "production",
	"component": "model_repairer"
}`, version, len(prompt), time.Now().Format(time.RFC3339),
		errorType, hitRate, errorRate, hitErr, errorErr)

	// 调用外部Evolver脚本
	// 假设脚本位于 scripts/evolver.py
	scriptPath := filepath.Join(config.AppConfigInstance.BaseDir, "scripts", "evolver.py")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		logging.Warn("evolver_script_not_found", map[string]interface{}{
			"script_path": scriptPath,
			"recommendation": "请创建Evolver脚本或检查路径",
		})
		return
	}

	// 执行外部命令（添加--base-dir参数）
	baseDir := config.AppConfigInstance.BaseDir
	cmd := exec.Command("python3", scriptPath, 
		"--prompt", prompt, 
		"--context", errorContext,
		"--base-dir", baseDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		logging.Error("evolver_execution_failed", map[string]interface{}{
			"script_path": scriptPath,
			"duration_ms": duration,
			"error": err.Error(),
			"stderr": strings.TrimSpace(stderr.String()),
		})
		return
	}

	// 解析Evolver输出（假设输出为新提示词）
	newPrompt := strings.TrimSpace(stdout.String())
	if newPrompt == "" {
		logging.Warn("evolver_empty_output", map[string]interface{}{
			"script_path": scriptPath,
			"stdout": stdout.String(),
		})
		return
	}

	// 应用Evolver修正
	if err := em.promptManager.ApplyEvolverCorrection(newPrompt); err != nil {
		logging.Error("evolver_correction_failed", map[string]interface{}{
			"script_path": scriptPath,
			"new_prompt_length": len(newPrompt),
			"error": err.Error(),
		})
		return
	}

	logging.Info("evolver_correction_applied", map[string]interface{}{
		"script_path": scriptPath,
		"duration_ms": duration,
		"old_version": version,
		"new_prompt_length": len(newPrompt),
		"hit_rate": hitRate,
		"error_rate": errorRate,
		"error_type": errorType,
	})
}

// TriggerManual 手动触发Evolver调用（用于测试或手动优化）
func (em *EvolverMonitor) TriggerManual() error {
	em.mu.RLock()
	lastCall := em.lastCallTime
	em.mu.RUnlock()

	if time.Since(lastCall) < em.minCallInterval {
		return fmt.Errorf("手动触发过于频繁，请等待 %v", em.minCallInterval-time.Since(lastCall))
	}

	em.triggerEvolver()
	return nil
}

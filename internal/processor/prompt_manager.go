package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"txt-cleaning/internal/database"
	"txt-cleaning/internal/logging"
)

// PromptManager 提示词管理器
// 负责动态加载、热更新和管理多版本提示词，支持Evolver自进化
type PromptManager struct {
	promptName      string
	promptDir       string
	defaultPrompt   string
	currentPrompt   string
	currentVersion  string
	lastModified    time.Time
	mu              sync.RWMutex
	watchers        []chan<- string
	dbRepo          *database.ChunkCacheRepo
	hotReload       bool
}

// PromptConfig 提示词配置
type PromptConfig struct {
	Name          string
	PromptDir     string
	DefaultPrompt string
	HotReload     bool
}

// NewPromptManager 创建提示词管理器
func NewPromptManager(config PromptConfig) (*PromptManager, error) {
	pm := &PromptManager{
		promptName:    config.Name,
		promptDir:     config.PromptDir,
		defaultPrompt: config.DefaultPrompt,
		currentPrompt: config.DefaultPrompt,
		currentVersion: "v1.0.0",
		hotReload:     config.HotReload,
		dbRepo:        database.NewChunkCacheRepo(),
	}

	// 尝试从数据库加载最新版本
	if latest, err := pm.dbRepo.GetLatestPromptVersion(config.Name); err == nil && latest != nil {
		pm.currentPrompt = latest.PromptContent
		pm.currentVersion = latest.PromptVersion
		logging.PromptLoaded(pm.currentVersion, "database", len(pm.currentPrompt))
	} else {
		// 尝试从文件加载
		if err := pm.loadFromFile(); err != nil {
			logging.Warn("prompt_load_failed", map[string]interface{}{
				"name":    config.Name,
				"source":  "file",
				"error":   err.Error(),
				"fallback": "default",
			})
			// 使用默认提示词
			pm.currentPrompt = config.DefaultPrompt
			pm.currentVersion = "v1.0.0-default"
		}
	}

	// 启动文件监视器（如果启用热重载）
	if pm.hotReload && pm.promptDir != "" {
		go pm.watchPromptFiles()
	}

	return pm, nil
}

// loadFromFile 从文件加载提示词
func (pm *PromptManager) loadFromFile() error {
	// 查找提示词文件
	pattern := filepath.Join(pm.promptDir, fmt.Sprintf("%s*.txt", pm.promptName))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("查找提示词文件失败: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("未找到提示词文件: %s", pattern)
	}

	// 使用最新修改的文件
	var latestFile string
	var latestMod time.Time
	for _, file := range matches {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestMod) {
			latestMod = info.ModTime()
			latestFile = file
		}
	}

	if latestFile == "" {
		return fmt.Errorf("无法读取提示词文件")
	}

	// 读取文件内容
	content, err := os.ReadFile(latestFile)
	if err != nil {
		return fmt.Errorf("读取提示词文件失败: %w", err)
	}

	prompt := strings.TrimSpace(string(content))
	if prompt == "" {
		return fmt.Errorf("提示词文件为空")
	}

	// 从文件名提取版本号
	version := extractVersionFromFilename(latestFile)
	if version == "" {
		version = fmt.Sprintf("v1.0.0-file-%d", latestMod.Unix())
	}

	pm.mu.Lock()
	pm.currentPrompt = prompt
	pm.currentVersion = version
	pm.lastModified = latestMod
	pm.mu.Unlock()

	logging.PromptLoaded(version, "file", len(prompt))

	// 保存到数据库
	record := &database.PromptVersionRecord{
		PromptName:    pm.promptName,
		PromptVersion: version,
		PromptContent: prompt,
		Source:        "file",
		SuccessRate:   0.0,
		TotalUses:     0,
		SuccessfulUses: 0,
	}
	if err := pm.dbRepo.SavePromptVersion(record); err != nil {
		logging.Error("prompt_save_failed", map[string]interface{}{
			"name":  pm.promptName,
			"version": version,
			"error": err.Error(),
		})
	}

	return nil
}

// extractVersionFromFilename 从文件名提取版本号
func extractVersionFromFilename(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// 尝试匹配版本模式 v1.0.0, v2, v1.2.3 等
	parts := strings.Split(nameWithoutExt, "_")
	for _, part := range parts {
		if strings.HasPrefix(part, "v") {
			// 移除'v'前缀并检查是否为有效版本号
			version := part[1:]
			if strings.ContainsAny(version, "0123456789.") {
				return part
			}
		}
	}

	// 如果没有明确版本，使用时间戳
	return ""
}

// GetCurrentPrompt 获取当前提示词（线程安全）
func (pm *PromptManager) GetCurrentPrompt() (prompt, version string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.currentPrompt, pm.currentVersion
}

// UpdatePrompt 更新提示词（用于Evolver自进化）
func (pm *PromptManager) UpdatePrompt(newPrompt, newVersion string, source string) error {
	if newPrompt == "" {
		return fmt.Errorf("新提示词不能为空")
	}

	if newVersion == "" {
		newVersion = fmt.Sprintf("v1.0.0-%s-%d", source, time.Now().Unix())
	}

	pm.mu.Lock()
	oldVersion := pm.currentVersion
	pm.currentPrompt = newPrompt
	pm.currentVersion = newVersion
	pm.lastModified = time.Now()
	pm.mu.Unlock()

	// 记录更新
	logging.PromptUpdated(oldVersion, newVersion, len(newPrompt)-len(pm.defaultPrompt))

	// 保存到数据库
	record := &database.PromptVersionRecord{
		PromptName:    pm.promptName,
		PromptVersion: newVersion,
		PromptContent: newPrompt,
		Source:        source,
		SuccessRate:   0.0,
		TotalUses:     0,
		SuccessfulUses: 0,
	}
	if err := pm.dbRepo.SavePromptVersion(record); err != nil {
		logging.Error("prompt_update_failed", map[string]interface{}{
			"name":    pm.promptName,
			"version": newVersion,
			"error":   err.Error(),
		})
		return err
	}

	// 通知所有观察者
	pm.notifyWatchers(newPrompt)

	return nil
}

// ApplyEvolverCorrection 应用Evolver修正（实现接口）
func (pm *PromptManager) ApplyEvolverCorrection(newPrompt string) error {
	version := fmt.Sprintf("v1.0.0-evolver-%d", time.Now().Unix())
	return pm.UpdatePrompt(newPrompt, version, "evolver")
}

// RegisterWatcher 注册提示词更新观察者
func (pm *PromptManager) RegisterWatcher(ch chan<- string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.watchers = append(pm.watchers, ch)
}

// notifyWatchers 通知所有观察者
func (pm *PromptManager) notifyWatchers(newPrompt string) {
	pm.mu.RLock()
	watchers := make([]chan<- string, len(pm.watchers))
	copy(watchers, pm.watchers)
	pm.mu.RUnlock()

	for _, ch := range watchers {
		select {
		case ch <- newPrompt:
			// 发送成功
		default:
			// 通道满，跳过
			logging.Warn("prompt_watcher_skipped", map[string]interface{}{
				"name": pm.promptName,
			})
		}
	}
}

// watchPromptFiles 监视提示词文件变化（热重载）
func (pm *PromptManager) watchPromptFiles() {
	if pm.promptDir == "" {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pattern := filepath.Join(pm.promptDir, fmt.Sprintf("%s*.txt", pm.promptName))
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range matches {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			pm.mu.RLock()
			needsReload := info.ModTime().After(pm.lastModified)
			pm.mu.RUnlock()

			if needsReload {
				logging.Info("prompt_file_modified", map[string]interface{}{
					"name": pm.promptName,
					"file": file,
					"time": info.ModTime(),
				})

				if err := pm.loadFromFile(); err != nil {
					logging.Error("prompt_reload_failed", map[string]interface{}{
						"name":  pm.promptName,
						"file":  file,
						"error": err.Error(),
					})
				}
				break
			}
		}
	}
}

// RecordUsage 记录提示词使用情况（用于评估效果）
func (pm *PromptManager) RecordUsage(success bool) {
	pm.mu.RLock()
	version := pm.currentVersion
	pm.mu.RUnlock()

	if err := pm.dbRepo.UpdatePromptUsage(pm.promptName, version, success); err != nil {
		logging.Error("prompt_usage_record_failed", map[string]interface{}{
			"name":    pm.promptName,
			"version": version,
			"success": success,
			"error":   err.Error(),
		})
	}
}

// GetPromptStats 获取提示词统计信息
func (pm *PromptManager) GetPromptStats() (map[string]interface{}, error) {
	pm.mu.RLock()
	currentPrompt := pm.currentPrompt
	currentVersion := pm.currentVersion
	pm.mu.RUnlock()

	// 从数据库获取统计信息
	latest, err := pm.dbRepo.GetLatestPromptVersion(pm.promptName)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"name":           pm.promptName,
		"current_version": currentVersion,
		"prompt_length":  len(currentPrompt),
		"default_length": len(pm.defaultPrompt),
	}

	if latest != nil {
		stats["total_uses"] = latest.TotalUses
		stats["successful_uses"] = latest.SuccessfulUses
		stats["success_rate"] = latest.SuccessRate
		stats["source"] = latest.Source
		stats["last_updated"] = latest.UpdatedAt
	}

	return stats, nil
}

// ResetToDefault 重置为默认提示词
func (pm *PromptManager) ResetToDefault() error {
	return pm.UpdatePrompt(pm.defaultPrompt, "v1.0.0-default", "reset")
}

// Close 关闭提示词管理器
func (pm *PromptManager) Close() {
	// 目前没有需要清理的资源
}
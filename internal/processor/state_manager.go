package processor

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/logging"
)

// ProcessingStats 处理统计
type ProcessingStats struct {
	CacheHits      int `json:"cache_hits"`
	LocalSuccess   int `json:"local_success"`
	LocalFailure   int `json:"local_failure"`
	RemoteFallback int `json:"remote_fallback"`
}

// ProcessingTiming 处理时间信息
type ProcessingTiming struct {
	StartTime      time.Time `json:"start_time"`
	LastUpdateTime time.Time `json:"last_update_time"`
	TotalDuration  int64     `json:"total_duration,omitempty"`
}

// ProcessingState 处理状态（完整状态保存）
type ProcessingState struct {
	FileMd5         string            `json:"file_md5"`
	CurrentStep     string            `json:"current_step"`
	Status          string            `json:"status"`
	Progress        int               `json:"progress"`
	TotalChunks     int               `json:"total_chunks"`
	ProcessedChunks int               `json:"processed_chunks"`
	ChunkStates     map[int]ChunkState `json:"chunk_states"`
	Stats           ProcessingStats   `json:"stats"`
	Timing          ProcessingTiming  `json:"timing"`
	Checkpoints     []Checkpoint      `json:"checkpoints"`
	ErrorMsg        string            `json:"error_msg,omitempty"`
}

// ChunkState 块状态
type ChunkState struct {
	ChunkID       int       `json:"chunk_id"`
	Status        string    `json:"status"` // pending/processing/completed/failed
	Source        string    `json:"source,omitempty"` // local/remote/cache
	Confidence    float64   `json:"confidence,omitempty"`
	ProcessingMs  int64     `json:"processing_ms,omitempty"`
	ErrorMsg      string    `json:"error_msg,omitempty"`
	CompletedTime time.Time `json:"completed_time,omitempty"`
}

// Checkpoint 检查点
type Checkpoint struct {
	Timestamp   time.Time              `json:"timestamp"`
	Description string                 `json:"description"`
	State       map[string]interface{} `json:"state"`
}

// StateManager 状态管理器
type StateManager struct {
	states  map[string]*ProcessingState
	mu      sync.RWMutex
	dataDir string
}

var (
	globalStateManager *StateManager
	stateManagerOnce   sync.Once
)

// GetStateManager 获取全局状态管理器（线程安全单例）
func GetStateManager() *StateManager {
	stateManagerOnce.Do(func() {
		globalStateManager = &StateManager{
			states:  make(map[string]*ProcessingState),
			dataDir: config.AppConfigInstance.DataDir,
		}
	})
	return globalStateManager
}

// StartProcessing 开始处理，初始化状态
func (sm *StateManager) StartProcessing(fileMd5 string, totalChunks int, currentStep string) *ProcessingState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state := &ProcessingState{
		FileMd5:         fileMd5,
		CurrentStep:     currentStep,
		Status:          "processing",
		Progress:        0,
		TotalChunks:     totalChunks,
		ProcessedChunks: 0,
		ChunkStates:     make(map[int]ChunkState),
		Stats: ProcessingStats{
			CacheHits:      0,
			LocalSuccess:   0,
			LocalFailure:   0,
			RemoteFallback: 0,
		},
		Timing: ProcessingTiming{
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
		Checkpoints: []Checkpoint{},
	}

	// 初始化所有块状态为pending
	for i := 0; i < totalChunks; i++ {
		state.ChunkStates[i] = ChunkState{
			ChunkID: i,
			Status:  "pending",
		}
	}

	sm.states[fileMd5] = state
	
	// 保存初始状态到文件
	sm.saveStateToFile(state)
	
	// 创建初始检查点
	sm.createCheckpoint(state, "开始处理")

	logging.Info("processing_state_started", map[string]interface{}{
		"file_md5":     fileMd5,
		"total_chunks": totalChunks,
		"current_step": currentStep,
	})

	return state
}

// UpdateChunkState 更新块状态
func (sm *StateManager) UpdateChunkState(fileMd5 string, chunkID int, status, source string, confidence float64, processingMs int64, errorMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state, exists := sm.states[fileMd5]
	if !exists {
		logging.Warn("state_not_found", map[string]interface{}{
			"file_md5": fileMd5,
			"chunk_id": chunkID,
		})
		return
	}

	chunkState := ChunkState{
		ChunkID:      chunkID,
		Status:       status,
		Source:       source,
		Confidence:   confidence,
		ProcessingMs: processingMs,
		ErrorMsg:     errorMsg,
	}

		if status == "completed" {
			chunkState.CompletedTime = time.Now()
			state.ProcessedChunks++
			
			// 更新统计
			switch source {
			case "cache":
				state.Stats.CacheHits++
			case "local":
				if errorMsg == "" {
					state.Stats.LocalSuccess++
				} else {
					state.Stats.LocalFailure++
				}
			case "remote":
				state.Stats.RemoteFallback++
			}
		}

		state.ChunkStates[chunkID] = chunkState
		state.Timing.LastUpdateTime = time.Now()
	
	// 计算进度
	if state.TotalChunks > 0 {
		state.Progress = int(float64(state.ProcessedChunks) / float64(state.TotalChunks) * 100)
	}

	// 定期保存状态到文件（每10个块或每30秒）
	if state.ProcessedChunks%10 == 0 || time.Since(state.Timing.LastUpdateTime) > 30*time.Second {
		sm.saveStateToFile(state)
	}

	// 创建检查点（每50个块）
	if state.ProcessedChunks%50 == 0 {
		sm.createCheckpoint(state, fmt.Sprintf("处理第%d个块", state.ProcessedChunks))
	}
}

// GetProcessingState 获取处理状态
func (sm *StateManager) GetProcessingState(fileMd5 string) (*ProcessingState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	state, exists := sm.states[fileMd5]
	if exists {
		return state, true
	}

	// 尝试从文件加载
	loadedState, err := sm.loadStateFromFile(fileMd5)
	if err != nil {
		return nil, false
	}

	sm.states[fileMd5] = loadedState
	return loadedState, true
}

// FinishProcessing 完成处理
func (sm *StateManager) FinishProcessing(fileMd5 string, status string, errorMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state, exists := sm.states[fileMd5]
	if !exists {
		return
	}

	state.Status = status
	state.ErrorMsg = errorMsg
	state.Timing.LastUpdateTime = time.Now()
	state.Progress = 100

	// 创建最终检查点
	sm.createCheckpoint(state, "处理完成")
	
	// 保存最终状态
	sm.saveStateToFile(state)
	
	// 清理内存状态（保留文件备份）
	delete(sm.states, fileMd5)

	logging.Info("processing_state_finished", map[string]interface{}{
		"file_md5": fileMd5,
		"status":   status,
		"duration": time.Since(state.Timing.StartTime).Seconds(),
	})
}

// ResumeProcessing 恢复处理
func (sm *StateManager) ResumeProcessing(fileMd5 string) ([]int, error) {
	state, exists := sm.GetProcessingState(fileMd5)
	if !exists {
		return nil, fmt.Errorf("未找到处理状态: %s", fileMd5)
	}

	if state.Status != "processing" {
		return nil, fmt.Errorf("处理状态不是进行中: %s", state.Status)
	}

	// 找出未完成的块
	unfinishedChunks := []int{}
	for chunkID, chunkState := range state.ChunkStates {
		if chunkState.Status != "completed" {
			unfinishedChunks = append(unfinishedChunks, chunkID)
		}
	}

	// 创建恢复检查点
	sm.createCheckpoint(state, "恢复处理")

	logging.Info("processing_state_resumed", map[string]interface{}{
		"file_md5":           fileMd5,
		"total_chunks":       state.TotalChunks,
		"processed_chunks":   state.ProcessedChunks,
		"unfinished_chunks":  len(unfinishedChunks),
		"cache_hits":         state.Stats.CacheHits,
		"local_success":      state.Stats.LocalSuccess,
		"local_failure":      state.Stats.LocalFailure,
		"remote_fallback":    state.Stats.RemoteFallback,
	})

	return unfinishedChunks, nil
}

// saveStateToFile 保存状态到文件
func (sm *StateManager) saveStateToFile(state *ProcessingState) error {
	stateDir := filepath.Join(sm.dataDir, "states")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}

	stateFile := filepath.Join(stateDir, fmt.Sprintf("%s_state.json", state.FileMd5))
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}

	return nil
}

// loadStateFromFile 从文件加载状态
func (sm *StateManager) loadStateFromFile(fileMd5 string) (*ProcessingState, error) {
	stateFile := filepath.Join(sm.dataDir, "states", fmt.Sprintf("%s_state.json", fileMd5))
	
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("读取状态文件失败: %w", err)
	}

	var state ProcessingState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("反序列化状态失败: %w", err)
	}

	return &state, nil
}

// createCheckpoint 创建检查点
func (sm *StateManager) createCheckpoint(state *ProcessingState, description string) {
	checkpoint := Checkpoint{
		Timestamp:   time.Now(),
		Description: description,
		State: map[string]interface{}{
			"processed_chunks": state.ProcessedChunks,
			"progress":         state.Progress,
			"cache_hits":       state.Stats.CacheHits,
			"local_success":    state.Stats.LocalSuccess,
			"local_failure":    state.Stats.LocalFailure,
			"remote_fallback":  state.Stats.RemoteFallback,
			"elapsed_seconds":  time.Since(state.Timing.StartTime).Seconds(),
		},
	}

	state.Checkpoints = append(state.Checkpoints, checkpoint)
	
	// 限制检查点数量
	maxCheckpoints := getMaxCheckpoints()
	if len(state.Checkpoints) > maxCheckpoints {
		state.Checkpoints = state.Checkpoints[len(state.Checkpoints)-maxCheckpoints:]
	}

	// 保存检查点到单独文件
	checkpointDir := filepath.Join(sm.dataDir, "checkpoints", state.FileMd5)
	if err := os.MkdirAll(checkpointDir, 0755); err == nil {
		checkpointFile := filepath.Join(checkpointDir, fmt.Sprintf("checkpoint_%d.json", time.Now().Unix()))
		if data, err := json.MarshalIndent(checkpoint, "", "  "); err == nil {
			os.WriteFile(checkpointFile, data, 0644)
		}
	}
}

// GetStatistics 获取处理统计
func (sm *StateManager) GetStatistics(fileMd5 string) map[string]interface{} {
	state, exists := sm.GetProcessingState(fileMd5)
	if !exists {
	return nil
}

	stats := map[string]interface{}{
		"file_md5":         state.FileMd5,
		"current_step":     state.CurrentStep,
		"status":           state.Status,
		"progress":         state.Progress,
		"total_chunks":     state.TotalChunks,
		"processed_chunks": state.ProcessedChunks,
		"cache_hits":       state.Stats.CacheHits,
		"local_success":    state.Stats.LocalSuccess,
		"local_failure":    state.Stats.LocalFailure,
		"remote_fallback":  state.Stats.RemoteFallback,
		"start_time":       state.Timing.StartTime.Format(time.RFC3339),
		"last_update_time": state.Timing.LastUpdateTime.Format(time.RFC3339),
		"elapsed_seconds":  time.Since(state.Timing.StartTime).Seconds(),
		"checkpoint_count": len(state.Checkpoints),
	}

	// 计算命中率
	if state.ProcessedChunks > 0 {
		cacheHitRate := float64(state.Stats.CacheHits) / float64(state.ProcessedChunks) * 100
		localSuccessRate := float64(state.Stats.LocalSuccess) / float64(state.ProcessedChunks) * 100
		remoteFallbackRate := float64(state.Stats.RemoteFallback) / float64(state.ProcessedChunks) * 100
		
		stats["cache_hit_rate"] = fmt.Sprintf("%.1f%%", cacheHitRate)
		stats["local_success_rate"] = fmt.Sprintf("%.1f%%", localSuccessRate)
		stats["remote_fallback_rate"] = fmt.Sprintf("%.1f%%", remoteFallbackRate)
	}

	return stats
}

// SyncWithDatabase 与数据库同步状态
func (sm *StateManager) SyncWithDatabase(fileMd5 string) error {
	state, exists := sm.GetProcessingState(fileMd5)
	if !exists {
		return fmt.Errorf("processing state not found: %s", fileMd5)
	}

	// 更新数据库中的文件状态
	err := database.UpdateFileStatus(
		fileMd5,
		state.Status,
		state.CurrentStep,
		state.Progress,
		state.ErrorMsg,
	)
	if err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	// 同步块状态到缓存表
	repo := database.NewChunkCacheRepo()
	for chunkID, chunkState := range state.ChunkStates {
		if chunkState.Status == "completed" && chunkState.Source != "" {
			// 检查是否已存在缓存记录
			chunkHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("chunk_%d", chunkID))))
			existing, _ := repo.GetChunkRepair(fileMd5, chunkHash)
			
			if existing == nil {
				// 创建缓存记录
				cacheRecord := &database.ChunkRepairCacheRecord{
					FileMd5:          fileMd5,
					ChunkID:          chunkID,
					ChunkHash:        chunkHash,
					OriginalText:     "", // 需要从实际内容获取
					RepairedText:     "", // 需要从实际内容获取
					PromptVersion:    getDefaultPromptVersion(),
					APIModel:         chunkState.Source,
					TokenUsage:       0,
					ProcessingTimeMs: int(chunkState.ProcessingMs),
					Confidence:       chunkState.Confidence,
					Source:           chunkState.Source,
				}
				
				if err := repo.SaveChunkRepair(cacheRecord); err != nil {
					logging.Error("state_sync_cache_save_failed", map[string]interface{}{
						"file_md5": fileMd5,
						"chunk_id": chunkID,
						"error":    err.Error(),
					})
				}
			}
		}
	}

	logging.Info("state_sync_completed", map[string]interface{}{
		"file_md5":       fileMd5,
		"chunks_synced":  len(state.ChunkStates),
		"cache_records":  state.Stats.CacheHits,
	})

	return nil
}

// getMaxCheckpoints 获取最大检查点数量
func getMaxCheckpoints() int {
	return 100
}

// getDefaultPromptVersion 获取默认提示词版本
func getDefaultPromptVersion() string {
	return "state_sync"
}
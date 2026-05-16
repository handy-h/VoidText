package processor

import (
	"fmt"
	"sync"
	"voidtext/internal/database"
)

// ProcessingState 处理状态
type ProcessingState struct {
	FileMd5         string  `json:"fileMd5"`
	Status          string  `json:"status"`
	Step            string  `json:"step"`
	TotalChunks     int     `json:"totalChunks"`
	ProcessedChunks int     `json:"processedChunks"`
	Progress        float64 `json:"progress"`
}

// ChunkState 块处理状态
type ChunkState struct {
	FileMd5    string  `json:"fileMd5"`
	ChunkID    int     `json:"chunkId"`
	Status     string  `json:"status"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	DurationMs int64   `json:"durationMs"`
	ErrorMsg   string  `json:"errorMsg"`
}

// StateManager 状态管理器
type StateManager struct {
	mu          sync.RWMutex
	states      map[string]*ProcessingState
	chunkStates map[string]map[int]*ChunkState
}

var (
	stateManagerInstance *StateManager
	stateManagerOnce     sync.Once
)

// GetStateManager 获取状态管理器单例
func GetStateManager() *StateManager {
	stateManagerOnce.Do(func() {
		stateManagerInstance = &StateManager{
			states:      make(map[string]*ProcessingState),
			chunkStates: make(map[string]map[int]*ChunkState),
		}
	})
	return stateManagerInstance
}

// GetProcessingState 获取文件处理状态
func (sm *StateManager) GetProcessingState(fileMd5 string) (*ProcessingState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	state, exists := sm.states[fileMd5]
	if !exists {
		return nil, false
	}
	return state, true
}

// StartProcessing 开始处理
func (sm *StateManager) StartProcessing(fileMd5 string, totalChunks int, step string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[fileMd5] = &ProcessingState{
		FileMd5:     fileMd5,
		Status:      "processing",
		Step:        step,
		TotalChunks: totalChunks,
	}
	sm.chunkStates[fileMd5] = make(map[int]*ChunkState)
}

// UpdateChunkState 更新块状态
func (sm *StateManager) UpdateChunkState(fileMd5 string, chunkID int, status string, source string, confidence float64, durationMs int64, errMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if chunks, ok := sm.chunkStates[fileMd5]; ok {
		chunks[chunkID] = &ChunkState{
			FileMd5:    fileMd5,
			ChunkID:    chunkID,
			Status:     status,
			Source:     source,
			Confidence: confidence,
			DurationMs: durationMs,
			ErrorMsg:   errMsg,
		}
	}

	if state, ok := sm.states[fileMd5]; ok {
		if status == "completed" {
			state.ProcessedChunks++
		}
		if state.TotalChunks > 0 {
			state.Progress = float64(state.ProcessedChunks) / float64(state.TotalChunks) * 100
		}
	}
}

// FinishProcessing 完成处理
func (sm *StateManager) FinishProcessing(fileMd5, status, errMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if state, ok := sm.states[fileMd5]; ok {
		state.Status = status
		state.Progress = 100
	}
}

// ResumeProcessing 恢复处理，返回未完成的块ID列表
func (sm *StateManager) ResumeProcessing(fileMd5 string) ([]int, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	chunks, ok := sm.chunkStates[fileMd5]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", fileMd5)
	}

	state, ok := sm.states[fileMd5]
	if !ok {
		return nil, fmt.Errorf("state not found: %s", fileMd5)
	}

	var unfinished []int
	for i := 0; i < state.TotalChunks; i++ {
		chunk, exists := chunks[i]
		if !exists || chunk.Status != "completed" {
			unfinished = append(unfinished, i)
		}
	}
	return unfinished, nil
}

// SyncWithDatabase 同步状态到数据库
func (sm *StateManager) SyncWithDatabase(fileMd5 string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	state, ok := sm.states[fileMd5]
	if !ok {
		return nil
	}

	return database.UpdateFileStatus(fileMd5, state.Status, state.Step, int(state.Progress), "")
}

// UpdateFileStatusWithStepProgress 更新文件状态和步骤进度
func UpdateFileStatusWithStepProgress(fileMd5, status, currentStep string, progress int) error {
	return database.UpdateFileStatus(fileMd5, status, currentStep, progress, "")
}

// StartProcessingStep 开始处理步骤
func StartProcessingStep(fileMd5, step string) error {
	return database.UpdateFileStatusWithLog(fileMd5, "processing", step, CalculateProgress(step, 0), "", map[string]interface{}{
		"action": "step_started",
		"details": map[string]interface{}{
			"step": step,
		},
	})
}

// CompleteProcessingStep 完成处理步骤
func CompleteProcessingStep(fileMd5, step string, result map[string]interface{}) error {
	nextStep := GetNextStep(step)
	progress := CalculateProgress(nextStep, 0)

	return database.UpdateFileStatusWithLog(fileMd5, "processing", nextStep, progress, "", map[string]interface{}{
		"action": "step_completed",
		"details": map[string]interface{}{
			"step":   step,
			"result": result,
		},
	})
}

// FailProcessingStep 处理步骤失败
func FailProcessingStep(fileMd5, step string, err error) error {
	return database.UpdateFileStatusWithLog(fileMd5, "failed", step, 0, err.Error(), map[string]interface{}{
		"action": "step_failed",
		"details": map[string]interface{}{
			"step":  step,
			"error": err.Error(),
		},
	})
}

// SkipProcessingStep 跳过处理步骤
func SkipProcessingStep(fileMd5, step, reason string) error {
	nextStep := GetNextStep(step)
	progress := CalculateProgress(nextStep, 0)

	return database.UpdateFileStatusWithLog(fileMd5, "processing", nextStep, progress, "", map[string]interface{}{
		"action": "step_skipped",
		"details": map[string]interface{}{
			"step":   step,
			"reason": reason,
		},
	})
}

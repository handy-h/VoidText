package processor

import (
	"sync"
	"time"
)

// ProcessingProgress 处理进度追踪
type ProcessingProgress struct {
	progressMu        sync.RWMutex
	FileMd5           string
	TotalChunks       int
	ProcessedChunks   int
	CacheHits         int
	APICalls          int
	TotalProcessingMs int64
	StartTime         time.Time
	LastChunkTime     time.Time
	ChunkTimes        []int64 // 记录每个chunk的处理时间（毫秒）
}

// NewProcessingProgress 创建进度追踪器
func NewProcessingProgress(fileMd5 string, totalChunks int) *ProcessingProgress {
	// 限制切片容量，防止内存泄漏
	maxTrackedChunks := 1000
	if totalChunks > maxTrackedChunks {
		totalChunks = maxTrackedChunks
	}

	return &ProcessingProgress{
		FileMd5:       fileMd5,
		TotalChunks:   totalChunks,
		StartTime:     time.Now(),
		LastChunkTime: time.Now(),
		ChunkTimes:    make([]int64, 0, totalChunks),
	}
}

// RecordChunkComplete 记录一个chunk处理完成
func (pp *ProcessingProgress) RecordChunkComplete(cacheHit bool, processingMs int64) {
	pp.progressMu.Lock()
	defer pp.progressMu.Unlock()

	pp.ProcessedChunks++
	if cacheHit {
		pp.CacheHits++
	} else {
		pp.APICalls++
	}
	pp.TotalProcessingMs += processingMs
	pp.ChunkTimes = append(pp.ChunkTimes, processingMs)
	pp.LastChunkTime = time.Now()
}

// GetProgress 获取当前进度信息
func (pp *ProcessingProgress) GetProgress() ProgressInfo {
	pp.progressMu.RLock()
	defer pp.progressMu.RUnlock()

	// 计算平均处理时间（仅计算API调用，缓存命中时间可忽略）
	var avgChunkTimeMs int64
	if pp.APICalls > 0 {
		// 只计算API调用的平均时间
		apiTotalTime := pp.TotalProcessingMs
		avgChunkTimeMs = apiTotalTime / int64(pp.APICalls)
	}

	// 计算剩余chunk数
	remainingChunks := pp.TotalChunks - pp.ProcessedChunks

	// 预估剩余时间（秒）
	estimatedRemainingSeconds := int64(0)
	if remainingChunks > 0 && avgChunkTimeMs > 0 {
		// 预估剩余时间 = 剩余chunk数 * 平均处理时间
		// 考虑缓存命中率的影响：未命中才需要API调用
		missRate := 1.0 // 默认假设全部需要API调用
		if pp.ProcessedChunks > 0 {
			missRate = float64(pp.APICalls) / float64(pp.ProcessedChunks)
		}

		// 添加边界检查，防止整数溢出
		estimatedRemainingMs := int64(float64(remainingChunks) * missRate * float64(avgChunkTimeMs))
		if estimatedRemainingMs > 0 && estimatedRemainingMs < 1<<62 { // 防止溢出
			estimatedRemainingSeconds = estimatedRemainingMs / 1000
		}
	}

	// 计算进度百分比（使用浮点数提高精度）
	progress := 0
	if pp.TotalChunks > 0 {
		progress = int(float64(pp.ProcessedChunks) / float64(pp.TotalChunks) * 100)
	}

	return ProgressInfo{
		TotalChunks:            pp.TotalChunks,
		ProcessedChunks:        pp.ProcessedChunks,
		RemainingChunks:        remainingChunks,
		CacheHits:              pp.CacheHits,
		APICalls:               pp.APICalls,
		AvgChunkTimeMs:         avgChunkTimeMs,
		EstimatedRemainingSecs: estimatedRemainingSeconds,
		Progress:               progress,
		ElapsedSeconds:         int64(time.Since(pp.StartTime).Seconds()),
	}
}

// ProgressInfo 进度信息
type ProgressInfo struct {
	TotalChunks            int   `json:"totalChunks"`
	ProcessedChunks        int   `json:"processedChunks"`
	RemainingChunks        int   `json:"remainingChunks"`
	CacheHits              int   `json:"cacheHits"`
	APICalls               int   `json:"apiCalls"`
	AvgChunkTimeMs         int64 `json:"avgChunkTimeMs"`
	EstimatedRemainingSecs int64 `json:"estimatedRemainingSecs"`
	Progress               int   `json:"progress"`
	ElapsedSeconds         int64 `json:"elapsedSeconds"`
}

// GlobalProgressTracker 全局进度追踪器（内存存储）
var GlobalProgressTracker = &ProgressTracker{
	progresses: make(map[string]*ProcessingProgress),
}

// ProgressTracker 进度追踪器
type ProgressTracker struct {
	trackerMu   sync.RWMutex
	progresses map[string]*ProcessingProgress
}

// StartTracking 开始追踪文件处理进度
func (pt *ProgressTracker) StartTracking(fileMd5 string, totalChunks int) *ProcessingProgress {
	pt.trackerMu.Lock()
	defer pt.trackerMu.Unlock()

	pp := NewProcessingProgress(fileMd5, totalChunks)
	pt.progresses[fileMd5] = pp
	return pp
}

// GetProgress 获取文件处理进度
func (pt *ProgressTracker) GetProgress(fileMd5 string) (*ProgressInfo, bool) {
	pt.trackerMu.RLock()
	defer pt.trackerMu.RUnlock()

	pp, exists := pt.progresses[fileMd5]
	if !exists {
		return nil, false
	}

	info := pp.GetProgress()
	return &info, true
}

// FinishTracking 完成追踪，清理内存
func (pt *ProgressTracker) FinishTracking(fileMd5 string) {
	pt.trackerMu.Lock()
	defer pt.trackerMu.Unlock()

	delete(pt.progresses, fileMd5)
}

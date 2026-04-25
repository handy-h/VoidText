package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogEvent 结构化日志事件（混合架构增强版）
type LogEvent struct {
	Time          string                 `json:"time"`
	Level         LogLevel               `json:"level"`
	Event         string                 `json:"event"`
	FileMd5       string                 `json:"file_md5,omitempty"`
	ChunkID       int                    `json:"chunk_id,omitempty"`
	PromptVersion string                 `json:"prompt_version,omitempty"`
	InputPreview  string                 `json:"input_preview,omitempty"`
	RawError      string                 `json:"raw_error,omitempty"`
	ErrorType     string                 `json:"error_type,omitempty"`
	Context       string                 `json:"context,omitempty"`
	DurationMs    int64                  `json:"duration_ms,omitempty"`
	Source        string                 `json:"source,omitempty"` // 处理来源：local/remote/cache
	Confidence    float64                `json:"confidence,omitempty"` // 置信度
	Extra         map[string]interface{} `json:"extra,omitempty"`
	Caller        string                 `json:"caller,omitempty"`
}

// Logger 结构化日志记录器
type Logger struct {
	serviceName string
	promptFile  string
	enableJSON  bool
}

var (
	defaultLogger *Logger
)

func init() {
	defaultLogger = &Logger{
		serviceName: "voidtext",
		promptFile:  "config/prompt.txt",
		enableJSON:  true,
	}
}

// SetPromptFile 设置提示词文件路径
func SetPromptFile(path string) {
	defaultLogger.promptFile = path
}

// SetEnableJSON 设置是否启用JSON格式
func SetEnableJSON(enable bool) {
	defaultLogger.enableJSON = enable
}

// Log 记录结构化日志
func Log(level LogLevel, event string, fields ...map[string]interface{}) {
	defaultLogger.Log(level, event, fields...)
}

// Log 实例方法记录日志
func (l *Logger) Log(level LogLevel, event string, fields ...map[string]interface{}) {
	eventObj := LogEvent{
		Time:  time.Now().UTC().Format(time.RFC3339Nano),
		Level: level,
		Event: event,
	}

	// 合并额外字段
	if len(fields) > 0 {
		eventObj.Extra = fields[0]
	}

	// 获取调用者信息（跳过logging包自身的调用栈）
	if _, file, line, ok := runtime.Caller(2); ok {
		// 简化文件路径
		parts := strings.Split(file, "/")
		if len(parts) > 2 {
			file = strings.Join(parts[len(parts)-2:], "/")
		}
		eventObj.Caller = fmt.Sprintf("%s:%d", file, line)
	}

	// 输出JSON格式日志
	if l.enableJSON {
		jsonBytes, err := json.Marshal(eventObj)
		if err != nil {
			// 回退到简单格式
			fmt.Fprintf(os.Stderr, `{"time":"%s","level":"error","event":"log_marshal_failed","raw_error":"%s"}`+"\n",
				time.Now().UTC().Format(time.RFC3339Nano), err.Error())
			return
		}
		fmt.Fprintln(os.Stderr, string(jsonBytes))
	} else {
		// 简单文本格式（兼容旧日志）
		fmt.Fprintf(os.Stderr, "[%s] %s %s", strings.ToUpper(string(level)), eventObj.Time, event)
		if eventObj.Caller != "" {
			fmt.Fprintf(os.Stderr, " (%s)", eventObj.Caller)
		}
		if eventObj.Extra != nil && len(eventObj.Extra) > 0 {
			for k, v := range eventObj.Extra {
				fmt.Fprintf(os.Stderr, " %s=%v", k, v)
			}
		}
		fmt.Fprintln(os.Stderr)
	}
}

// 快捷方法

// Debug 记录调试日志
func Debug(event string, fields ...map[string]interface{}) {
	Log(LevelDebug, event, fields...)
}

// Info 记录信息日志
func Info(event string, fields ...map[string]interface{}) {
	Log(LevelInfo, event, fields...)
}

// Warn 记录警告日志
func Warn(event string, fields ...map[string]interface{}) {
	Log(LevelWarn, event, fields...)
}

// Error 记录错误日志
func Error(event string, fields ...map[string]interface{}) {
	Log(LevelError, event, fields...)
}

// APIError 记录API错误
func APIError(event string, chunkID int, promptVersion, inputPreview, rawError string) {
	Log(LevelError, event, map[string]interface{}{
		"chunk_id":       chunkID,
		"prompt_version": promptVersion,
		"input_preview":  inputPreview,
		"raw_error":      rawError,
		"error_type":     "api_error",
	})
}

// APIRefusal 记录API拒绝错误
func APIRefusal(chunkID int, promptVersion, inputPreview, rawError string) {
	Log(LevelError, "api_refusal", map[string]interface{}{
		"chunk_id":       chunkID,
		"prompt_version": promptVersion,
		"input_preview":  inputPreview,
		"raw_error":      rawError,
		"error_type":     "api_refusal",
	})
}

// ChunkProcessing 记录块处理日志
func ChunkProcessing(fileMd5 string, chunkID, totalChunks int, action string) {
	Log(LevelInfo, "chunk_processing", map[string]interface{}{
		"file_md5":    fileMd5,
		"chunk_id":    chunkID,
		"total_chunks": totalChunks,
		"action":      action,
	})
}

// CacheHit 记录缓存命中
func CacheHit(fileMd5 string, chunkID int, chunkHash string) {
	Log(LevelDebug, "cache_hit", map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"chunk_hash": chunkHash,
	})
}

// CacheMiss 记录缓存未命中
func CacheMiss(fileMd5 string, chunkID int, chunkHash string) {
	Log(LevelDebug, "cache_miss", map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"chunk_hash": chunkHash,
	})
}

// WorkerPool 记录Worker池状态
func WorkerPool(action string, workerID, totalWorkers, queueSize int) {
	Log(LevelDebug, "worker_pool", map[string]interface{}{
		"action":       action,
		"worker_id":    workerID,
		"total_workers": totalWorkers,
		"queue_size":   queueSize,
	})
}

// PromptLoaded 记录提示词加载
func PromptLoaded(version, source string, length int) {
	Log(LevelInfo, "prompt_loaded", map[string]interface{}{
		"prompt_version": version,
		"source":         source,
		"length":         length,
	})
}

// EvolverTriggered 记录Evolver触发
func EvolverTriggered(reason string, errorCount, threshold int) {
	Log(LevelWarn, "evolver_triggered", map[string]interface{}{
		"reason":       reason,
		"error_count":  errorCount,
		"threshold":    threshold,
	})
}

// PromptUpdated 记录提示词更新
func PromptUpdated(oldVersion, newVersion string, changes int) {
	Log(LevelInfo, "prompt_updated", map[string]interface{}{
		"old_version": oldVersion,
		"new_version": newVersion,
		"changes":     changes,
	})
}

// RetryQueued 记录重试队列
func RetryQueued(fileMd5 string, chunkID int, reason string) {
	Log(LevelWarn, "retry_queued", map[string]interface{}{
		"file_md5": fileMd5,
		"chunk_id": chunkID,
		"reason":   reason,
	})
}

// RetryProcessed 记录重试处理
func RetryProcessed(fileMd5 string, chunkID int, success bool) {
	level := LevelInfo
	if !success {
		level = LevelError
	}
	Log(level, "retry_processed", map[string]interface{}{
		"file_md5": fileMd5,
		"chunk_id": chunkID,
		"success":  success,
	})
}

// ProcessingSource 记录处理来源（混合架构新增）
func ProcessingSource(fileMd5 string, chunkID int, source string, confidence float64, durationMs int64, cacheHit bool) {
	extra := map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"source":     source,
		"confidence": confidence,
		"duration":   durationMs,
		"cache_hit":  cacheHit,
	}
	
	Log(LevelInfo, "processing_source", extra)
}

// LocalModelProcessed 记录本地模型处理结果
func LocalModelProcessed(fileMd5 string, chunkID int, success bool, confidence float64, durationMs int64, errorMsg string) {
	level := LevelInfo
	if !success {
		level = LevelError
	}
	
	extra := map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"success":    success,
		"confidence": confidence,
		"duration":   durationMs,
		"source":     "local",
	}
	
	if errorMsg != "" {
		extra["error"] = errorMsg
	}
	
	Log(level, "local_model_processed", extra)
}

// RemoteAPIFallback 记录远程API降级处理
func RemoteAPIFallback(fileMd5 string, chunkID int, success bool, durationMs int64, errorMsg string) {
	level := LevelInfo
	if !success {
		level = LevelError
	}
	
	extra := map[string]interface{}{
		"file_md5": fileMd5,
		"chunk_id": chunkID,
		"success":  success,
		"duration": durationMs,
		"source":   "remote",
	}
	
	if errorMsg != "" {
		extra["error"] = errorMsg
	}
	
	Log(level, "remote_api_fallback", extra)
}

// HealthCheckResult 记录健康检查结果
func HealthCheckResult(service string, healthy bool, durationMs int64, errorMsg string) {
	level := LevelInfo
	if !healthy {
		level = LevelWarn
	}
	
	extra := map[string]interface{}{
		"service":  service,
		"healthy":  healthy,
		"duration": durationMs,
	}
	
	if errorMsg != "" {
		extra["error"] = errorMsg
	}
	
	Log(level, "health_check", extra)
}

// StateCheckpoint 记录状态检查点
func StateCheckpoint(fileMd5 string, checkpointType string, data map[string]interface{}) {
	extra := map[string]interface{}{
		"file_md5":        fileMd5,
		"checkpoint_type": checkpointType,
	}
	
	for k, v := range data {
		extra[k] = v
	}
	
	Log(LevelInfo, "state_checkpoint", extra)
}
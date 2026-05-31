package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	// 默认日志级别
	currentLevel = INFO

	// 日志文件
	logFile *os.File

	// 是否启用文件日志
	enableFileLog = false

	// 是否启用结构化日志
	enableStructuredLog = false

	// 防止重复初始化
	initialized = false
)

// Config 日志配置
type Config struct {
	Level               LogLevel
	EnableFileLog       bool
	EnableConsoleLog    bool // 同时输出到控制台（开发模式）
	LogFilePath         string
	EnableStructuredLog bool
	MaxFileSize         int64 // 最大文件大小（字节）
	MaxBackupFiles      int   // 最大备份文件数
}

// Init 初始化日志系统（重复调用安全：第二次及之后调用不会重新打开文件句柄）
func Init(config Config) error {
	if initialized {
		return nil // 已初始化，避免重复打开文件句柄导致泄漏
	}

	currentLevel = config.Level
	enableStructuredLog = config.EnableStructuredLog

	if config.EnableFileLog && config.LogFilePath != "" {
		// 创建日志目录
		logDir := filepath.Dir(config.LogFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 打开日志文件
		file, err := os.OpenFile(config.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("打开日志文件失败: %w", err)
		}

		logFile = file
		enableFileLog = true

		// 设置日志输出：文件 + 可选控制台（开发模式）
		if config.EnableConsoleLog {
			log.SetOutput(io.MultiWriter(file, os.Stdout))
		} else {
			log.SetOutput(file)
		}
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	}

	initialized = true
	return nil
}

// Close 关闭日志系统
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// SetLevel 设置日志级别
func SetLevel(level LogLevel) {
	currentLevel = level
}

// shouldLog 检查是否应该记录日志
func shouldLog(level LogLevel) bool {
	return level >= currentLevel
}

// getCallerInfo 获取调用者信息
func getCallerInfo() (string, int) {
	// 跳过3层调用栈：getCallerInfo -> log函数 -> 实际调用者
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown", 0
	}

	// 只保留文件名
	return filepath.Base(file), line
}

// formatMessage 格式化消息
func formatMessage(level string, msg string, fields map[string]interface{}) string {
	if enableStructuredLog {
		// 结构化日志格式
		parts := []string{
			fmt.Sprintf("level=%s", level),
			fmt.Sprintf("msg=%q", msg),
		}

		// 添加调用者信息
		file, line := getCallerInfo()
		parts = append(parts, fmt.Sprintf("caller=%s:%d", file, line))

		// 添加时间戳
		parts = append(parts, fmt.Sprintf("time=%s", time.Now().Format(time.RFC3339)))

		// 添加自定义字段
		for k, v := range fields {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}

		return strings.Join(parts, " ")
	} else {
		// 传统日志格式
		file, line := getCallerInfo()
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		return fmt.Sprintf("[%s] [%s] %s:%d - %s", timestamp, level, file, line, msg)
	}
}

// Debug 调试日志
func Debug(msg string, fields ...map[string]interface{}) {
	if !shouldLog(DEBUG) {
		return
	}

	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}

	log.Println(formatMessage("DEBUG", msg, fieldMap))
}

// Info 信息日志
func Info(msg string, fields ...map[string]interface{}) {
	if !shouldLog(INFO) {
		return
	}

	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}

	log.Println(formatMessage("INFO", msg, fieldMap))
}

// Warn 警告日志
func Warn(msg string, fields ...map[string]interface{}) {
	if !shouldLog(WARN) {
		return
	}

	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}

	log.Println(formatMessage("WARN", msg, fieldMap))
}

// Error 错误日志
func Error(msg string, err error, fields ...map[string]interface{}) {
	if !shouldLog(ERROR) {
		return
	}

	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}

	if err != nil {
		fieldMap["error"] = err.Error()
	}

	log.Println(formatMessage("ERROR", msg, fieldMap))
}

// Fatal 致命错误日志
func Fatal(msg string, err error, fields ...map[string]interface{}) {
	if !shouldLog(FATAL) {
		return
	}

	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}

	if err != nil {
		fieldMap["error"] = err.Error()
	}

	log.Println(formatMessage("FATAL", msg, fieldMap))
	// 同步日志文件，确保致命错误前的日志不丢失；随后 panic 以触发 defer 清理
	if logFile != nil {
		_ = logFile.Sync()
	}
	panic(msg)
}

// WithFields 创建带字段的日志记录器
func WithFields(fields map[string]interface{}) *Logger {
	return &Logger{fields: fields}
}

// Logger 带字段的日志记录器
type Logger struct {
	fields map[string]interface{}
}

// Debug 调试日志
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(l.fields, fields...)
	Debug(msg, mergedFields)
}

// Info 信息日志
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(l.fields, fields...)
	Info(msg, mergedFields)
}

// Warn 警告日志
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(l.fields, fields...)
	Warn(msg, mergedFields)
}

// Error 错误日志
func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	mergedFields := mergeFields(l.fields, fields...)
	Error(msg, err, mergedFields)
}

// Fatal 致命错误日志
func (l *Logger) Fatal(msg string, err error, fields ...map[string]interface{}) {
	mergedFields := mergeFields(l.fields, fields...)
	Fatal(msg, err, mergedFields)
}

// mergeFields 合并字段
func mergeFields(base map[string]interface{}, additional ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制基础字段
	for k, v := range base {
		result[k] = v
	}

	// 合并附加字段
	for _, fields := range additional {
		for k, v := range fields {
			result[k] = v
		}
	}

	return result
}

// 包级日志记录器
var (
	// 默认日志记录器
	defaultLogger = &Logger{}

	// 处理相关日志记录器
	processingLogger = WithFields(map[string]interface{}{"component": "processor"})

	// 数据库相关日志记录器
	databaseLogger = WithFields(map[string]interface{}{"component": "database"})

	// API相关日志记录器
	apiLogger = WithFields(map[string]interface{}{"component": "api"})

	// 文件相关日志记录器
	fileLogger = WithFields(map[string]interface{}{"component": "file"})
)

func APIRefusal(chunkID int, promptVersion, inputPreview, rawError string) {
	apiLogger.Warn("api_refusal", map[string]interface{}{
		"chunk_id":       chunkID,
		"prompt_version": promptVersion,
		"input_preview":  inputPreview,
		"raw_error":      rawError,
	})
}

func CacheHit(fileMd5 string, chunkID int, chunkHash string) {
	apiLogger.Debug("cache_hit", map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"chunk_hash": chunkHash,
	})
}

func EvolverTriggered(reason string, errorRate, threshold int) {
	apiLogger.Warn("evolver_triggered", map[string]interface{}{
		"reason":     reason,
		"error_rate": errorRate,
		"threshold":  threshold,
	})
}

func HealthCheckResult(service string, healthy bool, duration int64, errMsg string) {
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	fields := map[string]interface{}{
		"service":  service,
		"status":   status,
		"duration": duration,
	}
	if errMsg != "" {
		fields["error"] = errMsg
	}
	apiLogger.Info("health_check_result", fields)
}

func CacheMiss(fileMd5 string, chunkID int, chunkHash string) {
	apiLogger.Debug("cache_miss", map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"chunk_hash": chunkHash,
	})
}

func ProcessingSource(fileMd5 string, chunkID int, source string, confidence float64, duration int64, isCache bool) {
	apiLogger.Info("processing_source", map[string]interface{}{
		"file_md5":   fileMd5,
		"chunk_id":   chunkID,
		"source":     source,
		"confidence": confidence,
		"duration":   duration,
		"is_cache":   isCache,
	})
}

func PromptLoaded(version, source string, promptLen int) {
	apiLogger.Info("prompt_loaded", map[string]interface{}{
		"version":    version,
		"source":     source,
		"prompt_len": promptLen,
	})
}

func PromptUpdated(oldVersion, newVersion string, diffLen int) {
	apiLogger.Info("prompt_updated", map[string]interface{}{
		"old_version": oldVersion,
		"new_version": newVersion,
		"diff_len":    diffLen,
	})
}

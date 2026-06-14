package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// ModelEndpoint 模型端点配置（URL + 密钥 + 模型名）
type ModelEndpoint struct {
	URL       string
	APIKey    string
	ModelName string
	MaxTokens int // 该端点的最大输出 token 数，0 表示使用全局默认值
}

// AppConfig 应用配置
type AppConfig struct {
	Port           int
	DataDir        string
	BaseDir        string
	MaxFileSize    int64
	BackupKeepDays int

	EnableBasicCleaning bool
	BasicCleaningTool   string
	TraditionalToSimple bool

	EnableVectorDetection     bool
	VectorModelName           string
	VectorModelType           string
	VectorSimilarityThreshold float64
	VectorModelURL            string
	VectorModelApiKey         string

	EnableModelRepair     bool
	RepairModelName       string
	RepairModelType       string
	LLMApiURL             string
	LLMApiKey             string
	CompletionModelName   string
	ModelEndpoints        []ModelEndpoint // 多模型端点列表（含主模型 + 备用模型）
	CompletionTemperature float64
	CompletionMaxTokens   int
	LLMMaxOutputTokens    int // LLM 最大输出 token 数上限（防止 max_tokens 超出模型限制）
	LLMConcurrency        int
	LLMDisableThinking   bool // 禁用模型思维链（如 mimo-v2.5 的 thinking 模式）

	EnableLlmParagraphReconstruct bool
	ParagraphChunkSize            int
	MinParagraphLength            int // 换行修复：短段落合并阈值（字符数），低于此值的段落会与后续段落合并

	EnableLocalModel  bool
	LocalModelURL     string
	LocalModelName    string
	LocalModelTimeout int

	NameSeparators string
}

var AppConfigInstance AppConfig
var configLoadOnce sync.Once

// Load 加载配置（线程安全，多次调用仅首次生效）
func Load() error {
	var loadErr error
	configLoadOnce.Do(func() {
		loadErr = doLoad()
	})
	return loadErr
}

func doLoad() error {
	// 按优先级依次尝试加载 .env：可执行文件目录 → 工作目录
	paths := []string{""}
	if execPath, err := os.Executable(); err == nil {
		paths = append([]string{filepath.Join(filepath.Dir(execPath), ".env")}, paths...)
	}
	for _, p := range paths {
		if p == "" {
			if godotenv.Load() == nil {
				break
			}
		} else {
			log.Printf("尝试从 %s 加载配置", p)
			if godotenv.Load(p) == nil {
				break
			}
		}
	}

	// 获取当前工作目录作为 BaseDir
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}

	cfg := AppConfig{
		Port:                          getEnvInt("PORT", 8080),
		DataDir:                       getEnvStr("DATA_DIR", "./data"),
		BaseDir:                       baseDir,
		MaxFileSize:                   getEnvInt64("MAX_FILE_SIZE", 100*1024*1024),
		BackupKeepDays:                getEnvInt("BACKUP_KEEP_DAYS", 7),
		EnableBasicCleaning:           getEnvBool("ENABLE_BASIC_CLEANING", true),
		BasicCleaningTool:             getEnvStr("BASIC_CLEANING_TOOL", "regex"),
		TraditionalToSimple:           getEnvBool("TRADITIONAL_TO_SIMPLE", false),
		EnableVectorDetection:         getEnvBool("ENABLE_VECTOR_DETECTION", true),
		VectorModelName:               getEnvStr("VECTOR_MODEL_NAME", "all-MiniLM-L6-v2"),
		VectorModelType:               getEnvStr("VECTOR_MODEL_TYPE", "local"),
		VectorSimilarityThreshold:     getEnvFloat("VECTOR_SIMILARITY_THRESHOLD", 0.95),
		VectorModelURL:                getEnvStr("VECTOR_MODEL_URL", ""),
		VectorModelApiKey:             getEnvStr("VECTOR_MODEL_API_KEY", ""),
		EnableModelRepair:             getEnvBool("ENABLE_MODEL_REPAIR", true),
		RepairModelName:               getEnvStr("REPAIR_MODEL_NAME", "gpt-3.5-turbo-instruct"),
		RepairModelType:               getEnvStr("REPAIR_MODEL_TYPE", "api"),
		LLMApiURL:                     getEnvStr("LLM_API_URL", ""),
		LLMApiKey:                     getEnvStr("LLM_API_KEY", ""),
		CompletionModelName:           getEnvStr("COMPLETION_MODEL_NAME", "gpt-3.5-turbo-instruct"),
		CompletionTemperature:         getEnvFloat("COMPLETION_TEMPERATURE", 0.3),
		CompletionMaxTokens:           getEnvInt("COMPLETION_MAX_TOKENS", 2048),
		LLMMaxOutputTokens:            getEnvInt("LLM_MAX_OUTPUT_TOKENS", 4096),
		LLMConcurrency:                getEnvInt("LLM_CONCURRENCY", 2),
		LLMDisableThinking:           getEnvBool("LLM_DISABLE_THINKING", false),
		EnableLlmParagraphReconstruct: getEnvBool("ENABLE_LLM_PARAGRAPH_RECONSTRUCT", true),
		ParagraphChunkSize:            getEnvInt("PARAGRAPH_CHUNK_SIZE", 8000),
		MinParagraphLength:            getEnvInt("MIN_PARAGRAPH_LENGTH", 80),
		EnableLocalModel:              getEnvBool("ENABLE_LOCAL_MODEL", false),
		LocalModelURL:                 getEnvStr("LOCAL_MODEL_URL", "http://localhost:11434"),
		LocalModelName:                getEnvStr("LOCAL_MODEL_NAME", "qwen2.5"),
		LocalModelTimeout:             getEnvInt("LOCAL_MODEL_TIMEOUT", 30),
		NameSeparators:                getEnvStr("NAME_SEPARATORS", "-|—|·|_| "),
	}
	if cfg.LLMConcurrency < 1 {
		cfg.LLMConcurrency = 1
	}

	if cfg.VectorModelURL == "" {
		cfg.VectorModelURL = getEnvStr("EXTERNAL_API_URL", "")
	}
	if cfg.VectorModelApiKey == "" {
		cfg.VectorModelApiKey = getEnvStr("EXTERNAL_API_KEY", "")
	}
	if cfg.LLMApiURL == "" {
		cfg.LLMApiURL = getEnvStr("EXTERNAL_API_URL", "")
	}
	if cfg.LLMApiKey == "" {
		cfg.LLMApiKey = getEnvStr("EXTERNAL_API_KEY", "")
	}
	// 构建多模型端点列表（主模型 + 最多2个备用模型）
	cfg.ModelEndpoints = buildModelEndpoints(cfg)

	AppConfigInstance = cfg

	var errs []string
	for _, dir := range []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "uploads"),
		filepath.Join(cfg.DataDir, "backups"),
		filepath.Join(cfg.DataDir, "temp"),
	} {
		if err := ensureDir(dir); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", dir, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("创建目录失败: %s", strings.Join(errs, "; "))
	}

	return nil
}

// GetNameSeparators 获取文件名分隔符列表（自动去重）
func (c *AppConfig) GetNameSeparators() []string {
	parts := strings.Split(c.NameSeparators, "|")
	var result []string
	seen := make(map[string]bool)
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == " " {
			if !seen[" "] {
				result = append(result, " ")
				seen[" "] = true
			}
		} else if trimmed != "" {
			if !seen[trimmed] {
				result = append(result, trimmed)
				seen[trimmed] = true
			}
		}
	}
	return result
}

func ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
		}
	}
	return nil
}

func getEnvStr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
		log.Printf("[配置] 环境变量 %s 的值 '%s' 无法解析为整数，使用默认值 %d", key, val, fallback)
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
		log.Printf("[配置] 环境变量 %s 的值 '%s' 无法解析为 int64，使用默认值 %d", key, val, fallback)
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
		log.Printf("[配置] 环境变量 %s 的值 '%s' 无法解析为布尔值，使用默认值 %v", key, val, fallback)
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		log.Printf("[配置] 环境变量 %s 的值 '%s' 无法解析为浮点数，使用默认值 %f", key, val, fallback)
	}
	return fallback
}

// Validate 验证配置有效性
func Validate() error {
	cfg := AppConfigInstance
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("无效端口号: %d", cfg.Port)
	}
	if cfg.VectorSimilarityThreshold < 0 || cfg.VectorSimilarityThreshold > 1 {
		return fmt.Errorf("无效相似度阈值: %f", cfg.VectorSimilarityThreshold)
	}
	if cfg.CompletionTemperature < 0 || cfg.CompletionTemperature > 2 {
		return fmt.Errorf("无效生成温度: %f", cfg.CompletionTemperature)
	}
	if cfg.LLMMaxOutputTokens < 1 || cfg.LLMMaxOutputTokens > 128000 {
		return fmt.Errorf("无效LLM最大输出token数: %d（范围: 1~128000）", cfg.LLMMaxOutputTokens)
	}
	return nil
}

// parseModelNames 解析逗号分隔的模型名列表，最多返回 5 个
// 支持单个模型名（向后兼容）和逗号分隔的多个模型名
func parseModelNames(names string) []string {
	parts := strings.Split(names, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
		if len(result) >= 5 {
			break
		}
	}
	return result
}

// buildModelEndpoints 从环境变量构建多模型端点列表
// 第一服务商：COMPLETION_MODEL_NAME 支持逗号分隔最多 5 个模型名（共享 URL/APIKey）
// 备用服务商2：LLM_API_URL_2/LLM_API_KEY_2/COMPLETION_MODEL_NAME_2
// 备用服务商3：LLM_API_URL_3/LLM_API_KEY_3/COMPLETION_MODEL_NAME_3
// URL 和 APIKey 必须同时非空才算有效端点
func buildModelEndpoints(cfg AppConfig) []ModelEndpoint {
	var endpoints []ModelEndpoint

	// 第一服务商：解析逗号分隔的模型名，每个模型名生成一个端点（共享 URL/APIKey）
	if cfg.LLMApiURL != "" && cfg.LLMApiKey != "" {
		modelNames := parseModelNames(cfg.CompletionModelName)
		for _, name := range modelNames {
			endpoints = append(endpoints, ModelEndpoint{
				URL:       cfg.LLMApiURL,
				APIKey:    cfg.LLMApiKey,
				ModelName: name,
			})
		}
	}

	// 备用服务商2
	model2URL := getEnvStr("LLM_API_URL_2", "")
	model2Key := getEnvStr("LLM_API_KEY_2", "")
	model2Name := getEnvStr("COMPLETION_MODEL_NAME_2", cfg.CompletionModelName)
	if model2URL != "" && model2Key != "" {
		endpoints = append(endpoints, ModelEndpoint{
			URL:       model2URL,
			APIKey:    model2Key,
			ModelName: model2Name,
		})
	}

	// 备用服务商3
	model3URL := getEnvStr("LLM_API_URL_3", "")
	model3Key := getEnvStr("LLM_API_KEY_3", "")
	model3Name := getEnvStr("COMPLETION_MODEL_NAME_3", cfg.CompletionModelName)
	if model3URL != "" && model3Key != "" {
		endpoints = append(endpoints, ModelEndpoint{
			URL:       model3URL,
			APIKey:    model3Key,
			ModelName: model3Name,
		})
	}

	return endpoints
}

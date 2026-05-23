package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

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
	CompletionTemperature float64
	CompletionMaxTokens   int
	LLMConcurrency        int

	EnableLlmParagraphReconstruct bool
	ParagraphChunkSize            int

	EnableLocalModel   bool
	LocalModelURL      string
	LocalModelName     string
	LocalModelTimeout  int

	NameSeparators string
}

var AppConfigInstance AppConfig

// Load 加载配置
func Load() error {
	// 优先从可执行文件所在目录加载 .env
	execPath, err := os.Executable()
	if err == nil {
		envPath := filepath.Join(filepath.Dir(execPath), ".env")
		log.Printf("尝试从可执行文件目录加载配置: %s", envPath)
		if loadErr := godotenv.Load(envPath); loadErr != nil {
			log.Printf("从可执行文件目录加载失败: %v，尝试从工作目录加载", loadErr)
			if loadErr2 := godotenv.Load(); loadErr2 != nil {
				log.Printf("从工作目录加载也失败: %v", loadErr2)
			}
		}
	} else {
		log.Printf("获取可执行文件路径失败: %v，尝试从工作目录加载", err)
		if loadErr := godotenv.Load(); loadErr != nil {
			log.Printf("从工作目录加载失败: %v", loadErr)
		}
	}

	// 打印实际加载的环境变量值
	log.Printf("DATA_DIR=%s", os.Getenv("DATA_DIR"))

	// 获取当前工作目录作为 BaseDir
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}

	cfg := AppConfig{
		Port:                      getEnvInt("PORT", 8080),
		DataDir:                   getEnvStr("DATA_DIR", "./data"),
		BaseDir:                   baseDir,
		MaxFileSize:               getEnvInt64("MAX_FILE_SIZE", 100*1024*1024),
		BackupKeepDays:            getEnvInt("BACKUP_KEEP_DAYS", 7),
		EnableBasicCleaning:       getEnvBool("ENABLE_BASIC_CLEANING", true),
		BasicCleaningTool:         getEnvStr("BASIC_CLEANING_TOOL", "regex"),
		TraditionalToSimple:       getEnvBool("TRADITIONAL_TO_SIMPLE", false),
		EnableVectorDetection:     getEnvBool("ENABLE_VECTOR_DETECTION", true),
		VectorModelName:           getEnvStr("VECTOR_MODEL_NAME", "all-MiniLM-L6-v2"),
		VectorModelType:           getEnvStr("VECTOR_MODEL_TYPE", "local"),
		VectorSimilarityThreshold: getEnvFloat("VECTOR_SIMILARITY_THRESHOLD", 0.95),
		VectorModelURL:            getEnvStr("VECTOR_MODEL_URL", ""),
		VectorModelApiKey:         getEnvStr("VECTOR_MODEL_API_KEY", ""),
		EnableModelRepair:         getEnvBool("ENABLE_MODEL_REPAIR", true),
		RepairModelName:           getEnvStr("REPAIR_MODEL_NAME", "gpt-3.5-turbo-instruct"),
		RepairModelType:           getEnvStr("REPAIR_MODEL_TYPE", "api"),
		LLMApiURL:                 getEnvStr("LLM_API_URL", ""),
		LLMApiKey:                 getEnvStr("LLM_API_KEY", ""),
		CompletionModelName:       getEnvStr("COMPLETION_MODEL_NAME", "gpt-3.5-turbo-instruct"),
		CompletionTemperature:     getEnvFloat("COMPLETION_TEMPERATURE", 0.3),
		CompletionMaxTokens:           getEnvInt("COMPLETION_MAX_TOKENS", 2048),
		LLMConcurrency:                getEnvInt("LLM_CONCURRENCY", 2),
		EnableLlmParagraphReconstruct: getEnvBool("ENABLE_LLM_PARAGRAPH_RECONSTRUCT", false),
		ParagraphChunkSize:            getEnvInt("PARAGRAPH_CHUNK_SIZE", 8000),
		EnableLocalModel:              getEnvBool("ENABLE_LOCAL_MODEL", false),
		LocalModelURL:                 getEnvStr("LOCAL_MODEL_URL", "http://localhost:11434"),
		LocalModelName:                getEnvStr("LOCAL_MODEL_NAME", "qwen2.5"),
		LocalModelTimeout:             getEnvInt("LOCAL_MODEL_TIMEOUT", 30),
		NameSeparators:                getEnvStr("NAME_SEPARATORS", "-|—|·|·|_| "),
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
	if cfg.CompletionModelName == "" {
		cfg.CompletionModelName = getEnvStr("EMBEDDING_MODEL_NAME", "gpt-3.5-turbo-instruct")
	}

	AppConfigInstance = cfg

	ensureDir(cfg.DataDir)
	ensureDir(filepath.Join(cfg.DataDir, "uploads"))
	ensureDir(filepath.Join(cfg.DataDir, "backups"))
	ensureDir(filepath.Join(cfg.DataDir, "temp"))

	return nil
}

// GetNameSeparators 获取文件名分隔符列表
func (c *AppConfig) GetNameSeparators() []string {
	parts := strings.Split(c.NameSeparators, "|")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" || p == " " {
			if p == " " {
				result = append(result, " ")
			} else {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

func ensureDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
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
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
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
	return nil
}

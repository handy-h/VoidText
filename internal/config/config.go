package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// AppConfig 应用配置
type AppConfig struct {
	Port                   int    `mapstructure:"port"`
	DataDir                string `mapstructure:"data_dir"`
	ModelsDir              string `mapstructure:"models_dir"`
	MaxFileSize            int64  `mapstructure:"max_file_size"`
	BackupKeepDays         int    `mapstructure:"backup_keep_days"`
	
	// 第一阶段：基础文本清洗配置
	EnableBasicCleaning    bool   `mapstructure:"enable_basic_cleaning"`
	BasicCleaningTool      string `mapstructure:"basic_cleaning_tool"`
	TraditionalToSimple    bool   `mapstructure:"traditional_to_simple"`
	
	// 第二阶段：向量检测配置
	EnableVectorDetection  bool   `mapstructure:"enable_vector_detection"`
	VectorModelName        string `mapstructure:"vector_model_name"`
	VectorModelType        string `mapstructure:"vector_model_type"` // local 或 api
	VectorSimilarityThreshold float64 `mapstructure:"vector_similarity_threshold"`
	
	// 第三阶段：模型修复配置
	EnableModelRepair      bool   `mapstructure:"enable_model_repair"`
	RepairModelName        string `mapstructure:"repair_model_name"`
	RepairModelType        string `mapstructure:"repair_model_type"` // local 或 api
	
	// 外部API配置
	ExternalAPIKey         string `mapstructure:"external_api_key"`
	ExternalAPIURL         string `mapstructure:"external_api_url"`
	EmbeddingModelName     string `mapstructure:"embedding_model_name"`
	CompletionModelName    string `mapstructure:"completion_model_name"`
	
	// 文本生成模型调用参数
	CompletionTemperature  float64 `mapstructure:"completion_temperature"`
	CompletionMaxTokens    int     `mapstructure:"completion_max_tokens"`
}

// Global application config
var AppConfigInstance AppConfig

// Load 加载配置
func Load() error {
	// 设置默认值
	viper.SetDefault("port", 8080)
	viper.SetDefault("data_dir", "./data")
	viper.SetDefault("models_dir", "./models")
	viper.SetDefault("max_file_size", 100*1024*1024) // 100MB
	viper.SetDefault("backup_keep_days", 7)
	
	// 第一阶段：基础文本清洗默认值
	viper.SetDefault("enable_basic_cleaning", true)
	viper.SetDefault("basic_cleaning_tool", "regex")
	viper.SetDefault("traditional_to_simple", false)
	
	// 第二阶段：向量检测默认值
	viper.SetDefault("enable_vector_detection", true)
	viper.SetDefault("vector_model_name", "all-MiniLM-L6-v2")
	viper.SetDefault("vector_model_type", "local")
	viper.SetDefault("vector_similarity_threshold", 0.95)
	
	// 第三阶段：模型修复默认值
	viper.SetDefault("enable_model_repair", true)
	viper.SetDefault("repair_model_name", "gpt-3.5-turbo-instruct")
	viper.SetDefault("repair_model_type", "api")
	
	// 外部API默认值
	viper.SetDefault("embedding_model_name", "text-embedding-ada-002")
	viper.SetDefault("completion_model_name", "gpt-3.5-turbo-instruct")
	viper.SetDefault("completion_temperature", 0.3)
	viper.SetDefault("completion_max_tokens", 2048)

	// 从配置文件加载
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 从环境变量加载
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 配置文件不存在时使用默认值
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// 解析配置
	if err := viper.Unmarshal(&AppConfigInstance); err != nil {
		return err
	}

	// 确保数据目录存在
	ensureDir(AppConfigInstance.DataDir)
	ensureDir(filepath.Join(AppConfigInstance.DataDir, "uploads"))
	ensureDir(filepath.Join(AppConfigInstance.DataDir, "backups"))
	ensureDir(filepath.Join(AppConfigInstance.DataDir, "temp"))

	// 确保模型目录存在
	ensureDir(AppConfigInstance.ModelsDir)

	return nil
}

// 确保目录存在
func ensureDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
}
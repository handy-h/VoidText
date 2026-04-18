package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// AppConfig 应用配置
type AppConfig struct {
	Port            int    `mapstructure:"port"`
	DataDir         string `mapstructure:"data_dir"`
	ModelsDir       string `mapstructure:"models_dir"`
	MaxFileSize     int64  `mapstructure:"max_file_size"`
	BackupKeepDays  int    `mapstructure:"backup_keep_days"`
	ExternalAPIKey  string `mapstructure:"external_api_key"`
	ExternalAPIURL  string `mapstructure:"external_api_url"`
}

// Global application config
var AppConfig AppConfig

// Load 加载配置
func Load() error {
	// 设置默认值
	viper.SetDefault("port", 8080)
	viper.SetDefault("data_dir", "./data")
	viper.SetDefault("models_dir", "./models")
	viper.SetDefault("max_file_size", 100*1024*1024) // 100MB
	viper.SetDefault("backup_keep_days", 7)

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
	if err := viper.Unmarshal(&AppConfig); err != nil {
		return err
	}

	// 确保数据目录存在
	ensureDir(AppConfig.DataDir)
	ensureDir(filepath.Join(AppConfig.DataDir, "uploads"))
	ensureDir(filepath.Join(AppConfig.DataDir, "backups"))
	ensureDir(filepath.Join(AppConfig.DataDir, "temp"))

	// 确保模型目录存在
	ensureDir(AppConfig.ModelsDir)

	return nil
}

// 确保目录存在
func ensureDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
}
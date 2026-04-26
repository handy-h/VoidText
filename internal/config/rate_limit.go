package config

import (
  "time"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
  // 全局限流配置
  Global struct {
    MaxRequests int           `yaml:"max_requests" json:"max_requests"`
    Window      time.Duration `yaml:"window" json:"window"`
    Cleanup     time.Duration `yaml:"cleanup" json:"cleanup"`
  } `yaml:"global" json:"global"`
  
  // 上传文件限流配置
  Upload struct {
    MaxRequests int           `yaml:"max_requests" json:"max_requests"`
    Window      time.Duration `yaml:"window" json:"window"`
  } `yaml:"upload" json:"upload"`
  
  // API限流配置
  API struct {
    MaxRequests int           `yaml:"max_requests" json:"max_requests"`
    Window      time.Duration `yaml:"window" json:"window"`
  } `yaml:"api" json:"api"`
  
  // 严格限流配置
  Strict struct {
    MaxRequests int           `yaml:"max_requests" json:"max_requests"`
    Window      time.Duration `yaml:"window" json:"window"`
  } `yaml:"strict" json:"strict"`
  
  // 端点限流配置
  Endpoint struct {
    MaxRequests int           `yaml:"max_requests" json:"max_requests"`
    Window      time.Duration `yaml:"window" json:"window"`
  } `yaml:"endpoint" json:"endpoint"`
  
  // 是否启用限流
  Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultRateLimitConfig 返回默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
  config := &RateLimitConfig{
    Enabled: true,
  }
  
  // 全局配置：每分钟100个请求，每5分钟清理一次
  config.Global.MaxRequests = 100
  config.Global.Window = time.Minute
  config.Global.Cleanup = 5 * time.Minute
  
  // 上传配置：每分钟10个请求
  config.Upload.MaxRequests = 10
  config.Upload.Window = time.Minute
  
  // API配置：每分钟60个请求
  config.API.MaxRequests = 60
  config.API.Window = time.Minute
  
  // 严格配置：每分钟30个请求
  config.Strict.MaxRequests = 30
  config.Strict.Window = time.Minute
  
  // 端点配置：每分钟50个请求
  config.Endpoint.MaxRequests = 50
  config.Endpoint.Window = time.Minute
  
  return config
}

// GetRateLimitConfig 获取限流配置
func GetRateLimitConfig() *RateLimitConfig {
  if AppConfigInstance.RateLimit != nil {
    return AppConfigInstance.RateLimit
  }
  
  return DefaultRateLimitConfig()
}
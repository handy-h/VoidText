package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/logging"
	"voidtext/internal/processor"
	"voidtext/web/backend"
	"voidtext/web/backend/middleware"
)

func main() {
	// 初始化配置
	if err := config.Load(); err != nil {
		// 配置加载阶段尚未初始化 logging，仍使用标准 log
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志系统
	logConfig := logging.Config{
		Level:               logging.INFO,
		EnableFileLog:       true,
		EnableConsoleLog:    os.Getenv("LOG_TO_CONSOLE") == "true",
		LogFilePath:         filepath.Join(config.AppConfigInstance.DataDir, "logs", "voidtext.log"),
		EnableStructuredLog: true,
		MaxFileSize:         10 * 1024 * 1024, // 10MB
		MaxBackupFiles:      5,
	}

	if err := logging.Init(logConfig); err != nil {
		log.Fatalf("Failed to init logging: %v", err)
	}
	defer logging.Close()

	logging.Info("应用程序启动", map[string]interface{}{
		"version":  "1.0.0",
		"data_dir": config.AppConfigInstance.DataDir,
	})

	// 初始化数据库
	if err := database.Init(config.AppConfigInstance.DataDir); err != nil {
		logging.Fatal("初始化数据库失败", err, nil)
	}
	defer database.Close()

	logging.Info("数据库初始化完成", nil)

	// 清理服务器重启后残留的 processing 状态
	if err := database.CleanupStaleProcessingStatus(); err != nil {
		logging.Warn("清理残留状态失败", map[string]interface{}{"error": err.Error()})
	}

	// 初始化Web服务
	processor.GetHealthManager().Start()
	server := backend.NewServer()
	defer middleware.CloseAllRateLimiters()

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.AppConfigInstance.Port),
		Handler: server,
	}

	// 在goroutine中启动服务器
	go func() {
		logging.Info("服务器启动", map[string]interface{}{"port": config.AppConfigInstance.Port})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 使用 logging.Fatal 替代 log.Fatalf，确保 defer 能正常执行（数据库关闭、日志刷新等）
			logging.Fatal("服务器启动失败", err, nil)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logging.Info("正在关闭服务器...", nil)

	// 1. 先关闭工作池（等待现有任务完成，阻止新任务提交）
	logging.Info("正在关闭工作池...", nil)
	pool := processor.GetWorkerPool()
	pool.Shutdown()

	// 2. 再关闭 HTTP 服务器（设置独立超时，避免与工作池竞争）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// 使用 logging.Error 替代 log.Fatalf，确保后续 defer 清理能执行
		logging.Error("服务器强制关闭", err, nil)
	}

	// 3. 关闭健康检查管理器
	processor.GetHealthManager().Stop()

	logging.Info("服务器已退出", nil)
}

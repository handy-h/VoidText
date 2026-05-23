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
)

func main() {
	// 初始化配置
	if err := config.Load(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志系统
	logConfig := logging.Config{
		Level:              logging.INFO,
		EnableFileLog:      true,
		EnableConsoleLog:   os.Getenv("LOG_TO_CONSOLE") == "true",
		LogFilePath:        filepath.Join(config.AppConfigInstance.DataDir, "logs", "voidtext.log"),
		EnableStructuredLog: true,
		MaxFileSize:        10 * 1024 * 1024, // 10MB
		MaxBackupFiles:     5,
	}
	
	if err := logging.Init(logConfig); err != nil {
		log.Fatalf("Failed to init logging: %v", err)
	}
	defer logging.Close()
	
	logging.Info("应用程序启动", map[string]interface{}{
		"version": "1.0.0",
		"data_dir": config.AppConfigInstance.DataDir,
	})

	// 初始化数据库
	if err := database.Init(config.AppConfigInstance.DataDir); err != nil {
		logging.Fatal("初始化数据库失败", err, nil)
	}
	defer database.Close()
	
	logging.Info("数据库初始化完成", nil)

	// 初始化Web服务
	processor.GetHealthManager().Start()
	server := backend.NewServer()

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.AppConfigInstance.Port),
		Handler: server,
	}

	// 在goroutine中启动服务器
	go func() {
		log.Printf("Server started on port %d", config.AppConfigInstance.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 设置5秒的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 优雅地关闭服务器
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// 关闭工作池
	log.Println("正在关闭工作池...")
	pool := processor.GetWorkerPool()
	pool.Shutdown()

	log.Println("Server exited")
}
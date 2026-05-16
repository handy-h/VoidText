package backend

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"voidtext/web/backend/handlers"
	"voidtext/web/backend/middleware"
)

// NewServer 创建新的Web服务器
func NewServer() *gin.Engine {
	r := gin.Default()

	// 添加错误处理中间件
	r.Use(middleware.Recovery())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.ErrorHandler())

	// 添加全局限流中间件
	r.Use(middleware.RateLimitMiddleware())

	// 开发模式：禁用静态文件缓存
	r.Use(middleware.NoCache())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080", "http://127.0.0.1:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.Static("/static", "./web/frontend/static")

	r.GET("/", func(c *gin.Context) {
		c.File("./web/frontend/index.html")
	})

	// 健康检查端点
	r.GET("/health", handlers.HealthCheck)
	r.GET("/health/ready", handlers.ReadinessCheck)
	r.GET("/health/live", handlers.LivenessCheck)
	r.GET("/health/rate-limit", handlers.RateLimitStatusCheck)
	r.GET("/health/metrics", handlers.Metrics)

	api := r.Group("/api")
	{
		// 上传文件 - 严格限流
		api.POST("/files/upload", middleware.UploadRateLimit(), handlers.UploadFile)
		
		// 文件列表和详情 - 普通限流
		api.GET("/files", middleware.APIRateLimit(), handlers.ListFiles)
		api.GET("/files/:md5", middleware.APIRateLimit(), handlers.GetFile)
		api.GET("/files/:md5/content", middleware.APIRateLimit(), handlers.GetFileContent)
		api.GET("/files/:md5/download", middleware.APIRateLimit(), handlers.DownloadFile)
		
		// 删除文件 - 严格限流
		api.DELETE("/files/:md5", middleware.StrictRateLimit(), handlers.DeleteFile)
		
		// 恢复文件 - 普通限流
		api.POST("/files/:md5/resume", middleware.APIRateLimit(), handlers.ResumeFile)
		api.PUT("/files/:md5/rules", middleware.APIRateLimit(), handlers.UpdateFileRules)

		// 处理相关 - 普通限流
		api.POST("/files/:md5/run", middleware.APIRateLimit(), handlers.RunAllSteps)
		api.GET("/files/:md5/status", middleware.APIRateLimit(), handlers.GetFileStatus)
		api.GET("/files/:md5/review-items", middleware.APIRateLimit(), handlers.GetReviewItems)
		
		// 审核操作 - 普通限流
		api.POST("/files/:md5/approve", middleware.APIRateLimit(), handlers.ApproveReviewItem)
		api.POST("/files/:md5/reject", middleware.APIRateLimit(), handlers.RejectReviewItem)
		api.POST("/files/:md5/edit", middleware.APIRateLimit(), handlers.EditReviewItem)
		api.POST("/files/:md5/restore", middleware.APIRateLimit(), handlers.RestoreReviewItem)
		api.POST("/files/:md5/batch-approve", middleware.APIRateLimit(), handlers.BatchApproveReviewItems)
		api.POST("/files/:md5/batch-reject", middleware.APIRateLimit(), handlers.BatchRejectReviewItems)
		api.POST("/files/:md5/finalize", middleware.APIRateLimit(), handlers.FinalizeFile)
		api.GET("/files/:md5/report", middleware.APIRateLimit(), handlers.GetProcessingReport)

		// 规则管理 - 普通限流
		api.GET("/rules", middleware.APIRateLimit(), handlers.ListRules)
		api.POST("/rules", middleware.StrictRateLimit(), handlers.AddRule)
		api.DELETE("/rules/:id", middleware.StrictRateLimit(), handlers.DeleteRule)
	}

	return r
}

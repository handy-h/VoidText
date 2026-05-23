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

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080", "http://127.0.0.1:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-API-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.Use(middleware.AuthMiddleware())

	r.Static("/static", "./web/frontend/static")

	r.GET("/", func(c *gin.Context) {
		c.File("./web/frontend/index.html")
	})

	api := r.Group("/api")
	{
		api.POST("/files/upload", handlers.UploadFile)
		api.GET("/files", handlers.ListFiles)
		api.GET("/files/:md5", handlers.GetFile)
		api.GET("/files/:md5/content", handlers.GetFileContent)
		api.GET("/files/:md5/download", handlers.DownloadFile)
		api.DELETE("/files/:md5", handlers.DeleteFile)
		api.POST("/files/:md5/resume", handlers.ResumeFile)
		api.PUT("/files/:md5/rules", handlers.UpdateFileRules)

		api.POST("/files/:md5/run", handlers.RunAllSteps)
		api.POST("/files/:md5/cancel", handlers.CancelProcessing)
		api.GET("/files/:md5/status", handlers.GetFileStatus)
		api.GET("/files/:md5/review-items", handlers.GetReviewItems)
		api.POST("/files/:md5/approve", handlers.ApproveReviewItem)
		api.POST("/files/:md5/reject", handlers.RejectReviewItem)
		api.POST("/files/:md5/edit", handlers.EditReviewItem)
		api.POST("/files/:md5/restore", handlers.RestoreReviewItem)
		api.POST("/files/:md5/batch-approve", handlers.BatchApproveReviewItems)
		api.POST("/files/:md5/batch-reject", handlers.BatchRejectReviewItems)
		api.POST("/files/:md5/finalize", handlers.FinalizeFile)
		api.GET("/files/:md5/report", handlers.GetProcessingReport)

		api.GET("/rules", handlers.ListRules)
		api.POST("/rules", handlers.AddRule)
		api.DELETE("/rules/:id", handlers.DeleteRule)
	}

	return r
}

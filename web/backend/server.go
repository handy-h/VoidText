package backend

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"txt-cleaning/web/backend/handlers"
)

// NewServer 创建新的Web服务器
func NewServer() *gin.Engine {
	// 创建Gin引擎
	r := gin.Default()

	// 配置CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 静态文件服务
	r.Static("/static", "./web/frontend/static")
	
	// 主页面
	r.GET("/", func(c *gin.Context) {
		c.File("./web/frontend/index.html")
	})

	// API路由
	api := r.Group("/api")
	{
		// 文件上传和管理
		api.POST("/files/upload", handlers.UploadFile)
		api.GET("/files", handlers.ListFiles)
		api.GET("/files/:id", handlers.GetFile)
		api.DELETE("/files/:id", handlers.DeleteFile)

		// 文本处理
		api.POST("/process", handlers.StartProcessing)
		api.GET("/process/:id/status", handlers.GetProcessStatus)
		api.GET("/process/:id/suggestions", handlers.GetSuggestions)
		api.POST("/process/:id/approve", handlers.ApproveSuggestion)
		api.POST("/process/:id/reject", handlers.RejectSuggestion)
		api.POST("/process/:id/save", handlers.SaveProgress)

		// 版本管理
		api.GET("/files/:id/versions", handlers.ListVersions)
		api.GET("/files/:id/versions/:version", handlers.GetVersion)
		api.POST("/files/:id/versions/:version/restore", handlers.RestoreVersion)
		api.DELETE("/files/:id/versions/:version", handlers.DeleteVersion)

		// 自定义规则
		api.GET("/rules", handlers.ListRules)
		api.POST("/rules", handlers.AddRule)
		api.DELETE("/rules/:id", handlers.DeleteRule)

		// 外部API配置
		api.GET("/config/external", handlers.GetExternalConfig)
		api.PUT("/config/external", handlers.UpdateExternalConfig)
	}

	return r
}
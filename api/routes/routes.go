package routes

import (
    "github.com/gin-gonic/gin"
    "github.com/feichai0017/document-processor/api/handlers"
    "github.com/feichai0017/document-processor/api/middleware"
)

// SetupRoutes 配置所有路由
func SetupRoutes(r *gin.Engine, h *handlers.Handlers) {
    // 全局中间件
    r.Use(middleware.CORS())

    // API 版本组
    v1 := r.Group("/api/v1")
    
    // API 特定中间件
    v1.Use(middleware.CORS())
    // v1.Use(middleware.Auth())
    // v1.Use(middleware.RequestValidator())

    // 健康检查
    // v1.GET("/health", handlers.HealthCheck)

    // 文档处理路由组
    docs := v1.Group("/documents")
    {
        docs.POST("/process", h.Document.ProcessDocument)
        docs.POST("/batch", h.Document.ProcessBatch)
        docs.GET("/status/:taskId", h.Document.GetStatus)
        docs.GET("/download/:taskId", h.Document.DownloadResult)
        docs.DELETE("/task/:taskId", h.Document.CancelTask)
    }

}
package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/feichai0017/document-processor/api/handlers"
    "github.com/feichai0017/document-processor/api/routes"
    "github.com/feichai0017/document-processor/internal/service/document"
    "github.com/feichai0017/document-processor/pkg/logger"
)

func main() {
    // 初始化日志
    log, err := logger.NewLogger(
        logger.WithLevel("info"),
        logger.WithEncoding("json"),
        logger.WithOutputPaths([]string{"stdout", "logs/app.log"}),
    )
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    // 初始化文档服务
    docService, err := document.GetService(log)
    if err != nil {
        log.Fatal("Failed to get document service:", logger.Error(err))
    }

    // 初始化 HTTP 服务
    h := handlers.NewHandlers(docService, log)
    r := gin.New()
    r.Use(gin.Recovery())
    routes.SetupRoutes(r, h)

    srv := &http.Server{
        Addr:    ":8080",
        Handler: r,
    }

    // 启动 HTTP 服务器
    go func() {
        log.Info("Server starting on port 8080")
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error("Server error:", logger.Error(err))
        }
    }()

    // 等待退出信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // 优雅关闭
    log.Info("Shutting down server...")
    
    // 先关闭 HTTP 服务器
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer shutdownCancel()
    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Error("Server forced to shutdown:", logger.Error(err))
    }

}
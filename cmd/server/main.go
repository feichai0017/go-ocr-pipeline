package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feichai0017/document-processor/api/handlers"
	"github.com/feichai0017/document-processor/api/routes"
	"github.com/feichai0017/document-processor/internal/service/document"
	"github.com/feichai0017/document-processor/pkg/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	// init logger
	log, err := logger.NewLogger(
		logger.WithLevel("info"),
		logger.WithEncoding("json"),
		logger.WithOutputPaths([]string{"stdout", "logs/app.log"}),
	)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// init document service
	docService, err := document.GetService(log)
	if err != nil {
		log.Fatal("Failed to get document service:", logger.Error(err))
	}

	// init handlers
	h := handlers.NewHandlers(docService, log)
	r := gin.New()
	r.Use(gin.Recovery())
	routes.SetupRoutes(r, h)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// start server
	go func() {
		log.Info("Server starting on port 8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server error:", logger.Error(err))
		}
	}()

	// wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown:", logger.Error(err))
	}

}

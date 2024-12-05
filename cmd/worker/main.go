package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/feichai0017/document-processor/internal/service/document"
	"github.com/feichai0017/document-processor/pkg/logger"
	"github.com/feichai0017/document-processor/pkg/worker"
)

func main() {

	// init logger
	log, err := logger.NewLogger(
		logger.WithLevel("info"),
		logger.WithEncoding("json"),
		logger.WithOutputPaths([]string{"stdout", "logs/worker.log"}),
	)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// create document service
	docService, err := document.GetService(log)
	if err != nil {
		log.Error("Failed to create document service", logger.Error(err))
		os.Exit(1)
	}

	// create worker config
	workerCfg := &worker.Config{
		RedisAddr:   "localhost:6379",
		RedisDB:     0,
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	}

	// init worker
	documentWorker, err := worker.NewDocumentWorker(workerCfg, docService, log)
	if err != nil {
		log.Error("Failed to create document worker", logger.Error(err))
		os.Exit(1)
	}

	// create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 worker
	if err := documentWorker.Start(ctx); err != nil {
		log.Error("Failed to start worker", logger.Error(err))
		os.Exit(1)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 优雅关闭
	log.Info("Shutting down worker...")
	documentWorker.Stop()
	log.Info("Worker stopped")
}

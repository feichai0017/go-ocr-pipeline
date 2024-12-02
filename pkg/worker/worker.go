package worker

import (
    "context"
    "github.com/hibiken/asynq"
    "github.com/feichai0017/document-processor/pkg/logger"
)

type Worker interface {
    Start(ctx context.Context) error
    Stop() error
}

type Config struct {
    RedisAddr   string
    RedisDB     int
    Concurrency int
    Queues      map[string]int
}

type BaseWorker struct {
    server   *asynq.Server
    mux      *asynq.ServeMux
    logger   logger.Logger
    stopChan chan struct{}
}

func (w *BaseWorker) Stop() error {
    close(w.stopChan)
    w.server.Stop()
    return nil
}
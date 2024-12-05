package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/feichai0017/document-processor/internal/service/document"
	"github.com/feichai0017/document-processor/pkg/logger"
	"github.com/feichai0017/document-processor/pkg/queue"
	"github.com/hibiken/asynq"
	"time"
)

type DocumentWorker struct {
	BaseWorker
	docService document.DocumentProcessor
}

func NewDocumentWorker(cfg *Config, docService document.DocumentProcessor, logger logger.Logger) (*DocumentWorker, error) {
	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: cfg.RedisDB},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues:      cfg.Queues,
			RetryDelayFunc: func(n int, err error, task *asynq.Task) time.Duration {
				return time.Duration(n) * time.Minute
			},
		},
	)

	w := &DocumentWorker{
		BaseWorker: BaseWorker{
			server:   server,
			mux:      asynq.NewServeMux(),
			logger:   logger,
			stopChan: make(chan struct{}),
		},
		docService: docService,
	}

	// 注册任务处理器
	w.registerHandlers()
	return w, nil
}

func (w *DocumentWorker) registerHandlers() {
	w.mux.HandleFunc(queue.TaskTypeDocumentProcess, w.handleDocumentProcess)
}

func (w *DocumentWorker) handleDocumentProcess(ctx context.Context, t *asynq.Task) error {
	// 添加原始任务日志
	w.logger.Info("Received task",
		logger.String("payload", string(t.Payload())),
	)

	// 反序列化任务
	var task queue.Task
	if err := json.Unmarshal(t.Payload(), &task); err != nil {
		w.logger.Error("Failed to unmarshal task",
			logger.Error(err),
			logger.String("payload", string(t.Payload())),
		)
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	// 添加详细日志
	w.logger.Info("Processing document task",
		logger.String("taskId", task.ID),
		logger.Any("metadata", task.Metadata),
		logger.Any("payload", task.Payload),
	)

	// 检查必要字段
	if task.ID == "" || task.Metadata == nil || task.Payload == nil {
		w.logger.Error("Invalid task data",
			logger.String("taskId", task.ID),
			logger.Any("metadata", task.Metadata),
			logger.Any("payload", task.Payload),
		)
		return fmt.Errorf("invalid task data: missing required fields")
	}

	// 获取任务写入器
	info := t.ResultWriter()

	// 写入任务开始状态
	if _, err := info.Write([]byte(`{"status":"running","progress":0}`)); err != nil {
		w.logger.Error("Failed to write task status", logger.Error(err))
	}

	err := w.docService.HandleDocument(ctx, &task)
	if err != nil {
		// 写入失败状态
		if _, writeErr := info.Write([]byte(fmt.Sprintf(`{"status":"failed","error":%q}`, err.Error()))); writeErr != nil {
			w.logger.Error("Failed to write task failure", logger.Error(writeErr))
		}
		return err
	}

	// 写入完成状态
	if _, err := info.Write([]byte(`{"status":"completed","progress":100}`)); err != nil {
		w.logger.Error("Failed to write task completion", logger.Error(err))
	}

	return nil
}

func (w *DocumentWorker) Start(ctx context.Context) error {
	go func() {
		if err := w.server.Run(w.mux); err != nil {
			w.logger.Error("Worker server stopped", logger.Error(err))
		}
	}()

	go func() {
		<-ctx.Done()
		w.Stop()
	}()

	return nil
}

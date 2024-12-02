package document

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/feichai0017/document-processor/internal/agent"
	"github.com/feichai0017/document-processor/internal/models"
	"github.com/feichai0017/document-processor/pkg/logger"
	"github.com/feichai0017/document-processor/pkg/queue"
	"github.com/feichai0017/document-processor/pkg/storage"
	"github.com/feichai0017/document-processor/pkg/converters"
)

type DocumentService struct {
	processorFactory agent.ProcessorFactory
	queue            queue.Queue
	storage          storage.Storage
	logger           logger.Logger
	config           *ServiceConfig
}

type ServiceConfig struct {
	MaxFileSize       int64
	AllowedTypes      []string
	StoragePath       string
	QueuePriority     int
	MaxConcurrent     int
	ProcessTimeout    time.Duration
	RetentionPeriod   time.Duration
}

func NewService(
	factory agent.ProcessorFactory,
	queue queue.Queue,
	storage storage.Storage,
	logger logger.Logger,
	cfg *ServiceConfig,
) DocumentProcessor {
	if cfg == nil {
		cfg = &ServiceConfig{
			MaxFileSize:     50 * 1024 * 1024, // 50MB
			AllowedTypes:    []string{".pdf", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".tiff"},
			MaxConcurrent:   5,
			ProcessTimeout:  30 * time.Minute,
			RetentionPeriod: 24 * time.Hour,
		}
	}

	return &DocumentService{
		processorFactory: factory,
		queue:           queue,
		storage:         storage,
		logger:          logger,
		config:          cfg,
	}
}

func GetService(log logger.Logger) (DocumentProcessor, error) {
	// 初始化存储(S3)
	store, err := storage.NewStorage(storage.StorageTypeS3, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// 初始化队列
	q, err := queue.GetQueue()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize queue: %w", err)
	}

	// 初始化处理器工厂
	factory, err := agent.NewProcessorFactory(log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize processor factory: %w", err)
	}

	// 默认配置
	cfg := &ServiceConfig{
		MaxFileSize:      50 * 1024 * 1024, // 50MB
		AllowedTypes:     []string{".pdf", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".tiff"},
		MaxConcurrent:    5,
		ProcessTimeout:   30 * time.Minute,
		RetentionPeriod:  24 * time.Hour,
	}

	return NewService(factory, q, store, log, cfg), nil
}

// ProcessFile 处理单个文件
func (s *DocumentService) ProcessFile(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
) (*models.ProcessingTask, error) {
	s.logger.Info("Starting file processing",
		logger.String("filename", header.Filename),
		logger.Int64("size", header.Size),
	)

	// 验证文件
	if err := s.validateFile(header); err != nil {
		s.logger.Error("File validation failed",
			logger.String("filename", header.Filename),
			logger.Error(err),
		)
		return nil, err
	}

	// 成任务ID
	taskID := uuid.New().String()

	// 创建处理任务
	task := &models.ProcessingTask{
		ID:        taskID,
		Status:    models.StatusPending,
		Type:      "document:process",
		Priority:  s.config.QueuePriority,
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata: map[string]string{
			"filename": header.Filename,
			"size":    fmt.Sprintf("%d", header.Size),
			"type":    filepath.Ext(header.Filename),
		},
	}

	// 存储文件
	fileID, err := s.storage.Store(ctx, file, header.Filename)
	if err != nil {
		s.logger.Error("Failed to store file",
			logger.String("filename", header.Filename),
			logger.Error(err),
		)
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	// 准备任务数据
	queueTask := &queue.Task{
		ID:       taskID,
		Type:     task.Type,
		Priority: task.Priority,
		Payload: map[string]interface{}{
			"fileId":   fileID,
			"filename": header.Filename,
			"size":     header.Size,
			"type":     filepath.Ext(header.Filename),
		},
		Metadata:  task.Metadata,
		CreatedAt: task.CreatedAt,
	}

	// 加入处理队列
	if err := s.queue.Enqueue(ctx, queueTask); err != nil {
		s.logger.Error("Failed to enqueue task",
			logger.String("taskId", taskID),
			logger.Error(err),
		)
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	// 保存初始状态
	initialStatus := &queue.TaskStatus{
		TaskID:    taskID,
		Status:    "pending",
		Progress:  0,
		StartedAt: time.Now(),
	}

	if err := s.queue.SaveFinalStatus(ctx, initialStatus); err != nil {
		s.logger.Error("Failed to save initial status",
			logger.String("taskId", taskID),
			logger.Error(err),
		)
	}

	s.logger.Info("File processing task created",
		logger.String("taskId", taskID),
		logger.String("filename", header.Filename),
	)

	return task, nil
}

// ProcessBatch 批量处理文件
func (s *DocumentService) ProcessBatch(ctx context.Context, files []*multipart.FileHeader) ([]*models.ProcessingTask, error) {
	tasks := make([]*models.ProcessingTask, 0, len(files))
	var mu sync.Mutex
	
	// 使用 errgroup 来管理并发和错误
	g, ctx := errgroup.WithContext(ctx)
	
	for _, header := range files {
		header := header // 创建副本用于闭包
		g.Go(func() error {
			file, err := header.Open()
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", header.Filename, err)
			}
			defer file.Close()
			
			task, err := s.ProcessFile(ctx, file, header)
			if err != nil {
				return fmt.Errorf("failed to process file %s: %w", header.Filename, err)
			}
			
			mu.Lock()
			tasks = append(tasks, task)
			mu.Unlock()
			
			return nil
		})
	}
	
	if err := g.Wait(); err != nil {
		return tasks, err // 返回已处理的任务和错误
	}
	
	return tasks, nil
}

// HandleDocument 实现文档处理逻辑
func (s *DocumentService) HandleDocument(ctx context.Context, task *queue.Task) error {
	if task == nil || task.Payload == nil || task.Metadata == nil {
        return fmt.Errorf("invalid task: missing required data")
    }
	
	s.logger.Info("Processing document",
		logger.String("taskId", task.ID),
		logger.String("filename", task.Metadata["filename"]),
	)

	// 获取文件
	fileID := task.Payload["fileId"].(string)
	reader, err := s.storage.Get(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	// 获取处理器
	processor, err := s.processorFactory.GetProcessor(task.Metadata["type"])
	if err != nil {
		return fmt.Errorf("failed to get processor: %w", err)
	}

	// 处理文档
	chunks, err := processor.Process(ctx, reader)
	if err != nil {
		return fmt.Errorf("failed to process document: %w", err)
	}

	// 使用 JSONConverter 转换结果
	converter := converters.NewJSONConverter()
	processedDoc, err := converter.Convert(chunks)
	if err != nil {
		return fmt.Errorf("failed to convert document: %w", err)
	}

	// 更新处理结果
	processedDoc.TaskID = task.ID
	processedDoc.ProcessedAt = time.Now()
	processedDoc.Metadata.FileName = task.Metadata["filename"]
	processedDoc.Metadata.FileType = task.Metadata["type"]
	if size, err := strconv.ParseInt(task.Metadata["size"], 10, 64); err == nil {
		processedDoc.Metadata.FileSize = size
	}

	// 序列化并存储结果
	resultData, err := json.Marshal(processedDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	resultReader := bytes.NewReader(resultData)
	if _, err := s.storage.Store(ctx, resultReader, fmt.Sprintf("result:%s", task.ID)); err != nil {
		return fmt.Errorf("failed to store result: %w", err)
	}

	s.logger.Info("Document processing completed",
		logger.String("taskId", task.ID),
		logger.Int("chunkCount", len(chunks)),
	)

	// 在处理完成后，将最终状态保存到 Redis
	finalStatus := &queue.TaskStatus{
		TaskID:     task.ID,
		Status:     "completed",
		Progress:   1.0,
		Error:      "",
		StartedAt:  task.CreatedAt,
		FinishedAt: time.Now(),
	}
	
	if err := s.queue.SaveFinalStatus(ctx, finalStatus); err != nil {
		s.logger.Error("Failed to save final status",
			logger.String("taskId", task.ID),
			logger.Error(err),
		)
	}

	return nil
}

// GetProcessingStatus 获取处理状态
func (s *DocumentService) GetProcessingStatus(ctx context.Context, taskID string) (*models.ProcessingTask, error) {
    status, err := s.queue.GetTaskStatus(ctx, taskID)
    if err != nil {
        return nil, fmt.Errorf("failed to get task status: %w", err)
    }

    // 确保状态正确映射
    var taskStatus models.ProcessingStatus
    switch status.Status {
    case "pending":
        taskStatus = models.StatusPending
    case "active":
        taskStatus = models.StatusRunning
    case "completed":
        taskStatus = models.StatusCompleted
    case "failed":
        taskStatus = models.StatusFailed
    default:
        taskStatus = models.StatusPending
    }

    // 确保返回完整的任务信息
    return &models.ProcessingTask{
        ID:        status.TaskID,
        Status:    taskStatus,
        Type:      "document:process",
        Priority:  0, // 可以从队列配置中获取
        Progress:  status.Progress,
        Error:     status.Error,
        Metadata:  make(map[string]string), // 初始化一个空的 metadata map
        CreatedAt: status.StartedAt,
        UpdatedAt: status.FinishedAt,
    }, nil
}

// GetProcessingResult 获取处理结果
func (s *DocumentService) GetProcessedDocument(ctx context.Context, taskID string) (*converters.ProcessedDocument, error) {
	// 检查任务状态
	status, err := s.GetProcessingStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if status.Status != models.StatusCompleted {
		return nil, fmt.Errorf("task is not completed: %s", status.Status)
	}

	// 获取结果
	reader, err := s.storage.Get(ctx, fmt.Sprintf("result:%s", taskID))
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	var result converters.ProcessedDocument
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode result: %w", err)
	}

	return &result, nil
}

// CancelTask 取消任务
func (s *DocumentService) CancelTask(ctx context.Context, taskID string) error {
	if err := s.queue.CancelTask(ctx, taskID); err != nil {
		return fmt.Errorf("failed to cancel task: %w", err)
	}

	s.logger.Info("Task cancelled",
		logger.String("taskId", taskID),
	)

	return nil
}

// CleanupTasks 清理过期任务
func (s *DocumentService) CleanupTasks(ctx context.Context) error {
	threshold := time.Now().Add(-s.config.RetentionPeriod)
	
	if err := s.storage.CleanupBefore(ctx, threshold); err != nil {
		return fmt.Errorf("failed to cleanup storage: %w", err)
	}

	s.logger.Info("Completed tasks cleanup",
		logger.Time("threshold", threshold),
	)

	return nil
}

// validateFile 验证文件
func (s *DocumentService) validateFile(header *multipart.FileHeader) error {
	// 检查文件大小
	if header.Size > s.config.MaxFileSize {
		return fmt.Errorf("file size exceeds maximum limit of %d bytes", s.config.MaxFileSize)
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	validType := false
	for _, t := range s.config.AllowedTypes {
		if t == ext {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	return nil
}



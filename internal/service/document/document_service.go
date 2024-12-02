package document

import (
    "context"
    "mime/multipart"
    "github.com/feichai0017/document-processor/internal/models"
    "github.com/feichai0017/document-processor/pkg/converters"
    "github.com/feichai0017/document-processor/pkg/queue"
)

type DocumentProcessor interface {
    ProcessFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*models.ProcessingTask, error)
    ProcessBatch(ctx context.Context, files []*multipart.FileHeader) ([]*models.ProcessingTask, error)
    GetProcessingStatus(ctx context.Context, taskID string) (*models.ProcessingTask, error)
    HandleDocument(ctx context.Context, task *queue.Task) error
    GetProcessedDocument(ctx context.Context, taskID string) (*converters.ProcessedDocument, error)
    CancelTask(ctx context.Context, taskID string) error
}
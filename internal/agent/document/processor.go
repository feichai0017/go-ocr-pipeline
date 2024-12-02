package document

import (
    "context"
    "io"
    
    "github.com/feichai0017/document-processor/internal/models"
)

// Processor 文档处理器接口
type Processor interface {
    // CanProcess 检查是否可以处理指定MIME类型的文件
    CanProcess(mimeType string) bool
    
    // Process 处理文档并返回文档块
    Process(ctx context.Context, reader io.Reader) ([]models.DocumentChunk, error)
    
    // ExtractMetadata 提取文档元数据
    ExtractMetadata(ctx context.Context, reader io.Reader) (models.DocumentMetadata, error)
    
    // Close 清理资源
    Close() error
}
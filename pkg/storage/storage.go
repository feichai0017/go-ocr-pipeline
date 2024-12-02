package storage

import (
    "context"
    "fmt"
    "io"
    "time"

    "github.com/feichai0017/document-processor/pkg/logger"
    "github.com/feichai0017/document-processor/pkg/storage/s3"
    "github.com/feichai0017/document-processor/pkg/storage/minio"
)

// StorageType 定义存储类型
type StorageType string

const (
    StorageTypeS3    StorageType = "s3"
    StorageTypeMinio StorageType = "minio"
)

// Storage 接口定义
type Storage interface {
    // Store 存储文件
    Store(ctx context.Context, reader io.Reader, filename string) (string, error)
    // Get 获取文件
    Get(ctx context.Context, fileID string) (io.ReadCloser, error)
    // Delete 删除文件
    Delete(ctx context.Context, id string) error
    // CleanupBefore 清理过期文件
    CleanupBefore(ctx context.Context, threshold time.Time) error
}


// NewStorage 创建存储实例的工厂方法
func NewStorage(storageType StorageType, logger logger.Logger) (Storage, error) {
    switch storageType {
    case StorageTypeS3:
        return s3.GetClient(logger)
    case StorageTypeMinio:
        return minio.GetClient(logger)
    default:
        return nil, fmt.Errorf("unsupported storage type: %s", storageType)
    }
}



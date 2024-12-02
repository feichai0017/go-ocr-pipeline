package minio

import (
    "context"
    "fmt"
    "io"
    "time"
    
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    
    cfg "github.com/feichai0017/document-processor/config"
    "github.com/feichai0017/document-processor/pkg/logger"
)

type MinioStorage struct {
    client     *minio.Client
    bucketName string
    logger     logger.Logger
}

// Store implements Storage.Store
func (m *MinioStorage) Store(ctx context.Context, reader io.Reader, filename string) (string, error) {
    _, err := m.client.PutObject(ctx, m.bucketName, filename, reader, -1, minio.PutObjectOptions{})
    if err != nil {
        m.logger.Error("Failed to store file to MinIO",
            logger.String("bucket", m.bucketName),
            logger.String("filename", filename),
            logger.Error(err),
        )
        return "", fmt.Errorf("failed to store file: %w", err)
    }

    return filename, nil
}

// Get implements Storage.Get
func (m *MinioStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
    obj, err := m.client.GetObject(ctx, m.bucketName, key, minio.GetObjectOptions{})
    if err != nil {
        m.logger.Error("Failed to get file from MinIO",
            logger.String("bucket", m.bucketName),
            logger.String("key", key),
            logger.Error(err),
        )
        return nil, fmt.Errorf("failed to get file: %w", err)
    }

    return obj, nil
}

// Delete implements Storage.Delete
func (m *MinioStorage) Delete(ctx context.Context, key string) error {
    err := m.client.RemoveObject(ctx, m.bucketName, key, minio.RemoveObjectOptions{})
    if err != nil {
        m.logger.Error("Failed to delete file from MinIO",
            logger.String("bucket", m.bucketName),
            logger.String("key", key),
            logger.Error(err),
        )
        return fmt.Errorf("failed to delete file: %w", err)
    }

    return nil
}

// CleanupBefore implements Storage.CleanupBefore
func (m *MinioStorage) CleanupBefore(ctx context.Context, threshold time.Time) error {
    objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{})
    
    for obj := range objectCh {
        if obj.Err != nil {
            m.logger.Error("Error listing objects",
                logger.String("bucket", m.bucketName),
                logger.Error(obj.Err),
            )
            continue
        }

        if obj.LastModified.Before(threshold) {
            if err := m.Delete(ctx, obj.Key); err != nil {
                m.logger.Error("Failed to delete expired object",
                    logger.String("key", obj.Key),
                    logger.Error(err),
                )
                continue
            }
            m.logger.Info("Deleted expired object",
                logger.String("key", obj.Key),
                logger.Time("lastModified", obj.LastModified),
            )
        }
    }

    return nil
}

func NewMinioStorage(logger logger.Logger) (*MinioStorage, error) {
    minioConfig := cfg.GetMinioConfig()
    client, err := minio.New(minioConfig.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
        Secure: minioConfig.UseSSL,
        Region: minioConfig.Region,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create MinIO client: %w", err)
    }

    exists, err := client.BucketExists(context.Background(), minioConfig.BucketName)
    if err != nil {
        return nil, fmt.Errorf("failed to check bucket existence: %w", err)
    }

    if !exists {
        err = client.MakeBucket(context.Background(), minioConfig.BucketName, minio.MakeBucketOptions{
            Region: minioConfig.Region,
        })
        if err != nil {
            return nil, fmt.Errorf("failed to create bucket: %w", err)
        }
    }

    return &MinioStorage{
        client:     client,
        bucketName: minioConfig.BucketName,
        logger:     logger,
    }, nil
}

func GetClient(logger logger.Logger) (*MinioStorage, error) {
    return NewMinioStorage(logger)
}

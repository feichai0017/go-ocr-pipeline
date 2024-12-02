package s3

import (
    "context"
    "fmt"
    "io"
    "time"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    
    cfg "github.com/feichai0017/document-processor/config"
    "github.com/feichai0017/document-processor/pkg/logger"
)

type S3Storage struct {
    client     *s3.Client
    bucketName string
    region     string
    logger     logger.Logger
}

// Store 实现 Storage 接口的 Store 方法
func (s *S3Storage) Store(ctx context.Context, reader io.Reader, key string) (string, error) {
    input := &s3.PutObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
        Body:   reader,
    }

    _, err := s.client.PutObject(ctx, input)
    if err != nil {
        s.logger.Error("Failed to store file to S3",
            logger.String("bucket", s.bucketName),
            logger.String("key", key),
            logger.Error(err),
        )
        return "", fmt.Errorf("failed to store file: %w", err)
    }

    return key, nil
}

// Get 实现 Storage 接口的 Get 方法
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
    input := &s3.GetObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    }

    result, err := s.client.GetObject(ctx, input)
    if err != nil {
        s.logger.Error("Failed to get file from S3",
            logger.String("bucket", s.bucketName),
            logger.String("key", key),
            logger.Error(err),
        )
        return nil, fmt.Errorf("failed to get file: %w", err)
    }

    return result.Body, nil
}

// Delete 实现 Storage 接口的 Delete 方法
func (s *S3Storage) Delete(ctx context.Context, key string) error {
    input := &s3.DeleteObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    }

    _, err := s.client.DeleteObject(ctx, input)
    if err != nil {
        s.logger.Error("Failed to delete file from S3",
            logger.String("bucket", s.bucketName),
            logger.String("key", key),
            logger.Error(err),
        )
        return fmt.Errorf("failed to delete file: %w", err)
    }

    return nil
}

// CleanupBefore 实现 Storage 接口的 CleanupBefore 方法
func (s *S3Storage) CleanupBefore(ctx context.Context, threshold time.Time) error {
    input := &s3.ListObjectsV2Input{
        Bucket: aws.String(s.bucketName),
    }

    paginator := s3.NewListObjectsV2Paginator(s.client, input)
    for paginator.HasMorePages() {
        page, err := paginator.NextPage(ctx)
        if err != nil {
            s.logger.Error("Failed to list objects",
                logger.String("bucket", s.bucketName),
                logger.Error(err),
            )
            return fmt.Errorf("failed to list objects: %w", err)
        }

        for _, obj := range page.Contents {
            if obj.LastModified.Before(threshold) {
                if err := s.Delete(ctx, *obj.Key); err != nil {
                    s.logger.Error("Failed to delete expired object",
                        logger.String("key", *obj.Key),
                        logger.Error(err),
                    )
                    continue
                }
                s.logger.Info("Deleted expired object",
                    logger.String("key", *obj.Key),
                    logger.Time("lastModified", *obj.LastModified),
                )
            }
        }
    }

    return nil
}

func NewS3Storage(log logger.Logger) (*S3Storage, error) {
    s3Config := cfg.GetS3Config()
    
    log.Info("S3 Configuration",
        logger.String("bucket", s3Config.BucketName),
        logger.String("region", s3Config.Region),
        logger.String("access_key", s3Config.AccessKey),
        logger.String("endpoint", s3Config.Endpoint),
    )

    // AWS SDK 配置
    awsCfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion(s3Config.Region),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            s3Config.AccessKey,
            s3Config.SecretKey,
            "",
        )),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    client := s3.NewFromConfig(awsCfg)
    
    // 验证 bucket 是否存在
    _, err = client.HeadBucket(context.Background(), &s3.HeadBucketInput{
        Bucket: aws.String(s3Config.BucketName),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to verify bucket existence: %w", err)
    }

    return &S3Storage{
        client:     client,
        bucketName: s3Config.BucketName,
        region:     s3Config.Region,
        logger:     log,
    }, nil
}

func GetClient(logger logger.Logger) (*S3Storage, error) {
    return NewS3Storage(logger)
}


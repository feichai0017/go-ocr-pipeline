package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	minioOnce sync.Once
	minioConfig *MinioConfig
)

type MinioConfig struct {
	AccessKey   string
	SecretKey   string
	Endpoint    string
	UseSSL      bool
	Region      string
	BucketName  string
}

func GetMinioConfig() *MinioConfig {
	minioOnce.Do(func() {
		// 获取当前文件的目录
		_, filename, _, _ := runtime.Caller(0)
		configDir := filepath.Dir(filename)
		
		// 构建到项目根目录的路径
		rootDir := filepath.Dir(configDir)
		envPath := filepath.Join(rootDir, ".env")
		
		// 加载 .env 文件
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Warning: .env file not found at %s, falling back to environment variables", envPath)
		}

		minioConfig = &MinioConfig{
			AccessKey:   os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey:   os.Getenv("MINIO_SECRET_KEY"),
			Endpoint:    os.Getenv("MINIO_ENDPOINT"),
			UseSSL:      false,
			Region:      os.Getenv("MINIO_REGION"),
			BucketName:  os.Getenv("MINIO_BUCKET_NAME"),
		}
	})
	return minioConfig
}

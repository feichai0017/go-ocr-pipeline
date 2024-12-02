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
	s3Once sync.Once
	s3Config *S3Config
)

type S3Config struct {
	BucketName string
	Region     string
	Endpoint   string
	AccessKey  string
	SecretKey  string
}

func GetS3Config() *S3Config {
	s3Once.Do(func() {
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

		s3Config = &S3Config{
			BucketName: os.Getenv("AWS_S3_BUCKET_NAME"),
			Region:     os.Getenv("AWS_REGION"),
			Endpoint:   os.Getenv("AWS_ENDPOINT"),
			AccessKey:  os.Getenv("AWS_ACCESS_KEY"),
			SecretKey:  os.Getenv("AWS_SECRET_KEY"),
		}
	})
	return s3Config
}
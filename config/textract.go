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
	textractOnce sync.Once
	textractConfig *TextractConfig
)

type TextractConfig struct {
	BucketName string
	Region     string
	Endpoint   string
	AccessKey  string
	SecretKey  string
}

func GetTextractConfig() *TextractConfig {
	textractOnce.Do(func() {
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

		textractConfig = &TextractConfig{
			Region:     os.Getenv("AWS_REGION"),
			Endpoint:   os.Getenv("AWS_ENDPOINT"),
			AccessKey:  os.Getenv("AWS_ACCESS_KEY"),
			SecretKey:  os.Getenv("AWS_SECRET_KEY"),
		}
	})
	return textractConfig
}

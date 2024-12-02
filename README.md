# Document Processing Service

一个基于 Go 的文档处理服务，支持多种格式文档的处理、OCR 识别和文本分析。

## 技术栈

### 后端框架与工具
- Go 1.23
- Gin (Web 框架)
- Asynq + Redis (任务队列)
- AWS Textract (OCR 服务)
- Tesseract (本地 OCR 备选)

### 文档处理
- Tesseract (OCR 引擎)
- AWS Textract (云端 OCR)
- pdf (PDF 文档处理)
- imaging (llama3.2-vision)

### 存储与缓存
- AWS S3 (对象存储)
- MinIO (对象存储)
- Redis (缓存与队列)

### 日志与监控
- Zap (日志处理)
- Prometheus (可选，监控)

## 系统架构

### 核心模块
1. 文档处理服务 (DocumentService)
   - 支持 PDF、图片等多种文档格式
   - 异步处理队列
   - 文档元数据提取
   - AWS Textract 集成

2. 处理器模块 (Processor)
   - 工厂模式 (ProcessorFactory)
   - PDF 处理器 (PdfProcessor)
   - 图片处理器 (ImageProcessor)
   - Textract 处理器 (TextractProcessor)

3. 预处理模块 (Preprocessor)
   - 图像灰度化
   - 倾斜校正
   - 自适应阈值
   - 降噪处理
   - 边缘检测

4. 队列服务 (QueueService)
   - 基于 Asynq 的任务队列
   - 支持任务优先级
   - 错误重试机制

### API 接口
- POST /api/v1/documents/process - 文档处理
- POST /api/v1/documents/batch - 批量文档处理
- GET /api/v1/documents/download/:taskId - 获取处理结果
- GET /api/v1/documents/status/:taskId - 处理状态查询
- DELETE /api/v1/documents/:taskId - 取消处理

## 特性
- 多格式支持 (PDF、JPEG、PNG、TIFF)
- 双重 OCR 引擎 (AWS Textract + Tesseract)
- 图像预处理优化
- 异步处理机制
- 表格识别
- 文档分块处理
- 错误处理和重试

## Prerequisites

### macOS

1. Install Tesseract and required languages:

```bash
brew install tesseract
brew install tesseract-lang
```

2. Set environment variables for CGO:

```bash
export LIBRARY_PATH="/opt/homebrew/lib"
export CPATH="/opt/homebrew/include"
```

> Note: For Apple Silicon Macs, Homebrew installs packages in `/opt/homebrew`. These environment variables are required for the Go program to find the Tesseract libraries.

# Document Processing Service

一个基于 Node.js 的文档处理服务，支持多种格式文档的处理、OCR 识别和文本分析。

## 技术栈

### 后端框架与工具
- Node.js + TypeScript
- Express.js
- TypeDI (依赖注入)
- BullMQ + Redis (任务队列)
- Multer (文件上传)

### 文档处理
- PDF.js (PDF 文档处理)
- Tesseract.js (OCR 文字识别)
- file-type (文件类型检测)

### 代码质量与类型检查
- TypeScript
- ESLint
- Prettier

## 系统架构

### 核心模块
1. 文档处理服务 (DocumentProcessingService)
   - 支持 PDF、图片等多种文档格式
   - 异步处理队列
   - 文档元数据提取

2. 处理器模块
   - 基础处理器 (BaseDocumentProcessor)
   - PDF 处理器 (PdfProcessor)
   - 图片处理器 (ImageProcessor，支持 OCR)

3. 队列服务 (QueueService)
   - 基于 BullMQ 的任务队列
   - 支持任务优先级
   - 错误重试机制

4. 验证模块 (DocumentValidator)
   - 文件类型验证
   - 大小限制检查
   - 文件完整性验证

### API 接口
- POST /process - 单文件处理
- POST /batch - 批量文件处理
- GET /status/:taskId - 处理状态查询

## 特性
- 支持多种文档格式（PDF、DOCX、图片）
- OCR 文字识别
- 异步处理机制
- 文档分块处理
- 元数据提取
- 错误处理和重试机制
- 类型安全

## 项目结构


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

package agent

import (
    "context"
    "fmt"
    "strings"

    "github.com/aws/aws-sdk-go-v2/service/textract/types"
    "github.com/feichai0017/document-processor/internal/agent/document"
    "github.com/feichai0017/document-processor/internal/agent/document/pdf"
    "github.com/feichai0017/document-processor/internal/agent/document/image"
    "github.com/feichai0017/document-processor/pkg/logger"
    cfg "github.com/feichai0017/document-processor/config"
)

// 添加扩展名到 MIME 类型的映射
var extToMIME = map[string]string{
    ".jpg":  "image/jpeg",
    ".jpeg": "image/jpeg",
    ".png":  "image/png",
    ".tiff": "image/tiff",
    ".pdf":  "application/pdf",
    ".doc":  "application/msword",
    ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

type ProcessorFactory struct {
    processors map[string]document.Processor
    logger     logger.Logger
}

func NewProcessorFactory(logger logger.Logger) (ProcessorFactory, error) {
    factory := &ProcessorFactory{
        processors: make(map[string]document.Processor),
        logger:     logger,
    }

    // 初始化 PDF 处理器
    pdfProcessor := pdf.NewProcessor(logger)
    factory.processors["application/pdf"] = pdfProcessor

    textractCfg := cfg.GetTextractConfig()

    // 初始化 Textract 处理器
    textractConfig := &image.TextractConfig{
        Region:        textractCfg.Region,
        AccessKey:     textractCfg.AccessKey,
        SecretKey:     textractCfg.SecretKey,
        MinConfidence: 80.0,
        EnableTable:   true,
        EnableForm:    true,
        FeatureTypes: []types.FeatureType{
            types.FeatureTypeTables,
            types.FeatureTypeForms,
        },
    }

    textractProcessor, err := image.NewTextractProcessor(context.Background(), textractConfig, logger)
    if err != nil {
        return ProcessorFactory{}, fmt.Errorf("failed to create textract processor: %w", err)
    }

    // 注册 Textract 处理器支持的所有图像类型
    factory.processors["image/jpeg"] = textractProcessor
    factory.processors["image/jpg"] = textractProcessor
    factory.processors["image/png"] = textractProcessor
    factory.processors["image/tiff"] = textractProcessor

    /* 
    imageProcessor, err := image.NewProcessor(logger, nil)
    if err != nil {
        return ProcessorFactory{}, fmt.Errorf("failed to create image processor: %w", err)
    }
    factory.processors["image/jpeg"] = imageProcessor
    factory.processors["image/jpg"] = imageProcessor
    factory.processors["image/png"] = imageProcessor
    factory.processors["image/tiff"] = imageProcessor
    */

    return *factory, nil
}

func (f *ProcessorFactory) GetProcessor(fileType string) (document.Processor, error) {
    // 添加详细日志
    f.logger.Info("Getting processor",
        logger.String("fileType", fileType),
    )

    // 将扩展名转换为 MIME 类型
    mimeType, ok := extToMIME[strings.ToLower(fileType)]
    if !ok {
        f.logger.Error("Unsupported file type",
            logger.String("fileType", fileType),
        )
        return nil, fmt.Errorf("unsupported file type: %s", fileType)
    }

    f.logger.Info("Mapped MIME type",
        logger.String("fileType", fileType),
        logger.String("mimeType", mimeType),
    )

    // 获取处理器
    processor, ok := f.processors[mimeType]
    if !ok {
        f.logger.Error("No processor found",
            logger.String("mimeType", mimeType),
        )
        return nil, fmt.Errorf("no processor found for mime type: %s", mimeType)
    }

    return processor, nil
}
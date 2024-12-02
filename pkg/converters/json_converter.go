package converters

import (
    "fmt"
    "time"
    
    "github.com/feichai0017/document-processor/internal/models"
)

// DocumentConverter 定义文档转换器接口
type DocumentConverter interface {
    Convert(chunks []models.DocumentChunk) (*ProcessedDocument, error)
}

// ProcessedDocument 定义处理后的文档结构
type ProcessedDocument struct {
    TaskID      string                 `json:"taskId"`
    Status      string                 `json:"status"`
    Content     []ChunkContent         `json:"content"`
    Metadata    DocumentMetadata       `json:"metadata"`
    ProcessedAt time.Time             `json:"processedAt"`
}

// ChunkContent 定义文档块内容
type ChunkContent struct {
    Text     string                 `json:"text"`
    Position int                    `json:"position"`
    Type     string                 `json:"type"` // "page", "image", "table" 等
    Metadata map[string]interface{} `json:"metadata"`
}

// DocumentMetadata 定义文档元数据
type DocumentMetadata struct {
    FileName     string   `json:"fileName"`
    FileType     string   `json:"fileType"`
    FileSize     int64    `json:"fileSize"`
    PageCount    int      `json:"pageCount,omitempty"`
    Sections     []string `json:"sections"`
    Language     string   `json:"language,omitempty"`
    Confidence   float64  `json:"confidence"`
    ProcessingMs int64    `json:"processingMs"`
}

// JSONConverter 实现文档转换器
type JSONConverter struct{}

func NewJSONConverter() *JSONConverter {
    return &JSONConverter{}
}

func (c *JSONConverter) Convert(chunks []models.DocumentChunk) (*ProcessedDocument, error) {
    if len(chunks) == 0 {
        return nil, fmt.Errorf("no chunks to convert")
    }

    // 初始化文档结构
    doc := &ProcessedDocument{
        Status:      "completed",
        ProcessedAt: time.Now(),
        Content:     make([]ChunkContent, 0, len(chunks)),
        Metadata: DocumentMetadata{
            Sections:   make([]string, 0),
            Confidence: 1.0,
        },
    }

    // 处理每个文档块
    sections := make(map[string]bool)
    var totalConfidence float64

    for i, chunk := range chunks {
        // 创建内容块
        content := ChunkContent{
            Text:     chunk.Content,
            Position: i + 1,
            Metadata: chunk.Metadata,
        }

        // 根据元数据设置类型
        if pageNum, ok := chunk.Metadata["pageNumber"]; ok {
            content.Type = "page"
            content.Metadata["pageNumber"] = pageNum
        } else if imgType, ok := chunk.Metadata["imageType"]; ok {
            content.Type = "image"
            content.Metadata["imageType"] = imgType
        }

        doc.Content = append(doc.Content, content)

        // 收集元数据
        if section, ok := chunk.Metadata["section"].(string); ok {
            sections[section] = true
        }
        if conf, ok := chunk.Metadata["confidence"].(float64); ok {
            totalConfidence += conf
        }
    }

    // 设置元数据
    for section := range sections {
        doc.Metadata.Sections = append(doc.Metadata.Sections, section)
    }
    
    // 计算平均置信度
    if len(chunks) > 0 {
        doc.Metadata.Confidence = totalConfidence / float64(len(chunks))
    }

    // 设置文档基本信息
    if len(chunks) > 0 && len(chunks[0].Metadata) > 0 {
        if fileName, ok := chunks[0].Metadata["filename"].(string); ok {
            doc.Metadata.FileName = fileName
        }
        if fileType, ok := chunks[0].Metadata["type"].(string); ok {
            doc.Metadata.FileType = fileType
        }
        if fileSize, ok := chunks[0].Metadata["size"].(int64); ok {
            doc.Metadata.FileSize = fileSize
        }
        if pageCount, ok := chunks[0].Metadata["pageCount"].(int); ok {
            doc.Metadata.PageCount = pageCount
        }
    }

    return doc, nil
}
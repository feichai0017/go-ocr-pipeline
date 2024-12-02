package models

import (
    "time"
)

// FileType 文件类型
type FileType string

const (
    PDF   FileType = "pdf"
    Image FileType = "image"
    Word  FileType = "word"
)

// DocumentMetadata 文档元数据
type DocumentMetadata struct {
    ID        string                 `json:"id"`
    Title     string                 `json:"title"`
    Author    string                 `json:"author"`
    FileType  FileType              `json:"fileType"`
    FileSize  int64                 `json:"fileSize"`
    MimeType  string                `json:"mimeType"`
    Pages     int                   `json:"pages"`
    CreatedAt time.Time             `json:"createdAt"`
    Hash      string                `json:"hash"`
    Extra     map[string]interface{} `json:"extra,omitempty"`
    Properties map[string]interface{} `json:"properties"`
}

// DocumentChunk 文档块
type DocumentChunk struct {
    Content  string                 `json:"content"`
    Metadata map[string]interface{} `json:"metadata"`
}

type ProcessingTask struct {
    ID        string            `json:"id"`
    Status    ProcessingStatus  `json:"status"`
    Type      string           `json:"type"`
    Priority  int              `json:"priority"`
    Progress  float64          `json:"progress"`
    Error     string            `json:"error,omitempty"`
    Metadata  map[string]string `json:"metadata"`
    CreatedAt time.Time        `json:"createdAt"`
    UpdatedAt time.Time        `json:"updatedAt,omitempty"`
}

type ProcessingStatus string

const (
    StatusPending   ProcessingStatus = "pending"
    StatusRunning   ProcessingStatus = "running"
    StatusCompleted ProcessingStatus = "completed"
    StatusFailed    ProcessingStatus = "failed"
    StatusCancelled ProcessingStatus = "cancelled"
)


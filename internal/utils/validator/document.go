// internal/utils/validator/document.go
package validator

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "path/filepath"
    "strings"
    "sync"
    
    "github.com/feichai0017/document-processor/pkg/logger"
)

// DocumentValidator 文档验证器
type DocumentValidator struct {
    logger     logger.Logger
    config     *ValidatorConfig
    mimeTypes  map[string][]string
    mu         sync.RWMutex
}

// ValidatorConfig 验证器配置
type ValidatorConfig struct {
    MaxFileSize    int64               // 最大文件大小（字节）
    AllowedTypes   map[string][]string // 允许的文件类型 {扩展名: []MIME类型}
    MinDimension   int                 // 图片最小尺寸
    MaxDimension   int                 // 图片最大尺寸
    MaxPageCount   int                 // PDF最大页数
    EnableVirusScan bool               // 是否启用病毒扫描
}

// ValidationResult 验证结果
type ValidationResult struct {
    IsValid     bool              `json:"isValid"`
    Errors      []ValidationError `json:"errors,omitempty"`
    FileInfo    FileInfo         `json:"fileInfo"`
}

// ValidationError 验证错误
type ValidationError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Field   string `json:"field,omitempty"`
}

// FileInfo 文件信息
type FileInfo struct {
    Filename    string            `json:"filename"`
    Size        int64            `json:"size"`
    MimeType    string           `json:"mimeType"`
    Extension   string           `json:"extension"`
    Hash        string           `json:"hash"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewDocumentValidator 创建新的文档验证器
func NewDocumentValidator(logger logger.Logger, config *ValidatorConfig) *DocumentValidator {
    if config == nil {
        config = &ValidatorConfig{
            MaxFileSize: 50 * 1024 * 1024, // 50MB
            AllowedTypes: map[string][]string{
                ".pdf":  {"application/pdf"},
                ".doc":  {"application/msword"},
                ".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
                ".jpg":  {"image/jpeg"},
                ".jpeg": {"image/jpeg"},
                ".png":  {"image/png"},
                ".tiff": {"image/tiff"},
            },
            MinDimension:   100,    // 最小100像素
            MaxDimension:   5000,   // 最大5000像素
            MaxPageCount:   1000,   // 最大1000页
            EnableVirusScan: false,
        }
    }

    return &DocumentValidator{
        logger:    logger,
        config:    config,
        mimeTypes: make(map[string][]string),
    }
}

// ValidateFile 验证单个文件
func (v *DocumentValidator) ValidateFile(file *multipart.FileHeader) (*ValidationResult, error) {
    result := &ValidationResult{
        IsValid:  true,
        Errors:   make([]ValidationError, 0),
        FileInfo: FileInfo{
            Filename:  file.Filename,
            Size:      file.Size,
            Extension: strings.ToLower(filepath.Ext(file.Filename)),
            Metadata:  make(map[string]interface{}),
        },
    }

    // 打开文件
    f, err := file.Open()
    if err != nil {
        return nil, fmt.Errorf("failed to open file: %w", err)
    }
    defer f.Close()

    // 计算文件哈希
    hash, err := v.calculateHash(f)
    if err != nil {
        return nil, fmt.Errorf("failed to calculate hash: %w", err)
    }
    result.FileInfo.Hash = hash

    // 重置文件指针
    if _, err := f.Seek(0, 0); err != nil {
        return nil, fmt.Errorf("failed to reset file pointer: %w", err)
    }

    // 基本验证
    if errs := v.performBasicValidation(result.FileInfo); len(errs) > 0 {
        result.IsValid = false
        result.Errors = append(result.Errors, errs...)
    }

    // MIME类型验证
    mimeType, err := v.detectMimeType(f)
    if err != nil {
        return nil, fmt.Errorf("failed to detect mime type: %w", err)
    }
    result.FileInfo.MimeType = mimeType

    if errs := v.validateMimeType(result.FileInfo); len(errs) > 0 {
        result.IsValid = false
        result.Errors = append(result.Errors, errs...)
    }

    // 根据文件类型进行特定验证
    if errs := v.performTypeSpecificValidation(f, result.FileInfo); len(errs) > 0 {
        result.IsValid = false
        result.Errors = append(result.Errors, errs...)
    }

    // 病毒扫描（如果启用）
    if v.config.EnableVirusScan {
        if errs := v.performVirusScan(f); len(errs) > 0 {
            result.IsValid = false
            result.Errors = append(result.Errors, errs...)
        }
    }

    return result, nil
}

// ValidateFiles 批量验证文件
func (v *DocumentValidator) ValidateFiles(files []*multipart.FileHeader) ([]*ValidationResult, error) {
    results := make([]*ValidationResult, len(files))
    var wg sync.WaitGroup
    errCh := make(chan error, len(files))

    for i, file := range files {
        wg.Add(1)
        go func(index int, file *multipart.FileHeader) {
            defer wg.Done()

            result, err := v.ValidateFile(file)
            if err != nil {
                errCh <- err
                return
            }
            results[index] = result
        }(i, file)
    }

    wg.Wait()
    close(errCh)

    if err := <-errCh; err != nil {
        return nil, err
    }

    return results, nil
}

// 基本验证
func (v *DocumentValidator) performBasicValidation(fileInfo FileInfo) []ValidationError {
    var errors []ValidationError

    // 检查文件大小
    if fileInfo.Size > v.config.MaxFileSize {
        errors = append(errors, ValidationError{
            Code:    "FILE_TOO_LARGE",
            Message: fmt.Sprintf("File size exceeds maximum limit of %d bytes", v.config.MaxFileSize),
            Field:   "size",
        })
    }

    // 检查文件扩展名
    if _, ok := v.config.AllowedTypes[fileInfo.Extension]; !ok {
        errors = append(errors, ValidationError{
            Code:    "INVALID_FILE_TYPE",
            Message: fmt.Sprintf("File type %s is not allowed", fileInfo.Extension),
            Field:   "extension",
        })
    }

    return errors
}

// MIME类型验证
func (v *DocumentValidator) validateMimeType(fileInfo FileInfo) []ValidationError {
    var errors []ValidationError

    allowedMimes, ok := v.config.AllowedTypes[fileInfo.Extension]
    if !ok {
        return []ValidationError{{
            Code:    "INVALID_FILE_TYPE",
            Message: "File type not allowed",
            Field:   "mimeType",
        }}
    }

    mimeValid := false
    for _, mime := range allowedMimes {
        if mime == fileInfo.MimeType {
            mimeValid = true
            break
        }
    }

    if !mimeValid {
        errors = append(errors, ValidationError{
            Code:    "INVALID_MIME_TYPE",
            Message: fmt.Sprintf("Invalid MIME type %s for extension %s", fileInfo.MimeType, fileInfo.Extension),
            Field:   "mimeType",
        })
    }

    return errors
}

// 特定类型验证
func (v *DocumentValidator) performTypeSpecificValidation(file multipart.File, fileInfo FileInfo) []ValidationError {
    var errors []ValidationError

    switch fileInfo.Extension {
    case ".pdf":
        if errs := v.validatePDF(file); len(errs) > 0 {
            errors = append(errors, errs...)
        }
    case ".jpg", ".jpeg", ".png", ".tiff":
        if errs := v.validateImage(file); len(errs) > 0 {
            errors = append(errors, errs...)
        }
    case ".doc", ".docx":
        if errs := v.validateWord(file); len(errs) > 0 {
            errors = append(errors, errs...)
        }
    }

    return errors
}

// 检测MIME类型
func (v *DocumentValidator) detectMimeType(file multipart.File) (string, error) {
    // 读取文件头部
    buffer := make([]byte, 512)
    _, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        return "", err
    }

    // 重置文件指针
    if _, err := file.Seek(0, 0); err != nil {
        return "", err
    }

    return http.DetectContentType(buffer), nil
}

// 计算文件哈希
func (v *DocumentValidator) calculateHash(file multipart.File) (string, error) {
    hash := sha256.New()
    if _, err := io.Copy(hash, file); err != nil {
        return "", err
    }

    return hex.EncodeToString(hash.Sum(nil)), nil
}

// PDF特定验证
func (v *DocumentValidator) validatePDF(file multipart.File) []ValidationError {
    var errors []ValidationError
    // TODO: 实现PDF验证逻辑
    // - 检查页数
    // - 检查是否加密
    // - 检查PDF版本
    return errors
}

// 图片特定验证
func (v *DocumentValidator) validateImage(file multipart.File) []ValidationError {
    var errors []ValidationError
    // TODO: 实现图片验证逻辑
    // - 检查尺寸
    // - 检查分辨率
    // - 检查颜色空间
    return errors
}

// Word文档特定验证
func (v *DocumentValidator) validateWord(file multipart.File) []ValidationError {
    var errors []ValidationError
    // TODO: 实现Word文档验证逻辑
    // - 检查文档结构
    // - 检查宏
    // - 检查嵌入对象
    return errors
}

// 病毒扫描
func (v *DocumentValidator) performVirusScan(file multipart.File) []ValidationError {
    var errors []ValidationError
    if !v.config.EnableVirusScan {
        return errors
    }

    // TODO: 实现病毒扫描逻辑
    // - 集成防病毒引擎
    // - 扫描文件
    // - 返回结果

    return errors
}
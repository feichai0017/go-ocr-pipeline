package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "path/filepath"
    
    "github.com/gin-gonic/gin"
    "github.com/feichai0017/document-processor/internal/service/document"
    "github.com/feichai0017/document-processor/pkg/logger"
)

type DocumentHandler struct {
    service document.DocumentProcessor
    logger  logger.Logger
}

// ProcessResponse 定义处理响应结构
type ProcessResponse struct {
    TaskID    string `json:"taskId"`
    Status    string `json:"status"`
    Filename  string `json:"filename"`
    FileSize  int64  `json:"fileSize"`
    FileType  string `json:"fileType"`
    CreatedAt string `json:"createdAt"`
}

// ErrorResponse 定义错误响应结构
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}

func NewDocumentHandler(service document.DocumentProcessor, logger logger.Logger) *DocumentHandler {
    return &DocumentHandler{
        service: service,
        logger:  logger,
    }
}

// ProcessDocument 处理单个文档
func (h *DocumentHandler) ProcessDocument(c *gin.Context) {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        h.handleError(c, http.StatusBadRequest, "Invalid file upload", err)
        return
    }
    defer file.Close()

    task, err := h.service.ProcessFile(c.Request.Context(), file, header)
    if err != nil {
        h.handleError(c, http.StatusInternalServerError, "Failed to process file", err)
        return
    }

    c.JSON(http.StatusOK, ProcessResponse{
        TaskID:    task.ID,
        Status:    string(task.Status),
        Filename:  header.Filename,
        FileSize:  header.Size,
        FileType:  filepath.Ext(header.Filename),
        CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
    })
}

// ProcessBatch 批量处理文档
func (h *DocumentHandler) ProcessBatch(c *gin.Context) {
    form, err := c.MultipartForm()
    if err != nil {
        h.handleError(c, http.StatusBadRequest, "Invalid form data", err)
        return
    }

    files := form.File["files"]
    if len(files) == 0 {
        h.handleError(c, http.StatusBadRequest, "No files provided", nil)
        return
    }

    tasks, err := h.service.ProcessBatch(c.Request.Context(), files)
    if err != nil {
        h.handleError(c, http.StatusInternalServerError, "Failed to process files", err)
        return
    }

    responses := make([]ProcessResponse, len(tasks))
    for i, task := range tasks {
        responses[i] = ProcessResponse{
            TaskID:    task.ID,
            Status:    string(task.Status),
            Filename:  files[i].Filename,
            FileSize:  files[i].Size,
            FileType:  filepath.Ext(files[i].Filename),
            CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "message": fmt.Sprintf("Processing %d documents", len(files)),
        "tasks":   responses,
    })
}

// GetStatus 获取处理状态
func (h *DocumentHandler) GetStatus(c *gin.Context) {
    taskID := c.Param("taskId")
    if taskID == "" {
        h.handleError(c, http.StatusBadRequest, "Task ID is required", nil)
        return
    }

    task, err := h.service.GetProcessingStatus(c.Request.Context(), taskID)
    if err != nil {
        h.handleError(c, http.StatusInternalServerError, "Failed to get status", err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "taskId":    task.ID,
        "status":    string(task.Status),
        "progress":  task.Progress,
        "error":     task.Error,
        "metadata":  task.Metadata,
        "createdAt": task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
        "updatedAt": task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
    })
}

// DownloadResult 下载处理结果
func (h *DocumentHandler) DownloadResult(c *gin.Context) {
    taskID := c.Param("taskId")
    if taskID == "" {
        h.handleError(c, http.StatusBadRequest, "Task ID is required", nil)
        return
    }

    result, err := h.service.GetProcessedDocument(c.Request.Context(), taskID)
    if err != nil {
        h.handleError(c, http.StatusInternalServerError, "Failed to get result", err)
        return
    }

    // 将结果转换为 JSON
    resultJSON, err := json.Marshal(result)
    if err != nil {
        h.handleError(c, http.StatusInternalServerError, "Failed to serialize result", err)
        return
    }

    filename := fmt.Sprintf("result_%s.json", taskID)
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
    c.Data(http.StatusOK, "application/json", resultJSON)
}

// CancelTask 取消处理任务
func (h *DocumentHandler) CancelTask(c *gin.Context) {
    taskID := c.Param("taskId")
    if taskID == "" {
        h.handleError(c, http.StatusBadRequest, "Task ID is required", nil)
        return
    }

    if err := h.service.CancelTask(c.Request.Context(), taskID); err != nil {
        h.handleError(c, http.StatusInternalServerError, "Failed to cancel task", err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Task cancelled successfully",
        "taskId":  taskID,
    })
}

// handleError 统一错误处理
func (h *DocumentHandler) handleError(c *gin.Context, status int, message string, err error) {
    h.logger.Error(message,
        logger.String("path", c.Request.URL.Path),
        logger.Error(err),
    )

    response := ErrorResponse{
        Message: message,
    }
    if err != nil {
        response.Error = err.Error()
    }

    c.JSON(status, response)
}
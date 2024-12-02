package pdf

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "time"

    "github.com/ledongthuc/pdf"
    "golang.org/x/sync/errgroup"
    
    "github.com/feichai0017/document-processor/internal/models"
    "github.com/feichai0017/document-processor/pkg/logger"
)

type Processor struct {
    logger logger.Logger
}

func NewProcessor(logger logger.Logger) *Processor {
    return &Processor{
        logger: logger,
    }
}

func (p *Processor) CanProcess(mimeType string) bool {
    return mimeType == "application/pdf"
}

func (p *Processor) Process(ctx context.Context, file io.Reader) ([]models.DocumentChunk, error) {
    // 首先将文件读入内存
    content, err := io.ReadAll(file)
    if err != nil {
        return nil, err
    }

    // 创建一个bytes.Reader，它实现了io.ReaderAt接口
    reader := bytes.NewReader(content)

    // 使用正确的参数调用pdf.NewReader
    pdfReader, err := pdf.NewReader(reader, reader.Size())
    if err != nil {
        return nil, err
    }

    numPages := pdfReader.NumPage()
    chunks := make([]models.DocumentChunk, 0, numPages)

    // 计算文件哈希
    hash := sha256.Sum256(content)
    hashStr := hex.EncodeToString(hash[:])

    // 创建错误组以并行处理页面
    g, ctx := errgroup.WithContext(ctx)
    chunkChan := make(chan models.DocumentChunk, numPages)

    // 设置最大并发数
    maxWorkers := 4
    sem := make(chan struct{}, maxWorkers)
    
    // 并行处理每一页
    for i := 1; i <= numPages; i++ {
        pageNum := i
        g.Go(func() error {
            // 使用信号量控制并发
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }()
            case <-ctx.Done():
                return ctx.Err()
            }
            
            page := pdfReader.Page(pageNum)
            if page.V.IsNull() {
                return nil
            }

            text, err := page.GetPlainText(nil)
            if err != nil {
                return fmt.Errorf("failed to get text from page %d: %w", pageNum, err)
            }

            chunk := models.DocumentChunk{
                Content: text,
                Metadata: map[string]interface{}{
                    "page":     pageNum,
                    "hash":     hashStr,
                    "section":  fmt.Sprintf("page_%d", pageNum),
                },
            }

            select {
            case chunkChan <- chunk:
                return nil
            case <-ctx.Done():
                return ctx.Err()
            }
        })
    }

    // 等待所有页面处理完成
    go func() {
        g.Wait()
        close(chunkChan)
    }()

    // 收集结果
    for chunk := range chunkChan {
        chunks = append(chunks, chunk)
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }

    return p.postProcessChunks(chunks)
}

func (p *Processor) ExtractMetadata(ctx context.Context, file io.Reader) (models.DocumentMetadata, error) {
    content, err := io.ReadAll(file)
    if err != nil {
        return models.DocumentMetadata{}, err
    }

    // 创建一个bytes.Reader
    reader := bytes.NewReader(content)

    // 使用正确的参数调用pdf.NewReader
    pdfReader, err := pdf.NewReader(reader, reader.Size())
    if err != nil {
        return models.DocumentMetadata{}, err
    }

    // 计算文件哈希
    hash := sha256.Sum256(content)
    hashString := hex.EncodeToString(hash[:])

    // 初始化基本元数据
    metadata := models.DocumentMetadata{
        ID:        hashString[:8],
        Title:     "", // 将在后面尝试填充
        Author:    "", // 将在后面尝试填充
        FileType:  models.PDF,
        FileSize:  int64(len(content)),
        MimeType:  "application/pdf",
        Pages:     pdfReader.NumPage(),
        CreatedAt: time.Now(),
        Hash:      hashString,
    }

    // 尝试从PDF文档中获取更多信息
    trailer := pdfReader.Trailer()
    if !trailer.IsNull() {
        info := trailer.Key("Info")
        if !info.IsNull() {
            // 尝试获取标题
            title := info.Key("Title")
            if !title.IsNull() {
                metadata.Title = title.String()
            }

            // 尝试获取作者
            author := info.Key("Author")
            if !author.IsNull() {
                metadata.Author = author.String()
            }
        }
    }

    return metadata, nil
}

func (p *Processor) postProcessChunks(chunks []models.DocumentChunk) ([]models.DocumentChunk, error) {
    processed := make([]models.DocumentChunk, len(chunks))
    for i, chunk := range chunks {
        processed[i] = models.DocumentChunk{
            Content:  p.cleanText(chunk.Content),
            Metadata: chunk.Metadata,
        }
    }
    return processed, nil
}

func (p *Processor) cleanText(text string) string {
    // 实现文本清理逻辑
    return text
}

// Close 实现 document.Processor 接口的 Close 方法
func (p *Processor) Close() error {
    // PDF处理器没有需要清理的资源
    return nil
}
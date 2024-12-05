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
    // read all file content
    content, err := io.ReadAll(file)
    if err != nil {
        return nil, err
    }

    // create a bytes.Reader, it implements io.ReaderAt interface
    reader := bytes.NewReader(content)

    // use correct parameters to call pdf.NewReader
    pdfReader, err := pdf.NewReader(reader, reader.Size())
    if err != nil {
        return nil, err
    }

    numPages := pdfReader.NumPage()
    chunks := make([]models.DocumentChunk, 0, numPages)

    // calculate file hash
    hash := sha256.Sum256(content)
    hashStr := hex.EncodeToString(hash[:])

    // create error group to process pages in parallel
    g, ctx := errgroup.WithContext(ctx)
    chunkChan := make(chan models.DocumentChunk, numPages)

    // set max concurrency
    maxWorkers := 4
    sem := make(chan struct{}, maxWorkers)
    
    // process each page in parallel
    for i := 1; i <= numPages; i++ {
        pageNum := i
        g.Go(func() error {
            // use semaphore to control concurrency
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

    // wait for all pages to be processed
    go func() {
        g.Wait()
        close(chunkChan)
    }()

    // collect results
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

    // create a bytes.Reader
    reader := bytes.NewReader(content)

    // use correct parameters to call pdf.NewReader
    pdfReader, err := pdf.NewReader(reader, reader.Size())
    if err != nil {
        return models.DocumentMetadata{}, err
    }

    // calculate file hash
    hash := sha256.Sum256(content)
    hashString := hex.EncodeToString(hash[:])

    // initialize basic metadata
    metadata := models.DocumentMetadata{
        ID:        hashString[:8],
        Title:     "",
        Author:    "", 
        FileType:  models.PDF,
        FileSize:  int64(len(content)),
        MimeType:  "application/pdf",
        Pages:     pdfReader.NumPage(),
        CreatedAt: time.Now(),
        Hash:      hashString,
    }

    // try to get more information from PDF document
    trailer := pdfReader.Trailer()
    if !trailer.IsNull() {
        info := trailer.Key("Info")
        if !info.IsNull() {
            // try to get title
            title := info.Key("Title")
            if !title.IsNull() {
                metadata.Title = title.String()
            }

            // try to get author
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
    // implement text cleaning logic
    return text
}

// Close implements document.Processor interface's Close method
func (p *Processor) Close() error {
    // PDF processor has no resources to clean
    return nil
}
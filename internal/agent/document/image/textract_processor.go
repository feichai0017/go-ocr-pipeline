package image

import (
    "bytes"
    "context"
    "fmt"
    "image"
    "io"
    "strings"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/textract"
    "github.com/aws/aws-sdk-go-v2/service/textract/types"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/feichai0017/document-processor/internal/models"
    "github.com/feichai0017/document-processor/pkg/logger"
)

type TextractProcessor struct {
    client  *textract.Client
    logger  logger.Logger
    config  *TextractConfig
}

type TextractConfig struct {
    Region        string
    AccessKey     string
    SecretKey     string
    MinConfidence float32
    EnableTable   bool
    EnableForm    bool
    FeatureTypes  []types.FeatureType
	QueriesConfig []types.Query
}

func NewTextractProcessor(ctx context.Context, cfg *TextractConfig, log logger.Logger) (*TextractProcessor, error) {
    creds := credentials.NewStaticCredentialsProvider(
        cfg.AccessKey,
        cfg.SecretKey,
        "",
    )

    // load aws config
    awsCfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(cfg.Region),
        config.WithCredentialsProvider(creds),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to load AWS config: %w", err)
    }

    // create textract client
    client := textract.NewFromConfig(awsCfg)

    return &TextractProcessor{
        client: client,
        logger: log,
        config: cfg,
    }, nil
}

func (p *TextractProcessor) CanProcess(mimeType string) bool {
    supportedTypes := map[string]bool{
        "image/jpeg": true,
        "image/jpg":  true,
        "image/png":  true,
        "image/tiff": true,
        "application/pdf": true,
    }
    return supportedTypes[strings.ToLower(mimeType)]
}

func (p *TextractProcessor) Process(ctx context.Context, reader io.Reader) ([]models.DocumentChunk, error) {
    // read file content
    data, err := io.ReadAll(reader)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }

    // prepare textract request
    input := &textract.AnalyzeDocumentInput{
        Document: &types.Document{
            Bytes: data,
        },
        FeatureTypes: p.config.FeatureTypes,
    }

    // if queries config is set, add it to the request
    if len(p.config.QueriesConfig) > 0 {
        input.QueriesConfig = &types.QueriesConfig{
            Queries: p.config.QueriesConfig,
        }
    }

    // call textract api
    result, err := p.client.AnalyzeDocument(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("failed to analyze document: %w", err)
    }

    // process result
    chunks := []models.DocumentChunk{}
    
    // process text blocks
    textBlocks := p.processBlocks(result.Blocks)
    if len(textBlocks) > 0 {
        chunks = append(chunks, models.DocumentChunk{
            Content: strings.Join(textBlocks, "\n"),
            Metadata: map[string]interface{}{
                "source": "textract",
                "type":   "text",
            },
        })
    }

    // process tables (if enabled)
    if p.config.EnableTable {
        tables := p.processTables(result.Blocks)
        for _, table := range tables {
            chunks = append(chunks, models.DocumentChunk{
                Content: table.Content,
                Metadata: map[string]interface{}{
                    "source": "textract",
                    "type":   "table",
                    "rows":   table.Rows,
                    "cols":   table.Cols,
                },
            })
        }
    }

    // process forms (if enabled)
    if p.config.EnableForm {
        forms := p.processForms(result.Blocks)
        for _, form := range forms {
            chunks = append(chunks, models.DocumentChunk{
                Content: fmt.Sprintf("%s: %s", form.Key, form.Value),
                Metadata: map[string]interface{}{
                    "source": "textract",
                    "type":   "form",
                    "key":    form.Key,
                },
            })
        }
    }

    return chunks, nil
}

func (p *TextractProcessor) ExtractMetadata(ctx context.Context, reader io.Reader) (models.DocumentMetadata, error) {
    metadata := models.DocumentMetadata{
        Properties: make(map[string]interface{}),
    }

    // read image data
    data, err := io.ReadAll(reader)
    if err != nil {
        return metadata, fmt.Errorf("failed to read image data: %w", err)
    }

    // decode image to get basic info
    img, _, err := image.Decode(bytes.NewReader(data))
    if err != nil {
        return metadata, fmt.Errorf("failed to decode image: %w", err)
    }

    bounds := img.Bounds()
    metadata.Properties["width"] = bounds.Max.X - bounds.Min.X
    metadata.Properties["height"] = bounds.Max.Y - bounds.Min.Y
    metadata.Properties["processor"] = "textract"

    return metadata, nil
}

func (p *TextractProcessor) Close() error {
    // textract client doesn't need special cleanup
    return nil
}

// helper method: process text blocks
func (p *TextractProcessor) processBlocks(blocks []types.Block) []string {
    var texts []string
    for _, block := range blocks {
        if block.BlockType == types.BlockTypeLine && 
           block.Confidence != nil && 
           *block.Confidence >= p.config.MinConfidence {
            texts = append(texts, *block.Text)
        }
    }
    return texts
}

// table struct
type Table struct {
    Content string
    Rows    int
    Cols    int
    Cells   [][]string
}

// process tables
func (p *TextractProcessor) processTables(blocks []types.Block) []Table {
    var tables []Table
    var currentTable *Table
    
    for _, block := range blocks {
        // check if it's a table block
        if block.BlockType == types.BlockTypeTable {
            if currentTable != nil {
                tables = append(tables, *currentTable)
            }
            
            // get table rows and columns
            var rowCount, colCount int32
            for _, relationship := range block.Relationships {
                // use correct relationship type constant
                if relationship.Type == "CHILD" {
                    for _, childBlock := range blocks {
                        if childBlock.BlockType == types.BlockTypeCell {
                            if childBlock.RowIndex != nil && *childBlock.RowIndex > rowCount {
                                rowCount = *childBlock.RowIndex
                            }
                            if childBlock.ColumnIndex != nil && *childBlock.ColumnIndex > colCount {
                                colCount = *childBlock.ColumnIndex
                            }
                        }
                    }
                }
            }
            
            // initialize new table
            currentTable = &Table{
                Rows:  int(rowCount),
                Cols:  int(colCount),
                Cells: make([][]string, int(rowCount)),
            }
            for i := range currentTable.Cells {
                currentTable.Cells[i] = make([]string, int(colCount))
            }
        } else if block.BlockType == types.BlockTypeCell && currentTable != nil {
            if block.RowIndex != nil && block.ColumnIndex != nil {
                row := int(*block.RowIndex - 1)
                col := int(*block.ColumnIndex - 1)
                if block.Text != nil {
                    currentTable.Cells[row][col] = *block.Text
                }
            }
        }
    }
    
    if currentTable != nil {
        tables = append(tables, *currentTable)
    }
    
    return tables
}

// form field struct
type FormField struct {
    Key   string
    Value string
}

// process forms
func (p *TextractProcessor) processForms(blocks []types.Block) []FormField {
    var forms []FormField
    
    for _, block := range blocks {
        if block.BlockType == types.BlockTypeKeyValueSet &&
           block.EntityTypes != nil &&
           len(block.EntityTypes) > 0 &&
           block.EntityTypes[0] == types.EntityTypeKey {
            
            key := p.getTextFromRelationships(block.Relationships, blocks)
            value := p.getValueFromKeyBlock(block, blocks)
            
            if key != "" && value != "" {
                forms = append(forms, FormField{
                    Key:   key,
                    Value: value,
                })
            }
        }
    }
    
    return forms
}

// get text from relationships
func (p *TextractProcessor) getTextFromRelationships(relationships []types.Relationship, blocks []types.Block) string {
    var text strings.Builder
    
    for _, rel := range relationships {
        // use string constant instead of undefined constant
        if rel.Type == "CHILD" {
            for _, id := range rel.Ids {
                for _, block := range blocks {
                    if block.Id != nil && *block.Id == id && block.Text != nil {
                        text.WriteString(*block.Text)
                        text.WriteString(" ")
                    }
                }
            }
        }
    }
    
    return strings.TrimSpace(text.String())
}

// get value from key block
func (p *TextractProcessor) getValueFromKeyBlock(keyBlock types.Block, blocks []types.Block) string {
    for _, rel := range keyBlock.Relationships {
        // use string constant instead of undefined constant
        if rel.Type == "VALUE" {
            for _, id := range rel.Ids {
                for _, block := range blocks {
                    if block.Id != nil && *block.Id == id {
                        return p.getTextFromRelationships(block.Relationships, blocks)
                    }
                }
            }
        }
    }
    return ""
}

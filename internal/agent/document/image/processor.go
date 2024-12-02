// internal/agent/document/image/processor.go
package image

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "image"
    "image/color"
    "image/jpeg"
    _ "image/png"
    "io"
    "strings"
    "time"
    
    "github.com/disintegration/imaging"
    "github.com/otiai10/gosseract/v2"
    
    "github.com/feichai0017/document-processor/internal/models"
    "github.com/feichai0017/document-processor/pkg/logger"
)

// ImageProcessor 结构体定义
type Processor struct {
    logger        logger.Logger
    preprocessors []ImagePreprocessor
    config        *ProcessOptions
    ollamaPool    *OllamaClientPool
}

// 图像预处理接口
type ImagePreprocessor interface {
    Process(img image.Image) (image.Image, error)
}

// 处理选项
type ProcessOptions struct {
    Language      []string
    DPI           int
    PageSegMode   gosseract.PageSegMode
    Whitelist     string
    MinConfidence float64
    PreprocessConfig *PreprocessConfig
    OCRConfig     *OCRConfig
    OllamaConfig  *OllamaConfig
    TableConfig    *TableConfig
}

type OCRConfig struct {
    EnableLangModel bool
    ModelPath      string
    Dictionary     []string
    MinWordLength  int
    MaxWordLength  int
}

type PreprocessConfig struct {
    AdaptiveBlockSize  int
    AdaptiveConstant   float64
    MedianBlurSize     int
    BorderSize         int
    DeskewAngleLimit   float64
    Denoise           bool
    DenoiseStrength   float64
    Sharpen           bool
    SharpenStrength   float64
    ContrastNormalize bool
    GammaCorrection   float64
}

type OllamaConfig struct {
    Enabled     bool
    Endpoint    string
    Model       string
    MaxTokens   int
    Temperature float64
    Prompt      string
    MaxPoolSize int           // 新增：连接池大小
    PoolTimeout time.Duration // 新增：连接池超时时间
}

type TableConfig struct {
    Enabled        bool
    MinLineLength  int
    MaxLineGap     int
    EdgeThreshold  float64
    MinCellWidth   int
    MinCellHeight  int
}


// 创建新的处理器
func NewProcessor(logger logger.Logger, opts *ProcessOptions) (*Processor, error) {
    if logger == nil {
        return nil, fmt.Errorf("logger is required")
    }
    promptTemplate :=`Please analyze this image with high attention to detail and help improve the OCR results. Follow these steps:

    1. Text Verification:
    - Carefully examine the text detected by OCR: %s
    - Check for common OCR errors (0/O, 1/I/l, rn/m, etc.)
    - Verify numbers and special characters
    - Identify any missing or incorrectly merged words
    
    2. Layout Analysis:
    - Identify the document structure (headers, paragraphs, lists)
    - Note any columns or text blocks
    - Detect text alignment and formatting
    - Identify any tables or structured data
    
    3. Context-Based Correction:
    - Consider the document type and context
    - Check for domain-specific terminology
    - Verify proper nouns and technical terms
    - Ensure sentence coherence and grammatical correctness
    
    4. Output Format:
    - Provide the corrected text
    - Mark significant corrections with [CORRECTION: original -> corrected]
    - Note any uncertain interpretations with [UNCERTAIN: text]
    - Maintain original formatting where possible
    
    Please provide the most accurate transcription of the text in the image, incorporating all these aspects in your analysis.`

    // 设置默认选项
    if opts == nil {
        opts = &ProcessOptions{
            Language:      []string{"eng"},
            DPI:          300,
            PageSegMode:  gosseract.PSM_AUTO,
            MinConfidence: 60.0,
            PreprocessConfig: &PreprocessConfig{
                AdaptiveBlockSize:  11,
                AdaptiveConstant:   2,
                MedianBlurSize:     3,
                BorderSize:         10,
                DeskewAngleLimit:   5,
                DenoiseStrength:    0.5,
                SharpenStrength:    0.5,
                GammaCorrection:    1.0,
                Denoise:           true,
                Sharpen:           true,
                ContrastNormalize: true,
            },
            OCRConfig: &OCRConfig{
                EnableLangModel: true,
                MinWordLength:   3,
                MaxWordLength:   45,
            },
            OllamaConfig: &OllamaConfig{
                Enabled:     true,
                Endpoint:    "http://localhost:11434",
                Model:       "llama3.2-vision",
                MaxTokens:   2048,
                Temperature: 0.7,
                Prompt:      promptTemplate,
                MaxPoolSize: 4,
                PoolTimeout: time.Second * 30,
            },
            TableConfig: &TableConfig{
                Enabled:       true,
                MinLineLength: 50,
                MaxLineGap:    10,
                EdgeThreshold: 30,
                MinCellWidth:  20,
                MinCellHeight: 20,
            },
        }
    }


    // 构建预处理管道
    preprocessors := []ImagePreprocessor{
        NewGrayscaleProcessor(),
        NewDenoiseProcessor(opts.PreprocessConfig.DenoiseStrength),
        NewContrastNormalizationProcessor(),
        NewDeskewProcessor(opts.PreprocessConfig.DeskewAngleLimit),
        NewAdaptiveThresholdProcessor(
            opts.PreprocessConfig.AdaptiveBlockSize,
            opts.PreprocessConfig.AdaptiveConstant,
        ),
        NewSharpenProcessor(opts.PreprocessConfig.SharpenStrength),
    }

    return &Processor{
        logger:        logger,
        preprocessors: preprocessors,
        config:        opts,
        ollamaPool:    NewOllamaClientPool(opts.OllamaConfig),
    }, nil
}

func (p *Processor) CanProcess(mimeType string) bool {
    switch mimeType {
    case "image/jpeg", "image/jpg", "image/png", "image/tiff":
        return true
    default:
        return false
    }
}

// 处理图像
func (p *Processor) Process(ctx context.Context, file io.Reader) ([]models.DocumentChunk, error) {
    // 为每个任务创建新的 Tesseract 客户端
    client := gosseract.NewClient()
    defer client.Close()
    
    // 设置语言和页面分割模式
    if err := client.SetLanguage(strings.Join(p.config.Language, "+")); err != nil {
        return nil, fmt.Errorf("failed to set language: %w", err)
    }
    
    if err := client.SetPageSegMode(p.config.PageSegMode); err != nil {
        return nil, fmt.Errorf("failed to set page segmentation mode: %w", err)
    }

    // 读取图像数据
    imageData, err := io.ReadAll(file)
    if err != nil {
        return nil, fmt.Errorf("failed to read image data: %w", err)
    }

    // 解码图像
    img, _, err := image.Decode(bytes.NewReader(imageData))
    if err != nil {
        return nil, fmt.Errorf("failed to decode image: %w", err)
    }

    // 应用预处理管道
    processedImg, err := p.applyPreprocessing(img)
    if err != nil {
        return nil, fmt.Errorf("failed to preprocess image: %w", err)
    }

    // OCR 处理
    text, confidence, regions, err := p.performOCRWithClient(processedImg, client)
    if err != nil {
        return nil, err
    }

    // Ollama 视觉分析
    var ollamaText string
    if p.config.OllamaConfig.Enabled {
        // 从连接池获取客户端
        ollamaClient, err := p.ollamaPool.Get(ctx)
        if err != nil {
            p.logger.Error("Failed to get Ollama client", logger.Error(err))
        } else {
            defer p.ollamaPool.Put(ollamaClient)
            
            prompt := fmt.Sprintf(p.config.OllamaConfig.Prompt, text)
            ollamaText, err = ollamaClient.AnalyzeImage(ctx, processedImg, prompt)
            if err != nil {
                p.logger.Error("Failed to analyze image with Ollama", logger.Error(err))
            }
        }
    }

    // 合并结果
    chunks := []models.DocumentChunk{
        {
            Content: text,
            Metadata: map[string]interface{}{
                "source":     "tesseract",
                "confidence": confidence,
                "regions":    regions,
            },
        },
    }

    if ollamaText != "" {
        chunks = append(chunks, models.DocumentChunk{
            Content: ollamaText,
            Metadata: map[string]interface{}{
                "source": "ollama",
                "model":  p.config.OllamaConfig.Model,
            },
        })
    }

    return chunks, nil
}

// 图像预处理
func (p *Processor) applyPreprocessing(img image.Image) (image.Image, error) {
    if img == nil {
        return nil, fmt.Errorf("input image is nil")
    }

    var err error
    result := img
    
    for _, processor := range p.preprocessors {
        result, err = processor.Process(result)
        if err != nil {
            p.logger.Error("Preprocessing failed", logger.Error(err))
            return nil, fmt.Errorf("preprocessing failed: %w", err)
        }
        if result == nil {
            return nil, fmt.Errorf("preprocessor returned nil image")
        }
    }
    
    return result, nil
}

// 执行OCR (新方法，接受client参数)
func (p *Processor) performOCRWithClient(img image.Image, client *gosseract.Client) (string, float64, []map[string]interface{}, error) {
    // 置高级 OCR 参数
    if err := client.SetVariable("load_system_dawg", "1"); err != nil {
        return "", 0, nil, err
    }
    if err := client.SetVariable("language_model_penalty_non_dict_word", "0.8"); err != nil {
        return "", 0, nil, err
    }
    
    // 如果启用了语言模型
    if p.config.OCRConfig.EnableLangModel {
        if err := client.SetVariable("textord_force_make_prop_words", "1"); err != nil {
            return "", 0, nil, err
        }
        
        // 加载自定义词典
        if len(p.config.OCRConfig.Dictionary) > 0 {
            if err := p.loadCustomDictionaryWithClient(client); err != nil {
                p.logger.Error("Failed to load custom dictionary", logger.Error(err))
            }
        }
    }

    // 将图像转换为临时文件
    tmpImg := imaging.Clone(img)
    buf := new(bytes.Buffer)
    if err := jpeg.Encode(buf, tmpImg, &jpeg.Options{Quality: 100}); err != nil {
        return "", 0, nil, fmt.Errorf("failed to encode image: %w", err)
    }

    // 设置图像数据
    if err := client.SetImageFromBytes(buf.Bytes()); err != nil {
        return "", 0, nil, fmt.Errorf("failed to set image: %w", err)
    }

    // 获取文本
    text, err := client.Text()
    if err != nil {
        return "", 0, nil, fmt.Errorf("failed to get text: %w", err)
    }

    // 获取文本区域信息
    boxes, err := client.GetBoundingBoxesVerbose()
    if err != nil {
        p.logger.Error("Failed to get bounding boxes", logger.Error(err))
        return text, 0, []map[string]interface{}{}, nil
    }

    // 后处理识别结果
    text, confidence, regions := p.postProcessOCR(text, boxes)
    
    return text, confidence, regions, nil
}

// ExtractMetadata 实现 document.Processor 接口
func (p *Processor) ExtractMetadata(ctx context.Context, file io.Reader) (models.DocumentMetadata, error) {
    // 读取图像数据
    imageData, err := io.ReadAll(file)
    if err != nil {
        return models.DocumentMetadata{}, fmt.Errorf("failed to read image data: %w", err)
    }

    // 解码图像
    img, format, err := image.Decode(bytes.NewReader(imageData))
    if err != nil {
        return models.DocumentMetadata{}, fmt.Errorf("failed to decode image: %w", err)
    }

    // 计算文件哈希
    hash := sha256.Sum256(imageData)
    hashString := hex.EncodeToString(hash[:])

    bounds := img.Bounds()
    metadata := models.DocumentMetadata{
        ID:        hashString[:8],
        FileType:  models.Image,
        FileSize:  int64(len(imageData)),
        MimeType:  "image/" + format,
        Pages:     1,
        CreatedAt: time.Now(),
        Hash:      hashString,
        Extra: map[string]interface{}{
            "width":  bounds.Dx(),
            "height": bounds.Dy(),
            "format": format,
        },
    }

    return metadata, nil
}

type ContrastProcessor struct {
    amount float64
}

func NewContrastProcessor(amount float64) *ContrastProcessor {
    return &ContrastProcessor{amount: amount}
}

func (p *ContrastProcessor) Process(img image.Image) (image.Image, error) {
    return imaging.AdjustContrast(img, p.amount), nil
}

type BinarizationProcessor struct {
    threshold uint8
}

func NewBinarizationProcessor(threshold uint8) *BinarizationProcessor {
    return &BinarizationProcessor{threshold: threshold}
}

func (p *BinarizationProcessor) Process(img image.Image) (image.Image, error) {
    // 先转换为灰度图
    grayImg := imaging.Grayscale(img)
    bounds := grayImg.Bounds()
    binary := image.NewGray(bounds)

    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            // 获取灰度值
            grayColor := grayImg.At(x, y)
            grayValue := color.GrayModel.Convert(grayColor).(color.Gray).Y

            // 应用阈值
            if grayValue > p.threshold {
                binary.Set(x, y, color.White)
            } else {
                binary.Set(x, y, color.Black)
            }
        }
    }
    return binary, nil
}

type NoiseReductionProcessor struct{}

func NewNoiseReductionProcessor() *NoiseReductionProcessor {
    return &NoiseReductionProcessor{}
}

func (p *NoiseReductionProcessor) Process(img image.Image) (image.Image, error) {
    // 应用中值滤波减少噪声
    return imaging.Sharpen(img, 0.5), nil
}

// 加载自定义词典 (新方法，接受client参数)
func (p *Processor) loadCustomDictionaryWithClient(client *gosseract.Client) error {
    if err := client.SetVariable("user_words_suffix", "user-words"); err != nil {
        return err
    }
    
    if err := client.SetVariable("user_patterns_suffix", "user-patterns"); err != nil {
        return err
    }

    return nil
}

// 后处理OCR结果
func (p *Processor) postProcessOCR(text string, boxes []gosseract.BoundingBox) (string, float64, []map[string]interface{}) {
    var totalConfidence float64
    var validBoxes []gosseract.BoundingBox

    // 过滤低置信度结果
    for _, box := range boxes {
        if box.Confidence >= p.config.MinConfidence {
            validBoxes = append(validBoxes, box)
            totalConfidence += box.Confidence
        }
    }

    // 转换为区域信息
    regions := make([]map[string]interface{}, len(validBoxes))
    for i, box := range validBoxes {
        regions[i] = map[string]interface{}{
            "x":          box.Box.Min.X,
            "y":          box.Box.Min.Y,
            "width":      box.Box.Max.X - box.Box.Min.X,
            "height":     box.Box.Max.Y - box.Box.Min.Y,
            "text":       box.Word,
            "confidence": box.Confidence,
        }
    }

    avgConfidence := 0.0
    if len(validBoxes) > 0 {
        avgConfidence = totalConfidence / float64(len(validBoxes))
    }

    return text, avgConfidence, regions
}

// Close 实现 document.Processor 接口的 Close 方法
func (p *Processor) Close() error {
    if p.ollamaPool != nil {
        return p.ollamaPool.Close()
    }
    return nil
}

func (p *Processor) ProcessTable(ctx context.Context, img image.Image) ([]TableCell, error) {
    // 为表格处理创建新的 Tesseract 客户端
    client := gosseract.NewClient()
    defer client.Close()
    
    // 设置语言和页面分割模式
    if err := client.SetLanguage(strings.Join(p.config.Language, "+")); err != nil {
        return nil, fmt.Errorf("failed to set language: %w", err)
    }
    
    if err := client.SetPageSegMode(p.config.PageSegMode); err != nil {
        return nil, fmt.Errorf("failed to set page segmentation mode: %w", err)
    }

    // 1. 预处理图像
    processedImg := img
    for _, processor := range p.preprocessors {
        var err error
        processedImg, err = processor.Process(processedImg)
        if err != nil {
            return nil, fmt.Errorf("preprocessing failed: %w", err)
        }
    }
    
    // 2. 检测表格结构
    tableDetector := NewTableDetectionProcessor(
        p.config.TableConfig.MinLineLength,
        p.config.TableConfig.MaxLineGap,
    )
    
    cells, err := tableDetector.detectTableCells(processedImg)
    if err != nil {
        return nil, fmt.Errorf("table detection failed: %w", err)
    }
    
    // 3. 处理每个单元格
    for i := range cells {
        // 提取单元格图像
        cellImg := imaging.Crop(processedImg, cells[i].Bounds)
        
        // OCR识别
        text, _, _, err := p.performOCRWithClient(cellImg, client)
        if err != nil {
            p.logger.Error("Failed to recognize cell text", logger.Error(err))
            continue
        }
        
        cells[i].Content = text
    }
    
    return cells, nil
}

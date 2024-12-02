package image

import (
    "image"
    "image/color"
    "image/draw"
    "math"
    "github.com/disintegration/imaging"
    "fmt"
)

// 灰度处理器
type GrayscaleProcessor struct{}

func NewGrayscaleProcessor() *GrayscaleProcessor {
    return &GrayscaleProcessor{}
}

func (p *GrayscaleProcessor) Process(img image.Image) (image.Image, error) {
    return imaging.Grayscale(img), nil
}

// 倾斜校正处理器
type DeskewProcessor struct {
    angleLimit float64
}

func NewDeskewProcessor(angleLimit float64) *DeskewProcessor {
    return &DeskewProcessor{
        angleLimit: angleLimit,
    }
}

func (p *DeskewProcessor) Process(img image.Image) (image.Image, error) {
    // 基本的倾斜校正实现
    angle := p.detectSkewAngle(img)
    if math.Abs(angle) < p.angleLimit {
        return imaging.Rotate(img, angle, color.White), nil
    }
    return img, nil
}

func (p *DeskewProcessor) detectSkewAngle(img image.Image) float64 {
    // 简单的倾斜检测实现
    // 实际项目中可以使用更复杂的算法
    return 0
}

// 自适应阈值处理器
type AdaptiveThresholdProcessor struct {
    blockSize int
    constant  float64
}

func NewAdaptiveThresholdProcessor(blockSize int, constant float64) *AdaptiveThresholdProcessor {
    return &AdaptiveThresholdProcessor{
        blockSize: blockSize,
        constant:  constant,
    }
}

func (p *AdaptiveThresholdProcessor) Process(img image.Image) (image.Image, error) {
    if img == nil {
        return nil, fmt.Errorf("input image is nil")
    }

    // 转换为灰度图像
    grayImg := imaging.Grayscale(img)
    bounds := grayImg.Bounds()
    
    // 创建新的二值化图像
    result := image.NewGray(bounds)
    draw.Draw(result, bounds, &image.Uniform{color.White}, image.Point{}, draw.Src)
    
    halfBlock := p.blockSize / 2
    
    // 应用自适应阈值
    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            var sum int
            var count int
            
            // 计算局部区域平均值
            for dy := -halfBlock; dy <= halfBlock; dy++ {
                for dx := -halfBlock; dx <= halfBlock; dx++ {
                    nx, ny := x+dx, y+dy
                    if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
                        grayValue := color.GrayModel.Convert(grayImg.At(nx, ny)).(color.Gray).Y
                        sum += int(grayValue)
                        count++
                    }
                }
            }
            
            if count > 0 {
                mean := float64(sum) / float64(count)
                pixel := color.GrayModel.Convert(grayImg.At(x, y)).(color.Gray).Y
                if float64(pixel) < mean-p.constant {
                    result.Set(x, y, color.Black)
                }
            }
        }
    }
    
    return result, nil
}

// 降噪处理器
type DenoiseProcessor struct {
    strength float64
}

func NewDenoiseProcessor(strength float64) *DenoiseProcessor {
    return &DenoiseProcessor{strength: strength}
}

func (p *DenoiseProcessor) Process(img image.Image) (image.Image, error) {
    // 使用高斯模糊进行降噪
    blurred := imaging.Blur(img, p.strength)
    return blurred, nil
}

// 锐化处理器
type SharpenProcessor struct {
    strength float64
}

func NewSharpenProcessor(strength float64) *SharpenProcessor {
    return &SharpenProcessor{strength: strength}
}

func (p *SharpenProcessor) Process(img image.Image) (image.Image, error) {
    return imaging.Sharpen(img, p.strength), nil
}

// 对比度处理器
type ContrastNormalizationProcessor struct{}

func NewContrastNormalizationProcessor() *ContrastNormalizationProcessor {
    return &ContrastNormalizationProcessor{}
}

func (p *ContrastNormalizationProcessor) Process(img image.Image) (image.Image, error) {
    return imaging.AdjustContrast(img, 20), nil
}

// 边缘检测处理器
type EdgeDetectionProcessor struct {
    threshold float64
}

func NewEdgeDetectionProcessor(threshold float64) *EdgeDetectionProcessor {
    return &EdgeDetectionProcessor{
        threshold: threshold,
    }
}

func (p *EdgeDetectionProcessor) Process(img image.Image) (image.Image, error) {
    // 转换为灰度图
    grayImg := imaging.Grayscale(img)
    bounds := grayImg.Bounds()
    result := image.NewGray(bounds)
    
    // Sobel 算子
    for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
        for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
            // 计算水平和垂直梯度
            gx := float64(color.GrayModel.Convert(grayImg.At(x+1, y)).(color.Gray).Y) -
                 float64(color.GrayModel.Convert(grayImg.At(x-1, y)).(color.Gray).Y)
            gy := float64(color.GrayModel.Convert(grayImg.At(x, y+1)).(color.Gray).Y) -
                 float64(color.GrayModel.Convert(grayImg.At(x, y-1)).(color.Gray).Y)
            
            magnitude := math.Sqrt(gx*gx + gy*gy)
            
            if magnitude > p.threshold {
                result.Set(x, y, color.Black)
            } else {
                result.Set(x, y, color.White)
            }
        }
    }
    
    return result, nil
}

// 表格检测处理器
type TableDetectionProcessor struct {
    minLineLength int
    maxLineGap    int
}

func NewTableDetectionProcessor(minLineLength, maxLineGap int) *TableDetectionProcessor {
    return &TableDetectionProcessor{
        minLineLength: minLineLength,
        maxLineGap:    maxLineGap,
    }
}

type TableCell struct {
    Bounds  image.Rectangle
    Content string
    Image   image.Image
}

func (p *TableDetectionProcessor) Process(img image.Image) (image.Image, error) {
    // 先进行边缘检测
    edgeDetector := NewEdgeDetectionProcessor(30)
    edges, err := edgeDetector.Process(img)
    if err != nil {
        return nil, err
    }
    
    // 检测水平和垂直线
    cells, err := p.detectTableCells(edges)
    if err != nil {
        return nil, err
    }
    
    // 绘制检测到的单元格
    result := image.NewRGBA(img.Bounds())
    draw.Draw(result, img.Bounds(), img, image.Point{}, draw.Src)
    
    for _, cell := range cells {
        // 绘制单元格边框
        p.drawCellBorder(result, cell.Bounds, color.RGBA{R: 255, G: 0, B: 0, A: 255})
    }
    
    return result, nil
}

func (p *TableDetectionProcessor) detectTableCells(img image.Image) ([]TableCell, error) {
    bounds := img.Bounds()
    grayImg := imaging.Grayscale(img)
    
    // 存储检测到的水平和垂直线
    horizontalLines := make([]image.Rectangle, 0)
    verticalLines := make([]image.Rectangle, 0)
    
    // 检测水平线
    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        lineStart := -1
        blackCount := 0
        
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            if color.GrayModel.Convert(grayImg.At(x, y)).(color.Gray).Y < 128 {
                if lineStart == -1 {
                    lineStart = x
                }
                blackCount++
            } else if lineStart != -1 {
                if blackCount >= p.minLineLength {
                    horizontalLines = append(horizontalLines, image.Rect(lineStart, y, x, y+1))
                }
                lineStart = -1
                blackCount = 0
            }
        }
    }
    
    // 检测垂直线
    for x := bounds.Min.X; x < bounds.Max.X; x++ {
        lineStart := -1
        blackCount := 0
        
        for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
            if color.GrayModel.Convert(grayImg.At(x, y)).(color.Gray).Y < 128 {
                if lineStart == -1 {
                    lineStart = y
                }
                blackCount++
            } else if lineStart != -1 {
                if blackCount >= p.minLineLength {
                    verticalLines = append(verticalLines, image.Rect(x, lineStart, x+1, y))
                }
                lineStart = -1
                blackCount = 0
            }
        }
    }
    
    // 合并相近的线段
    horizontalLines = p.mergeLines(horizontalLines, true)
    verticalLines = p.mergeLines(verticalLines, false)
    
    // 查找交点构建单元格
    cells := p.buildCells(horizontalLines, verticalLines)
    
    return cells, nil
}

// 合并相近的线段
func (p *TableDetectionProcessor) mergeLines(lines []image.Rectangle, isHorizontal bool) []image.Rectangle {
    if len(lines) < 2 {
        return lines
    }
    
    merged := make([]image.Rectangle, 0)
    current := lines[0]
    
    for i := 1; i < len(lines); i++ {
        if isHorizontal {
            if lines[i].Min.Y-current.Max.Y <= p.maxLineGap {
                current = image.Rect(
                    min(current.Min.X, lines[i].Min.X),
                    current.Min.Y,
                    max(current.Max.X, lines[i].Max.X),
                    lines[i].Max.Y,
                )
            } else {
                merged = append(merged, current)
                current = lines[i]
            }
        } else {
            if lines[i].Min.X-current.Max.X <= p.maxLineGap {
                current = image.Rect(
                    current.Min.X,
                    min(current.Min.Y, lines[i].Min.Y),
                    lines[i].Max.X,
                    max(current.Max.Y, lines[i].Max.Y),
                )
            } else {
                merged = append(merged, current)
                current = lines[i]
            }
        }
    }
    merged = append(merged, current)
    return merged
}

// 构建单元格
func (p *TableDetectionProcessor) buildCells(horizontalLines, verticalLines []image.Rectangle) []TableCell {
    cells := make([]TableCell, 0)
    
    // 找到所有交点
    for i := 0; i < len(horizontalLines)-1; i++ {
        for j := 0; j < len(verticalLines)-1; j++ {
            // 构建单元格边界
            cellBounds := image.Rect(
                verticalLines[j].Min.X,
                horizontalLines[i].Min.Y,
                verticalLines[j+1].Max.X,
                horizontalLines[i+1].Max.Y,
            )
            
            cells = append(cells, TableCell{
                Bounds: cellBounds,
            })
        }
    }
    
    return cells
}

// 辅助函数
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// 在 TableDetectionProcessor 结构体的方法中添加
func (p *TableDetectionProcessor) drawCellBorder(img *image.RGBA, bounds image.Rectangle, color color.Color) {
    // 绘制水平线
    for x := bounds.Min.X; x <= bounds.Max.X; x++ {
        img.Set(x, bounds.Min.Y, color) // 上边框
        img.Set(x, bounds.Max.Y, color) // 下边框
    }
    
    // 绘制垂直线
    for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
        img.Set(bounds.Min.X, y, color) // 左边框
        img.Set(bounds.Max.X, y, color) // 右边框
    }
}
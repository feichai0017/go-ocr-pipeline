package image

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "image"
    "image/jpeg"
    "io"
    "net/http"
    "time"
)

// OllamaResponse 定义 Ollama API 响应结构
type OllamaResponse struct {
    Response    string    `json:"response"`
    Created     int64     `json:"created"`
    Model       string    `json:"model"`
    Done        bool      `json:"done"`
    Context     []int     `json:"context,omitempty"`
    TotalDuration int64   `json:"total_duration,omitempty"`
    LoadDuration  int64   `json:"load_duration,omitempty"`
    PromptEvalCount int   `json:"prompt_eval_count,omitempty"`
    EvalCount    int      `json:"eval_count,omitempty"`
    EvalDuration int64    `json:"eval_duration,omitempty"`
    Error        string   `json:"error,omitempty"`
}

type OllamaClient struct {
    endpoint    string
    model       string
    maxTokens   int
    temperature float64
    httpClient  *http.Client
}

func NewOllamaClient(config *OllamaConfig) *OllamaClient {
    return &OllamaClient{
        endpoint:    config.Endpoint,
        model:       config.Model,
        maxTokens:   config.MaxTokens,
        temperature: config.Temperature,
        httpClient: &http.Client{
            Timeout: 120 * time.Second,
        },
    }
}

func (c *OllamaClient) AnalyzeImage(ctx context.Context, img image.Image, prompt string) (string, error) {
    // 将图像转换为 base64
    buf := new(bytes.Buffer)
    if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 85}); err != nil {
        return "", fmt.Errorf("failed to encode image: %w", err)
    }
    base64Img := base64.StdEncoding.EncodeToString(buf.Bytes())

    // 准备请求体
    reqBody := map[string]interface{}{
        "model":       c.model,
        "prompt":      fmt.Sprintf("[img]%s[/img]\n%s", base64Img, prompt),
        "stream":      false,
        "raw":         true,
        "max_tokens":  c.maxTokens,
        "temperature": c.temperature,
    }

    reqData, err := json.Marshal(reqBody)
    if err != nil {
        return "", fmt.Errorf("failed to marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/generate", bytes.NewReader(reqData))
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
    }

    var result OllamaResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("failed to decode response: %w", err)
    }

    if result.Error != "" {
        return "", fmt.Errorf("ollama error: %s", result.Error)
    }

    return result.Response, nil
}

func (c *OllamaClient) Close() error {
    c.httpClient.CloseIdleConnections()
    return nil
}

type OllamaClientPool struct {
    clients chan *OllamaClient
    config  *OllamaConfig
}

func NewOllamaClientPool(config *OllamaConfig) *OllamaClientPool {
    pool := &OllamaClientPool{
        clients: make(chan *OllamaClient, config.MaxPoolSize),
        config:  config,
    }
    
    // 预创建客户端
    for i := 0; i < config.MaxPoolSize; i++ {
        pool.clients <- NewOllamaClient(config)
    }
    
    return pool
}

func (p *OllamaClientPool) Get(ctx context.Context) (*OllamaClient, error) {
    select {
    case client := <-p.clients:
        return client, nil
    case <-time.After(p.config.PoolTimeout):
        return nil, fmt.Errorf("timeout waiting for available client")
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (p *OllamaClientPool) Put(client *OllamaClient) {
    select {
    case p.clients <- client:
    default:
        // 池已满，丢弃客户端
    }
}

func (p *OllamaClientPool) Close() error {
    close(p.clients)
    // 关闭所有客户端
    for client := range p.clients {
        client.Close()
    }
    return nil
} 
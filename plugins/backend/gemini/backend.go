//go:build backend_gemini

package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"centag/core/pkg/backend"
	"centag/core/pkg/plugin"
)

// Backend Gemini 后端插件
type Backend struct {
	name           string
	status         plugin.PluginStatus
	config         *plugin.BackendConfig
	backendManager *backend.Manager
	currentBackend *backend.BackendConfig
	client         *http.Client
	mu             sync.RWMutex
}

// NewBackend 创建 Gemini 后端插件
func NewBackend() (plugin.Plugin, error) {
	return &Backend{
		name:           "gemini-backend",
		status:         plugin.StatusStopped,
		backendManager: backend.GetManager(),
		config: &plugin.BackendConfig{
			BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
			Timeout:    60,
			MaxRetries: 3,
			RetryDelay: 1,
		},
		client: &http.Client{
			Timeout: 0,
		},
	}, nil
}

// Name 返回插件名称
func (b *Backend) Name() string {
	return b.name
}

// Type 返回插件类型
func (b *Backend) Type() plugin.PluginType {
	return plugin.TypeBackend
}

// Version 返回插件版本
func (b *Backend) Version() string {
	return "1.0.0"
}

// Init 初始化插件
func (b *Backend) Init(config any) error {
	if err := b.backendManager.Load(); err != nil {
		log.Printf("[Gemini Backend] Warning: failed to load backend config: %v", err)
	}

	if cfg, ok := config.(*plugin.BackendConfig); ok {
		b.config = cfg
	}

	b.client.Timeout = 0
	log.Printf("[Gemini Backend] Plugin initialized with BaseURL: %s", b.config.BaseURL)
	return nil
}

// Start 启动插件
func (b *Backend) Start(ctx context.Context) error {
	b.status = plugin.StatusRunning
	log.Printf("[Gemini Backend] Plugin started")
	return nil
}

// Stop 停止插件
func (b *Backend) Stop(ctx context.Context) error {
	b.status = plugin.StatusStopped
	log.Printf("[Gemini Backend] Plugin stopped")
	return nil
}

// Status 返回插件状态
func (b *Backend) Status() plugin.PluginStatus {
	return b.status
}

// CallModel 调用模型（非流式）
func (b *Backend) CallModel(ctx context.Context, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	// 构建 Gemini 请求
	geminiReq := b.buildRequest(req)

	// 序列化请求
	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	// 构建 URL
	url := b.buildURL(req.Model, false)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if b.config.APIKey != "" {
		httpReq.Header.Set("x-goog-api-key", b.config.APIKey)
	}

	// 发送请求
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 转换为 ProxyResponse
	return b.convertResponse(&geminiResp), nil
}

// CallModelStream 流式调用模型
func (b *Backend) CallModelStream(ctx context.Context, req *plugin.ProxyRequest) (<-chan plugin.StreamChunk, error) {
	// 构建 Gemini 请求
	geminiReq := b.buildRequest(req)
	geminiReq.GenerationConfig = &generationConfig{
		ResponseModalities: []string{"TEXT"},
	}

	// 序列化请求
	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	// 构建 URL
	url := b.buildURL(req.Model, true)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if b.config.APIKey != "" {
		httpReq.Header.Set("x-goog-api-key", b.config.APIKey)
	}

	// 发送请求
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("gemini api error: status=%d", resp.StatusCode)
	}

	// 创建输出 channel
	outputCh := make(chan plugin.StreamChunk, 100)

	// 启动 goroutine 处理流式响应
	go b.handleStreamResponse(ctx, resp, outputCh)

	return outputCh, nil
}

// Authenticate 使用配置进行认证
func (b *Backend) Authenticate(config any) error {
	if cfg, ok := config.(*plugin.BackendConfig); ok {
		b.config = cfg
	}
	return nil
}

// HealthCheck 健康检查
func (b *Backend) HealthCheck(ctx context.Context) error {
	url := b.config.BaseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if b.config.APIKey != "" {
		req.Header.Set("x-goog-api-key", b.config.APIKey)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("health check failed: status=%d", resp.StatusCode)
}

// GetAvailableModels 获取可用的模型列表
func (b *Backend) GetAvailableModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{ID: "gemini-pro", Name: "Gemini Pro", Enabled: true},
		{ID: "gemini-pro-vision", Name: "Gemini Pro Vision", Enabled: true},
	}, nil
}

// 辅助方法

func (b *Backend) buildRequest(req *plugin.ProxyRequest) *geminiRequest {
	// 转换消息格式
	contents := make([]content, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, content{
			Role:  role,
			Parts: []part{{Text: msg.Content}},
		})
	}

	return &geminiRequest{
		Contents: contents,
		GenerationConfig: &generationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
			TopP:            req.TopP,
		},
	}
}

func (b *Backend) buildURL(model string, stream bool) string {
	base := b.config.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}

	action := "generateContent"
	if stream {
		action = "streamGenerateContent?alt=sse"
	}

	return fmt.Sprintf("%s/models/%s:%s", base, model, action)
}

func (b *Backend) convertResponse(resp *geminiResponse) *plugin.ProxyResponse {
	if len(resp.Candidates) == 0 {
		return &plugin.ProxyResponse{
			Error: &plugin.ErrorResponse{
				Message: "no candidates in response",
				Type:    "invalid_response_error",
			},
		}
	}

	candidate := resp.Candidates[0]
	content := ""
	reasoningContent := ""

	if len(candidate.Content.Parts) > 0 {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
		}
	}

	// 处理 reasoning（如果有的话）
	if candidate.GroundingMetadata != nil {
		// grounding metadata 可能包含推理信息
	}

	return &plugin.ProxyResponse{
		Content:          content,
		ReasoningContent: reasoningContent,
		Model:            resp.ModelVersion,
		FinishReason:     string(candidate.FinishReason),
	}
}

func (b *Backend) handleStreamResponse(ctx context.Context, resp *http.Response, outputCh chan<- plugin.StreamChunk) {
	defer close(outputCh)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				outputCh <- plugin.StreamChunk{Done: true}
				return
			}

			var chunk geminiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// 转换为 StreamChunk
			streamChunk := plugin.StreamChunk{
				Content:      chunk.Candidates[0].Content.Parts[0].Text,
				FinishReason: string(chunk.Candidates[0].FinishReason),
			}

			outputCh <- streamChunk
		}
	}
}

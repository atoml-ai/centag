//go:build backend_azure

package azure

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

// Backend Azure OpenAI 后端插件
type Backend struct {
	name           string
	status         plugin.PluginStatus
	config         *plugin.BackendConfig
	backendManager *backend.Manager
	currentBackend *backend.BackendConfig
	client         *http.Client
	mu             sync.RWMutex
}

// NewBackend 创建 Azure OpenAI 后端插件
func NewBackend() (plugin.Plugin, error) {
	return &Backend{
		name:           "azure-backend",
		status:         plugin.StatusStopped,
		backendManager: backend.GetManager(),
		config: &plugin.BackendConfig{
			BaseURL:    "",
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
		log.Printf("[Azure Backend] Warning: failed to load backend config: %v", err)
	}

	if cfg, ok := config.(*plugin.BackendConfig); ok {
		b.config = cfg
	}

	b.client.Timeout = 0
	log.Printf("[Azure Backend] Plugin initialized with BaseURL: %s", b.config.BaseURL)
	return nil
}

// Start 启动插件
func (b *Backend) Start(ctx context.Context) error {
	b.status = plugin.StatusRunning
	log.Printf("[Azure Backend] Plugin started")
	return nil
}

// Stop 停止插件
func (b *Backend) Stop(ctx context.Context) error {
	b.status = plugin.StatusStopped
	log.Printf("[Azure Backend] Plugin stopped")
	return nil
}

// Status 返回插件状态
func (b *Backend) Status() plugin.PluginStatus {
	return b.status
}

// CallModel 调用模型（非流式）
func (b *Backend) CallModel(ctx context.Context, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	// 构建 Azure OpenAI 请求（与 OpenAI 兼容）
	reqBody := b.buildRequest(req)

	// 序列化请求
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 构建 URL
	url := b.buildURL(req.Model, false)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if b.config.APIKey != "" {
		httpReq.Header.Set("api-key", b.config.APIKey)
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
		return nil, fmt.Errorf("azure api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	// 解析响应（与 OpenAI 兼容）
	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 转换为 ProxyResponse
	return b.convertResponse(&openaiResp), nil
}

// CallModelStream 流式调用模型
func (b *Backend) CallModelStream(ctx context.Context, req *plugin.ProxyRequest) (<-chan plugin.StreamChunk, error) {
	// 构建请求
	reqBody := b.buildRequest(req)
	reqBody.Stream = true

	// 序列化请求
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 构建 URL
	url := b.buildURL(req.Model, true)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if b.config.APIKey != "" {
		httpReq.Header.Set("api-key", b.config.APIKey)
	}

	// 发送请求
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("azure api error: status=%d", resp.StatusCode)
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
	url := b.config.BaseURL + "/models?api-version=2024-02-01"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if b.config.APIKey != "" {
		req.Header.Set("api-key", b.config.APIKey)
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
		{ID: "gpt-4", Name: "GPT-4", Enabled: true},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Enabled: true},
		{ID: "gpt-35-turbo", Name: "GPT-3.5 Turbo", Enabled: true},
	}, nil
}

// 辅助方法

func (b *Backend) buildRequest(req *plugin.ProxyRequest) *openaiRequest {
	messages := make([]message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = message{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
		}
	}
	return &openaiRequest{
		Model:            req.Model,
		Messages:         messages,
		Temperature:      req.Temperature,
		MaxTokens:        req.MaxTokens,
		TopP:             req.TopP,
		FrequencyPenalty: req.FrequencyPenalty,
		PresencePenalty:  req.PresencePenalty,
		Stop:             req.Stop,
		Stream:           req.Stream,
	}
}

func (b *Backend) buildURL(model string, stream bool) string {
	base := b.config.BaseURL
	if base == "" {
		return ""
	}

	// Azure OpenAI URL 格式: {endpoint}/openai/deployments/{deployment}/chat/completions?api-version={api-version}
	apiVersion := "2024-02-01"
	if b.config.Custom != nil {
		if v, ok := b.config.Custom["api_version"]; ok {
			apiVersion = fmt.Sprintf("%v", v)
		}
	}

	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", base, model, apiVersion)
}

func (b *Backend) convertResponse(resp *openaiResponse) *plugin.ProxyResponse {
	if len(resp.Choices) == 0 {
		return &plugin.ProxyResponse{
			Error: &plugin.ErrorResponse{
				Message: "no choices in response",
				Type:    "invalid_response_error",
			},
		}
	}

	choice := resp.Choices[0]
	content := ""
	reasoningContent := ""

	if choice.Message.Content != "" {
		content = choice.Message.Content
	}

	// 处理 reasoning（如果有的话）
	if choice.Message.ReasoningContent != "" {
		reasoningContent = choice.Message.ReasoningContent
	}

	return &plugin.ProxyResponse{
		Content:          content,
		ReasoningContent: reasoningContent,
		Model:            resp.Model,
		FinishReason:     choice.FinishReason,
		TokensUsed:       resp.Usage.TotalTokens,
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

			var chunk openaiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]

			// 转换为 StreamChunk
			streamChunk := plugin.StreamChunk{
				Content:          choice.Delta.Content,
				ReasoningContent: choice.Delta.ReasoningContent,
				FinishReason:     choice.FinishReason,
				Done:             false,
			}

			outputCh <- streamChunk
		}
	}
}

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"centag/core/pkg/backend"
)

// OpenAIChatService OpenAI对话服务实现
type OpenAIChatService struct {
	config *ChatConfig
	client *http.Client
}

// NewOpenAIChatService 创建OpenAI对话服务
func NewOpenAIChatService(config *ChatConfig) (*OpenAIChatService, error) {
	if config == nil {
		config = DefaultChatConfig()
		config.Provider = "openai"
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}

	if config.Model == "" {
		config.Model = "gpt-3.5-turbo"
	}

	client := &http.Client{
		Timeout: 0, // 不设置超时，依赖 context 控制
	}

	return &OpenAIChatService{
		config: config,
		client: client,
	}, nil
}

// Chat 进行对话
func (s *OpenAIChatService) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}

	// 设置默认值
	if request.Model == "" {
		request.Model = s.config.Model
	}

	// 构造OpenAI请求体
	openaiReq := map[string]interface{}{
		"model":       request.Model,
		"messages":    request.Messages,
		"temperature": request.Temperature,
		"max_tokens":  request.MaxTokens,
	}

	reqBodyJSON, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	url := fmt.Sprintf("%s/chat/completions", s.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(s.config.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	return &ChatResponse{
		Content:      result.Choices[0].Message.Content,
		FinishReason: result.Choices[0].FinishReason,
		TokensUsed:   result.Usage.TotalTokens,
	}, nil
}

// GetProviderInfo 获取提供者信息
func (s *OpenAIChatService) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Provider: "openai",
		Model:    s.config.Model,
		BaseURL:  s.config.BaseURL,
	}
}

// OpenAIChatResponse OpenAI聊天API响应
type OpenAIChatResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []OpenAIChoice   `json:"choices"`
	Usage   OpenAIUsage      `json:"usage"`
}

// OpenAIChoice OpenAI选择项
type OpenAIChoice struct {
	Index        int          `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

// OpenAIMessage OpenAI消息
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIUsage OpenAI使用情况
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

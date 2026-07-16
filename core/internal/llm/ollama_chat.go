package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"centag/core/pkg/logger"
)

// OllamaChatService Ollama对话服务实现
type OllamaChatService struct {
	config *ChatConfig
	client *http.Client
}

// NewOllamaChatService 创建Ollama对话服务
func NewOllamaChatService(config *ChatConfig) (*OllamaChatService, error) {
	if config == nil {
		config = DefaultChatConfig()
		config.Provider = "ollama"
	}

	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:21434"
	}

	if config.Model == "" {
		config.Model = "llama3.2:3b" // 默认小模型
	}

	client := &http.Client{
		Timeout: 0, // 不设置超时，依赖 context 控制
	}

	return &OllamaChatService{
		config: config,
		client: client,
	}, nil
}

// Chat 进行对话
func (s *OllamaChatService) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}

	// 设置默认值
	if request.Model == "" {
		request.Model = s.config.Model
	}

	// 构造Ollama请求体
	ollamaReq := map[string]interface{}{
		"model":  request.Model,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": request.Temperature,
			"num_predict": request.MaxTokens,
		},
	}

	// 设置格式（用于JSON输出）
	if request.Format != "" {
		ollamaReq["format"] = request.Format
	}

	// 转换消息格式
	ollamaMessages := make([]map[string]interface{}, 0, len(request.Messages))
	for _, msg := range request.Messages {
		ollamaMessages = append(ollamaMessages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	ollamaReq["messages"] = ollamaMessages

	reqBodyJSON, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	url := fmt.Sprintf("%s/api/chat", s.config.BaseURL)
	logger.Infof("Ollama chat request - URL: %s, Model: %s", url, request.Model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	logger.Infof("Sending Ollama chat request...")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.Infof("Ollama chat response received - Status: %d", resp.StatusCode)

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ChatResponse{
		Content:      result.Message.Content,
		FinishReason: result.DoneReason,
		TokensUsed:   result.EvalCount + result.PromptEvalCount,
	}, nil
}

// GetProviderInfo 获取提供者信息
func (s *OllamaChatService) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Provider: "ollama",
		Model:    s.config.Model,
		BaseURL:  s.config.BaseURL,
	}
}

// OllamaChatResponse Ollama聊天API响应
type OllamaChatResponse struct {
	Model           string       `json:"model"`
	CreatedAt       string       `json:"created_at"`
	Message         OllamaMessage `json:"message"`
	DoneReason      string       `json:"done_reason"`
	Done            bool         `json:"done"`
	TotalDuration   int64        `json:"total_duration"`
	LoadDuration    int64        `json:"load_duration"`
	PromptEvalCount int          `json:"prompt_eval_count"`
	PromptEvalDuration int64     `json:"prompt_eval_duration"`
	EvalCount       int          `json:"eval_count"`
	EvalDuration    int64        `json:"eval_duration"`
}

// OllamaMessage Ollama消息
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

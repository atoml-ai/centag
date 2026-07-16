package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"centag/core/pkg/backend"
)

// LLMService LLM 服务接口
// 支持统一的本地和远程模型调用
type LLMService interface {
	// Generate 生成文本
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateJSON 生成结构化 JSON 数据
	GenerateJSON(ctx context.Context, prompt string, result interface{}) error

	// GetModelName 获取模型名称
	GetModelName() string

	// GetProvider 获取服务提供商
	GetProvider() string
}

// LLMConfig LLM 配置
type LLMConfig struct {
	Provider    string  `json:"provider" yaml:"provider"`       // openai, ollama, local
	ModelName   string  `json:"model_name" yaml:"model_name"`   // 模型名称
	BaseURL     string  `json:"base_url" yaml:"base_url"`       // API 基础 URL
	APIKey      string  `json:"api_key" yaml:"api_key"`         // API 密钥
	Temperature float32 `json:"temperature" yaml:"temperature"` // 温度参数 (0-1)
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`   // 最大 token 数
	Timeout     int     `json:"timeout" yaml:"timeout"`         // 超时时间 (秒)
}

// DefaultLLMConfig 返回默认配置
func DefaultLLMConfig() *LLMConfig {
	return &LLMConfig{
		Provider:    "openai",
		ModelName:   "gpt-4o-mini",
		BaseURL:     "https://api.openai.com/v1",
		APIKey:      "",
		Temperature: 0.7,
		MaxTokens:   2000,
		Timeout:     60,
	}
}

// OpenAILLMService OpenAI LLM 服务 (占位符实现)
type OpenAILLMService struct {
	config *LLMConfig
}

// NewOpenAILLMService 创建 OpenAI LLM 服务
func NewOpenAILLMService(config *LLMConfig) (*OpenAILLMService, error) {
	if config.Provider != "openai" {
		return nil, fmt.Errorf("invalid provider for OpenAILLMService: %s", config.Provider)
	}
	return &OpenAILLMService{
		config: config,
	}, nil
}

// Generate 生成文本
func (s *OpenAILLMService) Generate(ctx context.Context, prompt string) (string, error) {
	// 构建请求体
	requestBody := map[string]interface{}{
		"model": s.config.ModelName,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": s.config.Temperature,
		"max_tokens":  s.config.MaxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: time.Duration(s.config.Timeout) * time.Second,
	}

	// 构建请求 URL
	url := s.config.BaseURL + "/chat/completions"

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(s.config.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// 发送请求（带重试）
	var lastErr error
	maxRetries := 3

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			// 指数退避
			waitTime := time.Duration(retry*retry) * time.Second
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		defer resp.Body.Close()

		// 读取响应
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
			// 对于 429（速率限制）或 5xx 错误，进行重试
			if resp.StatusCode == 429 || resp.StatusCode >= 500 {
				continue
			}
			return "", lastErr
		}

		// 解析响应
		var openaiResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}

		if err := json.Unmarshal(respBody, &openaiResp); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		// 检查 API 错误
		if openaiResp.Error != nil {
			lastErr = fmt.Errorf("API error: %s (type: %s, code: %s)",
				openaiResp.Error.Message, openaiResp.Error.Type, openaiResp.Error.Code)
			continue
		}

		// 提取生成的文本
		if len(openaiResp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		return openaiResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("all retries failed: %w", lastErr)
}

// GenerateJSON 生成结构化 JSON 数据
func (s *OpenAILLMService) GenerateJSON(ctx context.Context, prompt string, result interface{}) error {
	// 添加 JSON 格式化指令
	enhancedPrompt := fmt.Sprintf("%s\n\nPlease ensure the output is valid JSON format.", prompt)

	response, err := s.Generate(ctx, enhancedPrompt)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(response), result)
}

// GetModelName 获取模型名称
func (s *OpenAILLMService) GetModelName() string {
	return s.config.ModelName
}

// GetProvider 获取服务提供商
func (s *OpenAILLMService) GetProvider() string {
	return s.config.Provider
}

// OllamaLLMService Ollama LLM 服务 (占位符实现)
type OllamaLLMService struct {
	config *LLMConfig
}

// NewOllamaLLMService 创建 Ollama LLM 服务
func NewOllamaLLMService(config *LLMConfig) (*OllamaLLMService, error) {
	if config.Provider != "ollama" {
		return nil, fmt.Errorf("invalid provider for OllamaLLMService: %s", config.Provider)
	}

	// 设置默认 BaseURL
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:21434"
	}

	return &OllamaLLMService{
		config: config,
	}, nil
}

// Generate 生成文本
func (s *OllamaLLMService) Generate(ctx context.Context, prompt string) (string, error) {
	// 构建请求体（使用 chat/completions 接口以获得更好的兼容性）
	requestBody := map[string]interface{}{
		"model": s.config.ModelName,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"options": map[string]interface{}{
			"temperature": float64(s.config.Temperature),
			"num_predict": s.config.MaxTokens,
		},
		"stream": false, // 非流式响应
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: time.Duration(s.config.Timeout) * time.Second,
	}

	// 构建请求 URL
	url := fmt.Sprintf("%s/api/chat", s.config.BaseURL)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求（带重试）
	var lastErr error
	maxRetries := 3

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			// 指数退避
			waitTime := time.Duration(retry*retry) * time.Second
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		defer resp.Body.Close()

		// 读取响应
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		// 解析响应
		var ollamaResp struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
			Error string `json:"error,omitempty"`
		}

		if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		// 检查 API 错误
		if ollamaResp.Error != "" {
			lastErr = fmt.Errorf("ollama error: %s", ollamaResp.Error)
			continue
		}

		// 提取生成的文本
		if !ollamaResp.Done {
			lastErr = fmt.Errorf("response not complete")
			continue
		}

		if ollamaResp.Message.Content == "" {
			return "", fmt.Errorf("empty content in response")
		}

		return ollamaResp.Message.Content, nil
	}

	return "", fmt.Errorf("all retries failed: %w", lastErr)
}

// GenerateJSON 生成结构化 JSON 数据
func (s *OllamaLLMService) GenerateJSON(ctx context.Context, prompt string, result interface{}) error {
	// Ollama 支持 format 参数
	enhancedPrompt := fmt.Sprintf("%s\n\nPlease output in valid JSON format.", prompt)

	response, err := s.Generate(ctx, enhancedPrompt)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(response), result)
}

// GetModelName 获取模型名称
func (s *OllamaLLMService) GetModelName() string {
	return s.config.ModelName
}

// GetProvider 获取服务提供商
func (s *OllamaLLMService) GetProvider() string {
	return s.config.Provider
}

// CreateLLMService 根据配置创建 LLM 服务
func CreateLLMService(config *LLMConfig) (LLMService, error) {
	switch config.Provider {
	case "openai":
		return NewOpenAILLMService(config)
	case "ollama":
		return NewOllamaLLMService(config)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}

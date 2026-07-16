package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ChatService 对话模型服务接口
type ChatService interface {
	// Chat 进行对话
	Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error)

	// GetProviderInfo 获取提供者信息
	GetProviderInfo() ProviderInfo
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model       string            `json:"model"`
	Messages    []ChatMessage     `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Format      string            `json:"format,omitempty"` // json, text
}

// ChatMessage 对话消息（支持多模态内容）
type ChatMessage struct {
	Role    string         `json:"role"`    // system, user, assistant
	Content MessageContent `json:"content"` // 支持字符串或数组格式
}

// MessageContent 消息内容（支持字符串或数组格式）
type MessageContent struct {
	value interface{} // 存储原始值：string 或 []interface{}
}

// NewMessageContent 创建新的MessageContent
func NewMessageContent(content interface{}) MessageContent {
	return MessageContent{value: content}
}

// UnmarshalJSON 自定义JSON解析，支持字符串和数组
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		mc.value = str
		return nil
	}

	// 尝试解析为数组（多模态内容）
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		mc.value = arr
		return nil
	}

	return fmt.Errorf("content must be string or array")
}

// MarshalJSON 自定义JSON序列化
func (mc MessageContent) MarshalJSON() ([]byte, error) {
	if mc.value == nil {
		return json.Marshal("")
	}
	return json.Marshal(mc.value)
}

// String 获取字符串表示
func (mc MessageContent) String() string {
	if mc.value == nil {
		return ""
	}

	switch v := mc.value.(type) {
	case string:
		return v
	case []interface{}:
		// 从数组中提取文本内容
		var texts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ChatResponse 对话响应
type ChatResponse struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
	TokensUsed   int    `json:"tokens_used"`
	RawResponse  string `json:"raw_response,omitempty"`
}

// ProviderInfo 提供者信息
type ProviderInfo struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"`
}

// ChatConfig 对话模型配置
type ChatConfig struct {
	Provider    string  `json:"provider"`  // ollama, openai
	Model       string  `json:"model"`
	BaseURL     string  `json:"base_url"`
	APIKey      string  `json:"api_key"`
	Timeout     int     `json:"timeout"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Enabled     bool    `json:"enabled"`
}

// DefaultChatConfig 获取默认对话配置
func DefaultChatConfig() *ChatConfig {
	return &ChatConfig{
		Provider:    "ollama",
		Model:       "llama3.2:3b",
		BaseURL:     "http://localhost:21434",
		APIKey:      "",
		Timeout:     30,
		Temperature: 0.3,
		MaxTokens:   2000,
		Enabled:     true,
	}
}

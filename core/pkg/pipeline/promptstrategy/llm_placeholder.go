package promptstrategy

import "context"

// PromptLLMProcessor LLM 处理器接口（Phase A 占位）
type PromptLLMProcessor interface {
	// Name 处理器名称
	Name() string
	// Process 处理 prompt
	Process(ctx context.Context, req PromptLLMRequest) (PromptLLMResponse, error)
}

// PromptLLMRequest LLM 处理请求
type PromptLLMRequest struct {
	// Stage 阶段：user_check | user_optimize | output_post
	Stage string
	// Text 待处理文本
	Text string
	// Config 配置
	Config map[string]any
}

// PromptLLMResponse LLM 处理响应
type PromptLLMResponse struct {
	// Text 处理后文本
	Text string
	// Metadata 元数据
	Metadata map[string]any
}

// NullPromptLLMProcessor 空 LLM 处理器（Phase A 默认）
type NullPromptLLMProcessor struct{}

// NewNullPromptLLMProcessor 创建空 LLM 处理器
func NewNullPromptLLMProcessor() *NullPromptLLMProcessor {
	return &NullPromptLLMProcessor{}
}

// Name 返回处理器名称
func (n *NullPromptLLMProcessor) Name() string {
	return "null"
}

// Process 空处理器：直接透传
func (n *NullPromptLLMProcessor) Process(_ context.Context, req PromptLLMRequest) (PromptLLMResponse, error) {
	return PromptLLMResponse{
		Text:     req.Text,
		Metadata: map[string]any{"processor": "null"},
	}, nil
}

// LLMConfig LLM 配置（占位）
type LLMConfig struct {
	Enabled    bool   `json:"enabled"`
	Capability string `json:"capability,omitempty"`
	Backend    string `json:"backend,omitempty"`
	Model      string `json:"model,omitempty"`
	Mode       string `json:"mode,omitempty"`
}

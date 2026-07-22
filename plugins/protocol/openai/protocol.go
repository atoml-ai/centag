package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// MessageContent 消息内容(支持字符串或数组)
type MessageContent struct {
	Type     string      `json:"type,omitempty"` // text, image_url等
	Text     string      `json:"text,omitempty"`
	ImageURL interface{} `json:"image_url,omitempty"`
	Value    interface{} `json:"-"` // 用于存储原始值
}

// UnmarshalJSON 自定义JSON解析
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		mc.Value = str
		return nil
	}

	// 尝试解析为数组
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		mc.Value = arr
		return nil
	}

	// 尝试解析为对象
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		mc.Value = obj
		return nil
	}

	return fmt.Errorf("invalid message content format")
}

// MarshalJSON 自定义JSON序列化
func (mc *MessageContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(mc.Value)
}

// String 返回内容的字符串表示
func (mc *MessageContent) String() string {
	if mc.Value == nil {
		return ""
	}

	// 如果是字符串
	if str, ok := mc.Value.(string); ok {
		return str
	}

	// 如果是数组,提取文本内容；非文本内容（image_url/audio 等）以占位标记保留，避免静默丢弃
	if arr, ok := mc.Value.([]map[string]interface{}); ok {
		result := ""
		for _, item := range arr {
			typ, _ := item["type"].(string)
			switch typ {
			case "text":
				if text, ok := item["text"].(string); ok {
					result += text
				}
			case "image_url":
				result += "[image]"
			case "input_audio", "audio":
				result += "[audio]"
			case "file":
				result += "[file]"
			}
		}
		return result
	}

	// 如果是对象,尝试提取文本
	if obj, ok := mc.Value.(map[string]interface{}); ok {
		if typ, ok := obj["type"].(string); ok && typ == "text" {
			if text, ok := obj["text"].(string); ok {
				return text
			}
		}
	}

	return fmt.Sprintf("%v", mc.Value)
}

// Protocol OpenAI 协议插件
type Protocol struct {
	name   string
	status plugin.PluginStatus
}

// NewProtocol 创建 OpenAI 协议插件
func NewProtocol() (plugin.Plugin, error) {
	return &Protocol{
		name:   "openai-protocol",
		status: plugin.StatusStopped,
	}, nil
}

// Name 返回插件名称
func (p *Protocol) Name() string {
	return p.name
}

// Type 返回插件类型
func (p *Protocol) Type() plugin.PluginType {
	return plugin.TypeProtocol
}

// Version 返回插件版本
func (p *Protocol) Version() string {
	return "1.0.0"
}

// Init 初始化插件
func (p *Protocol) Init(config any) error {
	p.status = plugin.StatusStopped
	log.Printf("[OpenAI Protocol] Plugin initialized")
	return nil
}

// Start 启动插件
func (p *Protocol) Start(ctx context.Context) error {
	p.status = plugin.StatusRunning
	log.Printf("[OpenAI Protocol] Plugin started")
	return nil
}

// Stop 停止插件
func (p *Protocol) Stop(ctx context.Context) error {
	p.status = plugin.StatusStopped
	log.Printf("[OpenAI Protocol] Plugin stopped")
	return nil
}

// Status 返回插件状态
func (p *Protocol) Status() plugin.PluginStatus {
	return p.status
}

// ParseRequest 解析 OpenAI 请求
func (p *Protocol) ParseRequest(c *gin.Context) (*plugin.ProxyRequest, error) {
	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	// 先按原始对象解析，保留 scripts/tools/tool_choice 等未建模字段用于透明透传。
	var rawBody map[string]interface{}
	if err := json.Unmarshal(body, &rawBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw request: %w", err)
	}

	// 解析 OpenAI ChatCompletion 请求
	var openaiReq ChatCompletionRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// 转换为统一的 ProxyRequest
	req := &plugin.ProxyRequest{
		Messages:         convertMessages(openaiReq.Messages),
		Model:            openaiReq.Model,
		Stream:           openaiReq.Stream,
		Temperature:      openaiReq.Temperature,
		MaxTokens:        openaiReq.MaxTokens,
		TopP:             openaiReq.TopP,
		FrequencyPenalty: openaiReq.FrequencyPenalty,
		PresencePenalty:  openaiReq.PresencePenalty,
		Stop:             openaiReq.Stop,
		Metadata:         make(map[string]interface{}),
		RawBody:          rawBody,
		Headers:          make(map[string]string),

		// [v0.2.8 协议对齐] 显式字段映射（与 §3.1.1 修订 L2 一致）
		Tools:             openaiReq.Tools,
		ToolChoice:        openaiReq.ToolChoice,
		ResponseFormat:    openaiReq.ResponseFormat,
		Seed:              openaiReq.Seed,
		N:                 openaiReq.N,
		User:              openaiReq.User,
		ParallelToolCalls: openaiReq.ParallelToolCalls,
	}

	// reasoning_effort → ReasoningSpec（显式指定标记）
	if openaiReq.ReasoningEffort != "" {
		req.Reasoning.Effort = openaiReq.ReasoningEffort
		req.Reasoning.Specified = true
	}

	// 无显式字段承载的边缘 key 进 Metadata（RawBody 仍含原值供后端透传）
	if openaiReq.ServiceTier != "" {
		req.Metadata["service_tier"] = openaiReq.ServiceTier
	}
	if openaiReq.Store != nil {
		req.Metadata["store"] = *openaiReq.Store
	}
	if len(openaiReq.Metadata) > 0 {
		req.Metadata["client_metadata"] = openaiReq.Metadata
	}

	// [G7] Modalities / Audio / Logprobs / TopLogprobs 为 P2 占位，本轮不映射，仅 RawBody 透传

	// 收集请求头
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			req.Headers[k] = v[0]
		}
	}

	return req, nil
}

// HandleResponse 处理响应并返回给客户端
func (p *Protocol) HandleResponse(c *gin.Context, resp *plugin.ProxyResponse) error {
	// 如果有错误,返回错误响应
	if resp.Error != nil {
		errBody := gin.H{
			"message": resp.Error.Message,
			"type":    resp.Error.Type,
			"code":    resp.Error.Code,
		}
		// [v0.2.8] OpenAI 标准 param 字段，空值省略
		if resp.Error.Param != "" {
			errBody["param"] = resp.Error.Param
		}
		c.JSON(500, gin.H{"error": errBody})
		return nil
	}

	// 构建 OpenAI 格式的消息
	message := Message{
		Role: "assistant",
	}

	// 设置内容
	if resp.Content != "" {
		message.Content = MessageContent{Value: resp.Content}
	}

	// [v0.2.8] 模型拒绝时填充 refusal
	if refusal, ok := resp.Metadata["refusal"].(string); ok && refusal != "" {
		message.Refusal = refusal
	}

	// 设置工具调用(如果有)
	if len(resp.ToolCalls) > 0 {
		toolCalls := make([]ToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			toolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
		message.ToolCalls = toolCalls
	}

	// 如果finish_reason是tool_calls,使用这个值
	finishReason := resp.FinishReason
	if finishReason == "tool_calls" || len(resp.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}

	// [v0.2.8 G5] usage 修正：TotalTokens = PromptTokens + CompletionTokens，禁止 total == completion 单值
	promptTokens := 0
	if pt, ok := resp.Metadata["prompt_tokens"].(int); ok {
		promptTokens = pt
	}
	completionTokens := resp.TokensUsed
	usage := Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}

	// [v0.2.8] usage 明细（从后端响应 Metadata 提取，无则省略，不伪造数据）
	if details, ok := resp.Metadata["prompt_tokens_details"].(*TokenDetails); ok && details != nil {
		usage.PromptTokensDetails = details
	}
	if details, ok := resp.Metadata["completion_tokens_details"].(*CompletionTokenDetails); ok && details != nil {
		usage.CompletionTokensDetails = details
	}

	// 转换为 OpenAI 格式的响应
	openaiResp := ChatCompletionResponse{
		ID:      "chatcmpl-" + generateID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(), // [v0.2.8] 真实时间戳
		Model:   resp.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}

	// [v0.2.8] system_fingerprint / service_tier：从后端响应提取，无则省略
	if sfp, ok := resp.Metadata["system_fingerprint"].(string); ok && sfp != "" {
		openaiResp.SystemFingerprint = sfp
	}
	if st, ok := resp.Metadata["service_tier"].(string); ok && st != "" {
		openaiResp.ServiceTier = st
	}

	c.JSON(200, openaiResp)
	return nil
}

// SupportStream 是否支持流式响应
func (p *Protocol) SupportStream() bool {
	return true
}

// GetModels 获取支持的模型列表
func (p *Protocol) GetModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{
			ID:      "gpt-4",
			Name:    "GPT-4",
			Enabled: true,
		},
		{
			ID:      "gpt-4-turbo",
			Name:    "GPT-4 Turbo",
			Enabled: true,
		},
		{
			ID:      "qwen/qwen3-4b-fp8",
			Name:    "GPT-3.5 Turbo",
			Enabled: true,
		},
	}, nil
}

// ValidateRequest 验证请求
func (p *Protocol) ValidateRequest(req *plugin.ProxyRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("messages is required")
	}

	return nil
}

// FormatStreamChunk 将内部 StreamChunk 格式化为 OpenAI SSE data 行
func (p *Protocol) FormatStreamChunk(model string, chunk *plugin.StreamChunk, chunkIndex int) string {
	if chunk == nil {
		return ""
	}

	// [v0.2.8 G6] 流式结束时的 usage 事件（OpenAI 2024-10 官方行为）：
	// 判据为 UsagePromptTokens>0 || UsageCompletionTokens>0（非 TokensUsed>0），
	// prompt=0 且 completion>0 时仍须发送；两者皆为 0 时不发，仅 [DONE]。
	if chunk.Done && chunk.Content == "" {
		if chunk.UsagePromptTokens > 0 || chunk.UsageCompletionTokens > 0 {
			usageData := map[string]interface{}{
				"id":      "chatcmpl-" + generateID(),
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   model,
				"choices": []interface{}{},
				"usage": map[string]interface{}{
					"prompt_tokens":     chunk.UsagePromptTokens,
					"completion_tokens": chunk.UsageCompletionTokens,
					"total_tokens":      chunk.UsagePromptTokens + chunk.UsageCompletionTokens,
				},
			}
			dataBytes, _ := json.Marshal(usageData)
			return string(dataBytes)
		}
		return ""
	}

	delta := map[string]interface{}{}
	if chunk.Content != "" {
		delta["content"] = chunk.Content
	}
	// [v0.2.8] delta 扩展：reasoning_content / refusal / tool_calls 增量
	if chunk.ReasoningContent != "" {
		delta["reasoning_content"] = chunk.ReasoningContent
	}
	if refusal, ok := chunkRefusal(chunk); ok {
		delta["refusal"] = refusal
	}
	if len(chunk.ToolCalls) > 0 {
		toolCalls := make([]map[string]interface{}, len(chunk.ToolCalls))
		for i, tc := range chunk.ToolCalls {
			toolCalls[i] = map[string]interface{}{
				"index": i,
				"id":    tc.ID,
				"type":  "function",
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		delta["tool_calls"] = toolCalls
	}

	choice := map[string]interface{}{
		"index": 0,
		"delta": delta,
	}
	if chunkIndex == 0 {
		delta["role"] = "assistant"
	}
	if chunk.FinishReason != "" {
		choice["finish_reason"] = chunk.FinishReason
	}

	chunkData := map[string]interface{}{
		"id":      "chatcmpl-" + generateID(),
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   model,
		"choices": []interface{}{choice},
	}

	dataBytes, _ := json.Marshal(chunkData)
	return string(dataBytes)
}

// chunkRefusal 提取 chunk 的 refusal 内容（当前 StreamChunk 无显式字段，预留扩展点）
func chunkRefusal(_ *plugin.StreamChunk) (string, bool) {
	return "", false
}

// FormatStreamDone 返回 OpenAI 流结束标记
func (p *Protocol) FormatStreamDone() string {
	return "[DONE]"
}

// ChatCompletionRequest OpenAI ChatCompletion 请求
type ChatCompletionRequest struct {
	Messages         []Message `json:"messages"`
	Model            string    `json:"model"`
	Stream           bool      `json:"stream,omitempty"`
	Temperature      float64   `json:"temperature,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
	Stop             []string  `json:"stop,omitempty"`

	// [v0.2.8 协议对齐] 显式映射字段（ParseRequest 映射到 ProxyRequest 显式字段）
	Tools             []plugin.ToolDefinition    `json:"tools,omitempty"`
	ToolChoice        interface{}                `json:"tool_choice,omitempty"`
	ResponseFormat    *plugin.ResponseFormatSpec `json:"response_format,omitempty"`
	Seed              *int                       `json:"seed,omitempty"`
	N                 *int                       `json:"n,omitempty"`
	User              string                     `json:"user,omitempty"`
	ParallelToolCalls *bool                      `json:"parallel_tool_calls,omitempty"`
	ReasoningEffort   string                     `json:"reasoning_effort,omitempty"`
	ServiceTier       string                     `json:"service_tier,omitempty"`
	Store             *bool                      `json:"store,omitempty"`
	Metadata          map[string]string          `json:"metadata,omitempty"`

	// [v0.2.8 协议对齐] P2 占位：本轮不映射，仅经 RawBody 透传
	Modalities  []string    `json:"modalities,omitempty"`
	Audio       interface{} `json:"audio,omitempty"`
	Logprobs    *bool       `json:"logprobs,omitempty"`
	TopLogprobs *int        `json:"top_logprobs,omitempty"`
}

// ChatCompletionResponse OpenAI ChatCompletion 响应
type ChatCompletionResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"` // [v0.2.8]
	ServiceTier       string   `json:"service_tier,omitempty"`       // [v0.2.8]
}

// Choice 选择项
type Choice struct {
	Index        int             `json:"index"`
	Message      Message         `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"` // [v0.2.8] P2 占位
}

// ChoiceLogprobs choice 级别 logprobs（P2 占位）
type ChoiceLogprobs struct {
	Content []TokenLogprob `json:"content,omitempty"`
}

// TokenLogprob 单 token logprob（P2 占位）
type TokenLogprob struct {
	Token       string           `json:"token"`
	Logprob     float64          `json:"logprob"`
	Bytes       []int            `json:"bytes,omitempty"`
	TopLogprobs []TopTokenLogprob `json:"top_logprobs,omitempty"`
}

// TopTokenLogprob top_logprobs 条目（P2 占位）
type TopTokenLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message 消息
type Message struct {
	Role             string         `json:"role"`
	Content          MessageContent `json:"content,omitempty"`
	ToolCalls        []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	Name             string         `json:"name,omitempty"`    // [v0.2.8] function/tool 角色标识调用者
	Refusal          string         `json:"refusal,omitempty"` // [v0.2.8] 模型拒绝时填充
}

// Usage 使用量
type Usage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	TotalTokens             int                     `json:"total_tokens"`
	PromptTokensDetails     *TokenDetails           `json:"prompt_tokens_details,omitempty"`     // [v0.2.8]
	CompletionTokensDetails *CompletionTokenDetails `json:"completion_tokens_details,omitempty"` // [v0.2.8]
}

// TokenDetails prompt 维度 token 明细
type TokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
}

// CompletionTokenDetails completion 维度 token 明细
type CompletionTokenDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

// convertMessages 转换消息格式
func convertMessages(messages []Message) []plugin.Message {
	result := make([]plugin.Message, len(messages))
	for i, msg := range messages {
		var toolCalls []plugin.ToolCall
		if len(msg.ToolCalls) > 0 {
			toolCalls = make([]plugin.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCalls[j] = plugin.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: plugin.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		result[i] = plugin.Message{
			Role:             msg.Role,
			Content:          msg.Content.String(),
			ToolCalls:        toolCalls,
			ToolCallID:       msg.ToolCallID,
			ReasoningContent: msg.ReasoningContent,
		}
	}
	return result
}

// generateID 生成简单的ID
func generateID() string {
	return "proxy-" + randomString(16)
}

// randomString 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

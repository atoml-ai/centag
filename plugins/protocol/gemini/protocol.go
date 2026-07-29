//go:build protocol_gemini

package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// Protocol Gemini 协议插件
type Protocol struct {
	name   string
	status plugin.PluginStatus
}

// NewProtocol 创建 Gemini 协议插件
func NewProtocol() (plugin.Plugin, error) {
	return &Protocol{
		name:   "gemini-protocol",
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
	log.Printf("[Gemini Protocol] Plugin initialized")
	return nil
}

// Start 启动插件
func (p *Protocol) Start(ctx context.Context) error {
	p.status = plugin.StatusRunning
	log.Printf("[Gemini Protocol] Plugin started")
	return nil
}

// Stop 停止插件
func (p *Protocol) Stop(ctx context.Context) error {
	p.status = plugin.StatusStopped
	log.Printf("[Gemini Protocol] Plugin stopped")
	return nil
}

// Status 返回插件状态
func (p *Protocol) Status() plugin.PluginStatus {
	return p.status
}

// ParseRequest 解析 Gemini 请求为统一的 ProxyRequest
func (p *Protocol) ParseRequest(c *gin.Context) (*plugin.ProxyRequest, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var geminiReq geminiRequest
	if err := json.Unmarshal(body, &geminiReq); err != nil {
		return nil, fmt.Errorf("failed to parse gemini request: %w", err)
	}

	req := &plugin.ProxyRequest{
		Model:    extractModelFromPath(c.Request.URL.Path),
		Stream:   c.Query("alt") == "sse",
		Messages: convertContentsToMessages(geminiReq.Contents),
		RawBody:  body,
		Headers:  make(map[string]string),
		Metadata: map[string]any{
			"protocol":          "gemini",
			"systemInstruction": geminiReq.SystemInstruction,
			"generationConfig":  geminiReq.GenerationConfig,
		},
	}

	if geminiReq.SystemInstruction != nil {
		var sysText string
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != "" {
				sysText += part.Text
			}
		}
		if sysText != "" {
			req.System = sysText
		}
	}

	if geminiReq.GenerationConfig != nil {
		if geminiReq.GenerationConfig.Temperature != nil {
			req.Temperature = *geminiReq.GenerationConfig.Temperature
		}
		if geminiReq.GenerationConfig.MaxOutputTokens != nil {
			req.MaxTokens = *geminiReq.GenerationConfig.MaxOutputTokens
		}
		if geminiReq.GenerationConfig.TopP != nil {
			req.TopP = *geminiReq.GenerationConfig.TopP
		}
	}

	for k, v := range c.Request.Header {
		if len(v) > 0 {
			req.Headers[k] = v[0]
		}
	}

	return req, nil
}

// HandleResponse 处理 ProxyResponse 为 Gemini 格式
func (p *Protocol) HandleResponse(c *gin.Context, resp *plugin.ProxyResponse) error {
	finishReason := resp.FinishReason
	if finishReason == "" {
		finishReason = "STOP"
	}
	geminiResp := geminiResponse{
		Candidates: []candidate{
			{
				Index: 0,
				Content: content{
					Role:  "model",
					Parts: []part{{Text: resp.Content}},
				},
				FinishReason: finishReason,
			},
		},
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, geminiResp)
	return nil
}

// FormatStreamChunk 将 StreamChunk 格式化为 Gemini SSE 格式
func (p *Protocol) FormatStreamChunk(model string, chunk *plugin.StreamChunk, chunkIndex int) string {
	if chunk.Content == "" && chunk.ReasoningContent == "" {
		return ""
	}

	finishReason := chunk.FinishReason
	if finishReason == "" && chunkIndex > 0 {
		// Only the final chunk typically carries a finish reason; leave earlier chunks empty.
	}
	streamChunk := geminiStreamChunk{
		Candidates: []candidate{
			{
				Index: 0,
				Content: content{
					Role:  "model",
					Parts: []part{{Text: chunk.Content}},
				},
				FinishReason: finishReason,
			},
		},
	}

	data, _ := json.Marshal(streamChunk)
	return string(data)
}

// FormatStreamDone 返回流结束标记
func (p *Protocol) FormatStreamDone() string {
	return "[DONE]"
}

// SupportStream 是否支持流式响应
func (p *Protocol) SupportStream() bool {
	return true
}

// GetModels 获取支持的模型列表
func (p *Protocol) GetModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{ID: "gemini-pro", Name: "Gemini Pro", Enabled: true},
		{ID: "gemini-pro-vision", Name: "Gemini Pro Vision", Enabled: true},
	}, nil
}

// ValidateRequest 验证请求是否有效
func (p *Protocol) ValidateRequest(req *plugin.ProxyRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// 辅助函数

func extractModelFromPath(path string) string {
	// 从路径中提取模型名称
	// 格式: /v1beta/models/{model}:generateContent
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			model := parts[i+1]
			// 移除 :generateContent 等后缀
			if idx := strings.Index(model, ":"); idx != -1 {
				model = model[:idx]
			}
			return model
		}
	}
	return ""
}

func convertContentsToMessages(contents []content) []plugin.Message {
	messages := make([]plugin.Message, 0, len(contents))
	for _, c := range contents {
		role := c.Role
		if role == "model" {
			role = "assistant"
		}

		text := ""
		for _, part := range c.Parts {
			if part.Text != "" {
				text += part.Text
			}
		}

		messages = append(messages, plugin.Message{
			Role:    role,
			Content: text,
		})
	}
	return messages
}

func init() {
	plugin.RegisterProtocol("gemini", func(config map[string]interface{}) (interface{}, error) {
		return NewProtocol()
	})
}

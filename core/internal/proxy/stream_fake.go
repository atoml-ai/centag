package proxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// StreamFakeConfig Stream Fake 配置
type StreamFakeConfig struct {
	Enabled   bool
	MaxBytes  int64
}

// DefaultStreamFakeConfig 返回默认配置，受环境变量控制：
//
//	CENTAG_STREAM_FAKE（兼容 PROXYCLAW_STREAM_FAKE）— 设为 0/false/off 关闭（默认 true）
//	CENTAG_STREAM_FAKE_MAX_BYTES（兼容 PROXYCLAW_*）— 最大聚合字节数（默认 32MB）
func DefaultStreamFakeConfig() StreamFakeConfig {
	cfg := StreamFakeConfig{
		Enabled:  true,
		MaxBytes: 32 * 1024 * 1024, // 32MB
	}

	if v := firstEnv("CENTAG_STREAM_FAKE", "PROXYCLAW_STREAM_FAKE"); v != "" {
		cfg.Enabled = !isEnvFalse(v)
	}

	if v := firstEnv("CENTAG_STREAM_FAKE_MAX_BYTES", "PROXYCLAW_STREAM_FAKE_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.MaxBytes = n
		}
	}

	return cfg
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func isEnvFalse(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "off", "no", "":
		return true
	}
	return false
}

// StreamFakeAggregator 流式聚合器
type StreamFakeAggregator struct {
	maxBytes         int64
	totalBytes       int64
	content          strings.Builder
	reasoningContent strings.Builder
	toolCalls        []plugin.ToolCall
	finishReason     string
}

// NewStreamFakeAggregator 创建新的聚合器
func NewStreamFakeAggregator(maxBytes int64) *StreamFakeAggregator {
	return &StreamFakeAggregator{
		maxBytes: maxBytes,
	}
}

// Feed 处理一个流式 chunk
func (a *StreamFakeAggregator) Feed(chunk plugin.StreamChunk) error {
	// 检查大小限制
	chunkSize := int64(len(chunk.Content) + len(chunk.ReasoningContent))
	a.totalBytes += chunkSize
	if a.maxBytes > 0 && a.totalBytes > a.maxBytes {
		return fmt.Errorf("stream fake response exceeded max bytes limit: %d > %d", a.totalBytes, a.maxBytes)
	}

	// 累积内容
	if chunk.Content != "" {
		a.content.WriteString(chunk.Content)
	}
	if chunk.ReasoningContent != "" {
		a.reasoningContent.WriteString(chunk.ReasoningContent)
	}

	// 处理工具调用
	if len(chunk.ToolCalls) > 0 {
		a.toolCalls = chunk.ToolCalls
	}

	// 记录完成原因
	if chunk.FinishReason != "" {
		a.finishReason = chunk.FinishReason
	}

	return nil
}

// Result 聚合结果
type StreamFakeResult struct {
	Content          string
	ReasoningContent string
	ToolCalls        []plugin.ToolCall
	FinishReason     string
}

// Result 返回聚合结果
func (a *StreamFakeAggregator) Result() StreamFakeResult {
	return StreamFakeResult{
		Content:          a.content.String(),
		ReasoningContent: a.reasoningContent.String(),
		ToolCalls:        a.toolCalls,
		FinishReason:     a.finishReason,
	}
}

// StreamFakeHandler Stream Fake 处理器
type StreamFakeHandler struct {
	config StreamFakeConfig
}

// NewStreamFakeHandler 创建新的 Stream Fake 处理器
func NewStreamFakeHandler(config StreamFakeConfig) *StreamFakeHandler {
	return &StreamFakeHandler{
		config: config,
	}
}

// IsEnabled 检查 Stream Fake 是否启用
func (h *StreamFakeHandler) IsEnabled() bool {
	return h.config.Enabled
}

// HandleStreamFake 处理 Stream Fake 逻辑
// 将非流式请求转换为流式请求，聚合后再返回非流式响应
func (h *StreamFakeHandler) HandleStreamFake(
	c *gin.Context,
	req *plugin.ProxyRequest,
	callModel func(ctx context.Context, req *plugin.ProxyRequest) (<-chan plugin.StreamChunk, error),
) error {
	// 保存原始 stream 状态并设置为 true
	originalStream := req.Stream
	req.Stream = true

	// 调用模型获取流式响应
	streamCh, err := callModel(c.Request.Context(), req)
	if err != nil {
		req.Stream = originalStream
		return fmt.Errorf("stream fake call model failed: %w", err)
	}

	// 创建聚合器
	aggregator := NewStreamFakeAggregator(h.config.MaxBytes)

	// 聚合所有 chunk
	for chunk := range streamCh {
		if chunk.Error != nil {
			req.Stream = originalStream
			return fmt.Errorf("stream fake chunk error: %w", chunk.Error)
		}

		if err := aggregator.Feed(chunk); err != nil {
			req.Stream = originalStream
			return err
		}
	}

	// 恢复原始 stream 状态
	req.Stream = originalStream

	// 构建非流式响应
	result := aggregator.Result()
	response := &plugin.ProxyResponse{
		Content:          result.Content,
		ReasoningContent: result.ReasoningContent,
		ToolCalls:        result.ToolCalls,
		FinishReason:     result.FinishReason,
		Model:            req.Model,
	}

	// 设置响应头
	c.Header("Content-Type", "application/json")

	// 返回非流式响应
	c.JSON(http.StatusOK, response)
	return nil
}

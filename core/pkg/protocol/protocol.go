// Package protocol 定义协议解析插件接口。
//
// 协议插件负责将客户端特定协议请求（如 OpenAI Chat Completions、Anthropic Messages）
// 解析为框架统一格式，并将统一格式响应编码回客户端协议格式。
//
// 实现方通过 init() 调用 Register() 注册工厂函数：
//
//	func init() {
//	    protocol.Register("openai", NewOpenAIProtocol)
//	}
package protocol

import (
	"net/http"

	"centag/core/pkg/types"
)

// Protocol 协议解析插件接口。
//
// 每个协议（OpenAI、Anthropic 等）实现此接口，
// 框架通过 DecodeRequest / EncodeResponse 完成协议翻译。
type Protocol interface {
	// Name 返回协议名称（如 "openai"、"anthropic"）。
	Name() string

	// Version 返回协议插件版本号。
	Version() string

	// DecodeRequest 将客户端 HTTP 请求解析为统一格式。
	//
	// 实现方应：
	//   - 从 http.Request 读取 Body 并解析为 UnifiedRequest
	//   - 设置 Content-Type、Accept 等响应头
	//   - 处理流式请求标记（req.Stream）
	//
	// 返回错误时，框架会返回适当的 HTTP 错误响应。
	DecodeRequest(r *http.Request) (*types.UnifiedRequest, error)

	// EncodeResponse 将统一格式响应编码为客户端协议格式并写入 http.ResponseWriter。
	//
	// 实现方应设置正确的 Content-Type 头并写入 JSON 响应体。
	EncodeResponse(w http.ResponseWriter, resp *types.UnifiedResponse) error

	// EncodeStreamResponse 将统一格式流式响应编码为客户端协议格式。
	//
	// 实现方应：
	//   - 设置 Transfer-Encoding: chunked 和正确的 Content-Type
	//   - 从 ch 读取 UnifiedChunk 并编码为客户端协议的 SSE 格式
	//   - 当 ch 关闭时完成响应
	EncodeStreamResponse(w http.ResponseWriter, ch <-chan *types.UnifiedChunk) error

	// SupportStream 是否支持流式响应。
	SupportStream() bool
}

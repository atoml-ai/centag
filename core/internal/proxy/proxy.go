package proxy

import (
	"fmt"
	"strings"

	"centag/core/pkg/plugin"
)

// Proxy 代理服务
type Proxy struct {
	pluginManager *plugin.Manager
}

// New 创建代理服务
func New(pluginManager *plugin.Manager) *Proxy {
	return &Proxy{
		pluginManager: pluginManager,
	}
}

// HandleRequest 处理代理请求
func (p *Proxy) HandleRequest(ctx *plugin.ProxyContext, protocolPlugin string, backendPlugin string, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	// 获取后端插件
	backend, err := p.pluginManager.GetBackend(backendPlugin)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend plugin: %w", err)
	}

	// 调用后端模型
	var resp *plugin.ProxyResponse
	if req.Stream {
		// 流式处理
		chunks, err := backend.CallModelStream(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to call model stream: %w", err)
		}

		// 合并流式响应
		var content strings.Builder
		tokensUsed := 0
		finishReason := ""

		for chunk := range chunks {
			if chunk.Error != nil {
				return nil, chunk.Error
			}
			content.WriteString(chunk.Content)
			if chunk.TokensUsed > 0 {
				tokensUsed = chunk.TokensUsed
			}
			if chunk.FinishReason != "" {
				finishReason = chunk.FinishReason
			}
			if chunk.Done {
				break
			}
		}

		resp = &plugin.ProxyResponse{
			Content:      content.String(),
			TokensUsed:   tokensUsed,
			FinishReason: finishReason,
			Model:        req.Model,
		}
	} else {
		// 非流式处理
		resp, err = backend.CallModel(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to call model: %w", err)
		}
	}

	return resp, nil
}

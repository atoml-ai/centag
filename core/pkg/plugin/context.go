package plugin

import (
	"context"
	"time"
)

// ProxyContext 代理上下文
type ProxyContext struct {
	context.Context

	// 请求ID
	RequestID string

	// 时间戳
	StartTime time.Time

	// 元数据
	Metadata map[string]any

	// 插件间共享数据
	Shared map[string]any
}

// NewProxyContext 创建代理上下文
func NewProxyContext(ctx context.Context) *ProxyContext {
	return &ProxyContext{
		Context:   ctx,
		StartTime: time.Now(),
		Metadata:  make(map[string]any),
		Shared:    make(map[string]any),
	}
}

// WithRequestID 设置请求ID
func (c *ProxyContext) WithRequestID(requestID string) *ProxyContext {
	c.RequestID = requestID
	c.Metadata["request_id"] = requestID
	return c
}

// GetRequestID 获取请求ID
func (c *ProxyContext) GetRequestID() string {
	return c.RequestID
}

// WithMetadata 设置元数据
func (c *ProxyContext) WithMetadata(key string, value any) *ProxyContext {
	c.Metadata[key] = value
	return c
}

// GetMetadata 获取元数据
func (c *ProxyContext) GetMetadata(key string) (any, bool) {
	val, ok := c.Metadata[key]
	return val, ok
}

// WithShared 设置共享数据
func (c *ProxyContext) WithShared(key string, value any) *ProxyContext {
	c.Shared[key] = value
	return c
}

// GetShared 获取共享数据
func (c *ProxyContext) GetShared(key string) (any, bool) {
	val, ok := c.Shared[key]
	return val, ok
}

// Elapsed 获取已用时间
func (c *ProxyContext) Elapsed() time.Duration {
	return time.Since(c.StartTime)
}

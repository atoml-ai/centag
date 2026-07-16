package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// RequestConfig HTTP 请求配置
type RequestConfig struct {
	Method      string
	URL         string
	Body        []byte
	Headers     map[string]string
	Timeout     time.Duration
	SkipHeaders []string // 不应转发的 headers
}

// Response HTTP 响应
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Client HTTP 客户端包装器
type Client struct {
	httpClient *http.Client
}

// NewClient 创建 HTTP 客户端
func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewClientNoProxy 创建不走系统代理的 HTTP 客户端
// 用于模型探测等需要直连远端的场景，避免 http_proxy/https_proxy 环境变量干扰
func NewClientNoProxy(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: func(*http.Request) (*url.URL, error) {
					return nil, nil // 不使用任何代理，直连目标
				},
			},
		},
	}
}

// Do 执行 HTTP 请求
func (c *Client) Do(ctx context.Context, config *RequestConfig) (*Response, error) {
	var bodyReader io.Reader
	if config.Body != nil {
		bodyReader = bytes.NewReader(config.Body)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 headers
	for k, v := range config.Headers {
		if !shouldSkipHeader(k, config.SkipHeaders) {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// DoWithHeaders 执行 HTTP 请求并从原始请求头中复制
func (c *Client) DoWithHeaders(ctx context.Context, config *RequestConfig, originalHeaders http.Header) (*Response, error) {
	var bodyReader io.Reader
	if config.Body != nil {
		bodyReader = bytes.NewReader(config.Body)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 复制原始请求头
	for k, v := range originalHeaders {
		if !shouldSkipHeader(k, config.SkipHeaders) {
			req.Header[k] = v
		}
	}

	// 设置额外的 headers
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// shouldSkipHeader 判断是否应该跳过某个 header
func shouldSkipHeader(header string, skipHeaders []string) bool {
	for _, skip := range skipHeaders {
		if header == skip {
			return true
		}
	}
	return false
}

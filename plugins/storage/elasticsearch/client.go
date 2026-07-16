package elasticsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client Elasticsearch 客户端
type Client struct {
	client       *http.Client
	addresses    []string
	username     string
	password     string
	apiKey       string
	headers      map[string]string
	requestTimeout time.Duration
}

// NewClient 创建 Elasticsearch 客户端
func NewClient(config *Config) (*Client, error) {
	// 创建 HTTP 客户端
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.EnableTLS,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(config.RequestTimeout) * time.Second,
	}

	esClient := &Client{
		client:        client,
		addresses:     config.Addresses,
		username:      config.Username,
		password:      config.Password,
		apiKey:        config.APIKey,
		requestTimeout: time.Duration(config.RequestTimeout) * time.Second,
		headers:       make(map[string]string),
	}

	// 设置默认 headers
	esClient.headers["Content-Type"] = "application/json"

	// 设置认证
	if config.APIKey != "" {
		esClient.headers["Authorization"] = "ApiKey " + config.APIKey
	} else if config.Username != "" && config.Password != "" {
		esClient.headers["Authorization"] = "Basic " + basicAuth(config.Username, config.Password)
	}

	return esClient, nil
}

// GetClusterHealth 获取集群健康状态
func (c *Client) GetClusterHealth(ctx context.Context) (*ClusterHealthInfo, error) {
	path := "/_cluster/health"

	var result ClusterHealthInfo
	if err := c.request(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// IndexExists 检查索引是否存在
func (c *Client) IndexExists(ctx context.Context, index string) (bool, error) {
	path := "/" + index

	var result map[string]interface{}
	err := c.request(ctx, "HEAD", path, nil, &result)
	if err != nil {
		// 404 表示索引不存在
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// CreateIndex 创建索引
func (c *Client) CreateIndex(ctx context.Context, index string, body map[string]interface{}) error {
	path := "/" + index
	return c.request(ctx, "PUT", path, body, nil)
}

// DeleteIndex 删除索引
func (c *Client) DeleteIndex(ctx context.Context, index string) error {
	path := "/" + index
	return c.request(ctx, "DELETE", path, nil, nil)
}

// IndexDocument 索引文档
func (c *Client) IndexDocument(ctx context.Context, index, id string, body map[string]interface{}, refresh string) error {
	path := fmt.Sprintf("/%s/_doc/%s", index, id)

	params := make(map[string]string)
	if refresh != "" {
		params["refresh"] = refresh
	}

	return c.request(ctx, "PUT", path, body, nil, params)
}

// GetDocument 获取文档
func (c *Client) GetDocument(ctx context.Context, index, id string) (*DocumentResult, error) {
	path := fmt.Sprintf("/%s/_doc/%s", index, id)

	var result DocumentResult
	if err := c.request(ctx, "GET", path, nil, &result); err != nil {
		if isNotFoundError(err) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}

	if !result.Found {
		return nil, ErrDocumentNotFound
	}

	return &result, nil
}

// BulkRequest 批量请求
func (c *Client) BulkRequest(ctx context.Context, body io.Reader, refresh string) (*BulkResult, error) {
	path := "/_bulk"

	params := make(map[string]string)
	if refresh != "" {
		params["refresh"] = refresh
	}

	var result BulkResult
	if err := c.request(ctx, "POST", path, body, &result, params); err != nil {
		return nil, err
	}

	return &result, nil
}

// SearchRequest 搜索请求
func (c *Client) SearchRequest(ctx context.Context, index string, query map[string]interface{}) (*SearchResult, error) {
	path := "/" + index + "/_search"

	var result SearchResult
	if err := c.request(ctx, "POST", path, query, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MgetRequest 多文档获取请求
func (c *Client) MgetRequest(ctx context.Context, body io.Reader) (*MgetResult, error) {
	path := "/_mget"

	var result MgetResult
	if err := c.request(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteByQuery 按查询删除
func (c *Client) DeleteByQuery(ctx context.Context, index string, query map[string]interface{}) (*DeleteByQueryResult, error) {
	path := "/" + index + "/_delete_by_query"

	var result DeleteByQueryResult
	if err := c.request(ctx, "POST", path, query, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateByScript 使用脚本更新
func (c *Client) UpdateByScript(ctx context.Context, index, id string, script map[string]interface{}) error {
	path := fmt.Sprintf("/%s/_update/%s", index, id)
	return c.request(ctx, "POST", path, script, nil)
}

// request 执行 HTTP 请求
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}, params ...map[string]string) error {
	// 构建请求 URL
	url := c.buildURL(path)

	// 序列化请求体
	var bodyReader io.Reader
	if body != nil {
		if reader, ok := body.(io.Reader); ok {
			bodyReader = reader
		} else {
			jsonData, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("failed to marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonData)
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// 执行请求
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 对于 HEAD 请求，404 表示资源不存在（这是正常的）
	if method == "HEAD" && resp.StatusCode == 404 {
		return fmt.Errorf("index not found")
	}

	// 检查 HTTP 状态码
	if resp.StatusCode >= 400 {
		return fmt.Errorf("elasticsearch error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	// 解析响应
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// buildURL 构建请求 URL
func (c *Client) buildURL(path string) string {
	// 使用第一个地址（可以扩展为负载均衡）
	if len(c.addresses) > 0 {
		return c.addresses[0] + path
	}
	return path
}

// basicAuth 生成 Basic Auth 头
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// isNotFoundError 判断是否为 404 错误
func isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "document not found" ||
		err.Error() == "404 Not Found" ||
		err.Error() == "index not found" ||
		err.Error() == "index not found")
}

// 错误定义
var (
	ErrDocumentNotFound = fmt.Errorf("document not found")
	ErrIndexNotFound    = fmt.Errorf("index not found")
)

// ClusterHealthInfo 集群健康信息
type ClusterHealthInfo struct {
	ClusterName string `json:"cluster_name"`
	Status      string `json:"status"` // green, yellow, red
	Active      int    `json:"active_shards"`
	Relocating  int    `json:"relocating_shards"`
}

// DocumentResult 文档结果
type DocumentResult struct {
	Found  bool                   `json:"found"`
	Source map[string]interface{} `json:"_source"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Hits HitsInfo `json:"hits"`
}

type HitsInfo struct {
	Total TotalInfo `json:"total"`
	Hits  []HitInfo `json:"hits"`
}

type TotalInfo struct {
	Value int `json:"value"`
}

type HitInfo struct {
	ID     string                 `json:"_id"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

// MgetResult 多文档获取结果
type MgetResult struct {
	Docs []DocInfo `json:"docs"`
}

type DocInfo struct {
	Found  bool                   `json:"found"`
	ID     string                 `json:"_id"`
	Source map[string]interface{} `json:"_source,omitempty"`
}

// BulkResult 批量操作结果
type BulkResult struct {
	Errors bool         `json:"errors"`
	Items  []BulkItem   `json:"items"`
}

type BulkItem struct {
	Index  BulkItemInfo `json:"index"`
	Update BulkItemInfo `json:"update"`
	Delete BulkItemInfo `json:"delete"`
}

type BulkItemInfo struct {
	ID     string `json:"_id"`
	Status int    `json:"status"`
	Error  ErrorInfo `json:"error,omitempty"`
}

type ErrorInfo struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// DeleteByQueryResult 按查询删除结果
type DeleteByQueryResult struct {
	Total      int `json:"total"`
	Deleted    int `json:"deleted"`
	Failed     int `json:"failed"`
}

package registry

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"centag/core/pkg/logger"
)

// Client 插件注册表客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	
	// 本地缓存
	cacheMu      sync.RWMutex
	cache        map[string]*PluginMetadata
	cacheExpiry  map[string]time.Time
	cacheTTL     time.Duration
}

// NewClient 创建客户端
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		cache:       make(map[string]*PluginMetadata),
		cacheExpiry: make(map[string]time.Time),
		cacheTTL:    5 * time.Minute,
	}
}

// SetCacheTTL 设置缓存过期时间
func (c *Client) SetCacheTTL(ttl time.Duration) {
	c.cacheTTL = ttl
}

// Register 注册插件
func (c *Client) Register(ctx context.Context, req *RegisterPluginRequest) (*RegisterPluginResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/registry/plugins", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to register plugin: %s - %s", resp.Status, string(body))
	}
	
	var result RegisterPluginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

// List 列出插件
func (c *Client) List(ctx context.Context, req *ListPluginsRequest) (*ListPluginsResponse, error) {
	// 构建查询参数
	params := url.Values{}
	if req.Category != "" {
		params.Set("category", req.Category)
	}
	if req.Author != "" {
		params.Set("author", req.Author)
	}
	if req.Search != "" {
		params.Set("search", req.Search)
	}
	if req.SortBy != "" {
		params.Set("sort_by", req.SortBy)
	}
	if req.SortOrder != "" {
		params.Set("sort_order", req.SortOrder)
	}
	if req.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", req.Page))
	}
	if req.PageSize > 0 {
		params.Set("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	for _, tag := range req.Tags {
		params.Add("tags", tag)
	}
	
	url := c.baseURL + "/api/v1/registry/plugins"
	if len(params) > 0 {
		url += "?" + params.Encode()
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list plugins: %s - %s", resp.Status, string(body))
	}
	
	var result ListPluginsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

// Get 获取插件详情（带缓存）
func (c *Client) Get(ctx context.Context, id string) (*PluginMetadata, error) {
	// 检查缓存
	c.cacheMu.RLock()
	if cached, ok := c.cache[id]; ok {
		if expiry, ok := c.cacheExpiry[id]; ok && time.Now().Before(expiry) {
			c.cacheMu.RUnlock()
			return cached, nil
		}
	}
	c.cacheMu.RUnlock()
	
	// 从服务器获取
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s", c.baseURL, id)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get plugin: %s - %s", resp.Status, string(body))
	}
	
	var plugin PluginMetadata
	if err := json.NewDecoder(resp.Body).Decode(&plugin); err != nil {
		return nil, err
	}
	
	// 更新缓存
	c.cacheMu.Lock()
	c.cache[id] = &plugin
	c.cacheExpiry[id] = time.Now().Add(c.cacheTTL)
	c.cacheMu.Unlock()
	
	return &plugin, nil
}

// Delete 删除插件
func (c *Client) Delete(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s", c.baseURL, id)
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete plugin: %s - %s", resp.Status, string(body))
	}
	
	// 清除缓存
	c.cacheMu.Lock()
	delete(c.cache, id)
	delete(c.cacheExpiry, id)
	c.cacheMu.Unlock()
	
	return nil
}

// ListVersions 列出插件版本
func (c *Client) ListVersions(ctx context.Context, id string) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s/versions", c.baseURL, id)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list versions: %s - %s", resp.Status, string(body))
	}
	
	var result struct {
		PluginID string   `json:"plugin_id"`
		Versions []string `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Versions, nil
}

// GetVersion 获取特定版本
func (c *Client) GetVersion(ctx context.Context, id string, version string) (*PluginMetadata, error) {
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s/versions/%s", c.baseURL, id, version)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get version: %s - %s", resp.Status, string(body))
	}
	
	var plugin PluginMetadata
	if err := json.NewDecoder(resp.Body).Decode(&plugin); err != nil {
		return nil, err
	}
	
	return &plugin, nil
}

// Rate 评分插件
func (c *Client) Rate(ctx context.Context, id string, score int, comment string) error {
	body, err := json.Marshal(RatePluginRequest{Score: score, Comment: comment})
	if err != nil {
		return err
	}
	
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s/ratings", c.baseURL, id)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to rate plugin: %s - %s", resp.Status, string(body))
	}
	
	return nil
}

// GetRating 获取插件评分
func (c *Client) GetRating(ctx context.Context, id string) (float64, int, error) {
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s/ratings", c.baseURL, id)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("failed to get rating: %s - %s", resp.Status, string(body))
	}
	
	var result struct {
		PluginID    string  `json:"plugin_id"`
		Rating      float64 `json:"rating"`
		RatingCount int     `json:"rating_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}
	
	return result.Rating, result.RatingCount, nil
}

// Download 下载插件
func (c *Client) Download(ctx context.Context, id string) (*PluginMetadata, error) {
	url := fmt.Sprintf("%s/api/v1/registry/plugins/%s/download", c.baseURL, id)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to download plugin: %s - %s", resp.Status, string(body))
	}
	
	var result struct {
		PluginID    string `json:"plugin_id"`
		DownloadURL string `json:"download_url"`
		Checksum    string `json:"checksum"`
		Signature   string `json:"signature"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	// 获取插件元数据
	return c.Get(ctx, id)
}

// Discover 发现插件（搜索 + 筛选）
func (c *Client) Discover(ctx context.Context, category string, tags []string) ([]PluginMetadata, error) {
	req := &ListPluginsRequest{
		Category: category,
		Tags:     tags,
		SortBy:   "download_count",
		SortOrder: "desc",
		Page:     1,
		PageSize: 50,
	}
	
	resp, err := c.List(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return resp.Plugins, nil
}

// Search 搜索插件
func (c *Client) Search(ctx context.Context, query string) ([]PluginMetadata, error) {
	req := &ListPluginsRequest{
		Search:   query,
		SortBy:   "rating",
		SortOrder: "desc",
		Page:     1,
		PageSize: 20,
	}
	
	resp, err := c.List(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return resp.Plugins, nil
}

// GetLatestVersion 获取最新版本
func (c *Client) GetLatestVersion(ctx context.Context, name string) (*PluginMetadata, error) {
	// 获取所有版本
	versions, err := c.ListVersions(ctx, name)
	if err != nil {
		return nil, err
	}
	
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for plugin: %s", name)
	}
	
	// 找出最新版本
	latest, err := LatestVersion(versions)
	if err != nil {
		return nil, err
	}
	
	// 获取该版本的元数据
	return c.GetVersion(ctx, name, latest)
}

// ResolveDependency 解析依赖
func (c *Client) ResolveDependency(ctx context.Context, dep Dependency) (*PluginMetadata, error) {
	// 获取插件的所有版本
	versions, err := c.ListVersions(ctx, dep.ID)
	if err != nil {
		return nil, err
	}
	
	// 找出满足约束的版本
	compatible, err := CompatibleVersions(versions, dep.Version)
	if err != nil {
		return nil, err
	}
	
	if len(compatible) == 0 {
		return nil, fmt.Errorf("no compatible version found for %s@%s", dep.ID, dep.Version)
	}
	
	// 获取最新兼容版本
	latest, err := LatestVersion(compatible)
	if err != nil {
		return nil, err
	}
	
	return c.GetVersion(ctx, dep.ID, latest)
}

// ClearCache 清除缓存
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	
	c.cache = make(map[string]*PluginMetadata)
	c.cacheExpiry = make(map[string]time.Time)
}

// GetCacheStats 获取缓存统计
func (c *Client) GetCacheStats() (int, int) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	
	total := len(c.cache)
	expired := 0
	now := time.Now()
	
	for id, expiry := range c.cacheExpiry {
		if now.After(expiry) {
			expired++
			// 从缓存中删除过期项
			delete(c.cache, id)
			delete(c.cacheExpiry, id)
		}
	}
	
	return total, expired
}

// PluginDownloader 插件下载器
type PluginDownloader struct {
	client      *Client
	downloadDir string
	// trustedKeys 信任的 Ed25519 公钥列表（base64 编码），用于验证远程插件签名
	trustedKeys []string
}

// NewPluginDownloader 创建下载器
func NewPluginDownloader(client *Client, downloadDir string, trustedKeys ...string) *PluginDownloader {
	return &PluginDownloader{
		client:      client,
		downloadDir: downloadDir,
		trustedKeys: trustedKeys,
	}
}

// SetTrustedKeys 设置信任的公钥列表
func (d *PluginDownloader) SetTrustedKeys(keys []string) {
	d.trustedKeys = keys
}

// DownloadAndVerify 下载并验证插件
func (d *PluginDownloader) DownloadAndVerify(ctx context.Context, id string) (string, error) {
	// 获取插件信息
	plugin, err := d.client.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get plugin metadata: %w", err)
	}
	
	if plugin.DownloadURL == "" {
		return "", fmt.Errorf("plugin download URL is empty")
	}
	
	// 下载插件文件
	logger.Infof("Downloading plugin %s from %s", id, plugin.DownloadURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", plugin.DownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	
	resp, err := d.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// 读取文件内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read downloaded data: %w", err)
	}
	
	// 验证 checksum（如果提供）
	if plugin.Checksum != "" {
		checksum := sha256.Sum256(data)
		actualChecksum := hex.EncodeToString(checksum[:])
		
		if actualChecksum != plugin.Checksum {
			return "", fmt.Errorf("checksum mismatch: expected %s, got %s", plugin.Checksum, actualChecksum)
		}
		logger.Infof("Checksum verified for plugin %s", id)
	}
	
	// 验证 signature（如果提供）
	if plugin.Signature != "" {
		if len(d.trustedKeys) == 0 {
			logger.Warnf("Plugin %s has signature but no trusted keys configured; skipping verification", id)
		} else {
			verified, verifyErr := d.verifySignature(data, plugin.Signature)
			if verifyErr != nil {
				return "", fmt.Errorf("signature verification error for plugin %s: %w", id, verifyErr)
			}
			if !verified {
				return "", fmt.Errorf("signature verification failed for plugin %s: no matching trusted key", id)
			}
			logger.Infof("Signature verified for plugin %s", id)
		}
	}
	
	// 确保下载目录存在
	if err := os.MkdirAll(d.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}
	
	// 保存到文件
	filename := fmt.Sprintf("%s-%s.zip", plugin.Name, plugin.Version)
	filePath := filepath.Join(d.downloadDir, filename)
	
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save plugin file: %w", err)
	}
	
	logger.Infof("Plugin %s downloaded and verified successfully to %s", id, filePath)

	return filePath, nil
}

// verifySignature 使用信任的 Ed25519 公钥验证插件签名
// signature 为 base64 编码的签名数据
// 依次尝试所有信任的公钥，任一公钥验证通过即返回 true
func (d *PluginDownloader) verifySignature(data []byte, signature string) (bool, error) {
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding: %w", err)
	}

	for _, keyB64 := range d.trustedKeys {
		keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
		if err != nil {
			logger.Warnf("Skipping invalid trusted key encoding: %v", err)
			continue
		}
		if len(keyBytes) != ed25519.PublicKeySize {
			logger.Warnf("Skipping invalid Ed25519 public key length: got %d, expected %d", len(keyBytes), ed25519.PublicKeySize)
			continue
		}
		pubKey := ed25519.PublicKey(keyBytes)
		if ed25519.Verify(pubKey, data, sigBytes) {
			return true, nil
		}
	}

	return false, nil
}

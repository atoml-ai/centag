package pipeline

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/plugin"
)

// CapabilityBroker 按权限向插件提供受控能力
type CapabilityBroker interface {
	// GetLLMClient 根据权限返回受控的 LLM 客户端（非流式）
	GetLLMClient(ctx context.Context, permissions []string) (LLMClient, error)

	// GetLLMStreamClient 根据权限返回受控的 LLM 流式客户端
	GetLLMStreamClient(ctx context.Context, permissions []string) (LLMStreamClient, error)

	// GetStorage 根据权限返回受控的存储访问
	GetStorage(ctx context.Context, permissions []string) (Storage, error)

	// GetMemory 根据权限返回受控的记忆访问
	GetMemory(ctx context.Context, permissions []string) (Memory, error)

	// GetSecretsResolver 根据权限返回密钥解析器
	GetSecretsResolver(ctx context.Context, permissions []string) (SecretsResolver, error)

	// GetHTTPClient 根据权限返回受控的 HTTP 客户端
	GetHTTPClient(ctx context.Context, permissions []string) (HTTPClient, error)

	// GetCacheStrategy 获取缓存策略插件
	// strategy: 策略名称 (exact/semantic/hybrid 或自定义)
	// permissions: 权限列表，用于控制访问
	GetCacheStrategy(ctx context.Context, strategy string, permissions []string) (CacheStrategyCapability, error)

	// GetVectorCache 根据权限返回向量缓存能力
	GetVectorCache(ctx context.Context, permissions []string) (VectorCacheCapability, error)

	// GetEmbeddingService 根据权限返回嵌入服务能力
	GetEmbeddingService(ctx context.Context, permissions []string) (EmbeddingCapability, error)
}

// LLMClient 受控的 LLM 调用能力（非流式）
type LLMClient interface {
	Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
}

// LLMStreamClient 受控的 LLM 流式调用能力
type LLMStreamClient interface {
	// ChatStream 流式调用 LLM，返回 chunk channel
	ChatStream(ctx context.Context, req *LLMRequest) (<-chan plugin.StreamChunk, error)
}

// Storage 受控的存储访问能力
type Storage interface {
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
}

// Memory 受控的记忆访问能力
type Memory interface {
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, value []byte) error
	Search(ctx context.Context, query string, limit int) ([]MemoryResult, error)
}

// MemoryResult 记忆搜索结果
type MemoryResult struct {
	Key   string
	Score float64
	Data  []byte
}

// SecretsResolver 密钥解析能力
type SecretsResolver interface {
	Resolve(ref string) (string, error)
}

// HTTPClient 受控的 HTTP 调用能力
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// LLMProvider LLM 提供者接口
// 用于通过后端管理器创建 LLM 客户端
type LLMProvider interface {
	// CreateClient 创建 LLM 客户端
	// backendID: 后端 ID
	// model: 模型名称
	CreateClient(backendID, model string) (LLMClient, error)
}

// ========== Cache Strategy Capabilities ==========

// CacheStrategyCapability 缓存策略能力接口
// 通过 CapabilityBroker 提供缓存策略访问
type CacheStrategyCapability interface {
	// Read 读取缓存
	Read(ctx context.Context, query string, threshold float32, topK int) (*CacheReadResult, error)
	// Write 写入缓存；request 为用于 embedding/召回的查询文本（可为空，语义策略会拒绝空向量）
	Write(ctx context.Context, key string, request string, content string, ttl time.Duration) error
	// Delete 删除缓存
	Delete(ctx context.Context, key string) error
	// StrategyName 返回策略名称
	StrategyName() string
}

// CacheReadResult 缓存读取结果
type CacheReadResult struct {
	Hit     bool
	Content string
	Key     string
	Score   float64
}

// VectorCacheCapability 向量缓存能力接口
type VectorCacheCapability interface {
	// Search 搜索相似向量
	Search(ctx context.Context, vector []float32, topK int, threshold float32) ([]VectorSearchResult, error)
	// Insert 插入向量
	Insert(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	// Delete 删除向量
	Delete(ctx context.Context, id string) error
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	ID       string
	Score    float64
	Metadata map[string]interface{}
}

// EmbeddingCapability 嵌入服务能力接口
type EmbeddingCapability interface {
	// Embed 生成文本的嵌入向量
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch 批量生成嵌入向量
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension 返回嵌入向量维度
	Dimension() int
}

// PermissionChecker 权限检查器
type PermissionChecker interface {
	HasPermission(permissions []string, required string) bool
}

// DefaultCapabilityBroker 默认的 CapabilityBroker 实现
type DefaultCapabilityBroker struct {
	storageProvider    StorageProvider
	memoryProvider     MemoryProvider
	secretsProvider    SecretsProvider
	httpConfig         HTTPConfig
	permissionChecker  PermissionChecker
	// 新增：缓存策略相关提供者
	cacheStrategyProvider  CacheStrategyProvider
	vectorCacheProvider    VectorCacheProvider
	embeddingProvider      EmbeddingProvider
	// LLM 提供者（用于通过后端管理器调用 LLM）
	llmProvider LLMProvider
}

// CacheStrategyProvider 缓存策略提供者接口
type CacheStrategyProvider interface {
	GetStrategy(name string) (CacheStrategyCapability, error)
}

// VectorCacheProvider 向量缓存提供者接口
type VectorCacheProvider interface {
	GetVectorCache(namespace string) (VectorCacheCapability, error)
}

// EmbeddingProvider 嵌入服务提供者接口
type EmbeddingProvider interface {
	GetEmbeddingService(model string) (EmbeddingCapability, error)
}

// StorageProvider 存储提供者接口
type StorageProvider interface {
	GetStorage(namespace string) (Storage, error)
}

// MemoryProvider 记忆提供者接口
type MemoryProvider interface {
	GetMemory(namespace string) (Memory, error)
}

// SecretsProvider 密钥提供者接口
type SecretsProvider interface {
	ResolveSecret(ref string) (string, error)
}

// HTTPConfig HTTP 配置
type HTTPConfig struct {
	Allowlist     []string
	Timeout       int
	MaxResponse   int64
	TLSVerify     bool
}

// NewCapabilityBroker 创建默认的能力代理
func NewCapabilityBroker(
	storageProvider StorageProvider,
	memoryProvider MemoryProvider,
	secretsProvider SecretsProvider,
	httpConfig HTTPConfig,
) *DefaultCapabilityBroker {
	return &DefaultCapabilityBroker{
		storageProvider:   storageProvider,
		memoryProvider:    memoryProvider,
		secretsProvider:   secretsProvider,
		httpConfig:        httpConfig,
		permissionChecker: &defaultPermissionChecker{},
	}
}

// NewCapabilityBrokerWithCache 创建带缓存策略支持的能力代理
func NewCapabilityBrokerWithCache(
	storageProvider StorageProvider,
	memoryProvider MemoryProvider,
	secretsProvider SecretsProvider,
	httpConfig HTTPConfig,
	cacheStrategyProvider CacheStrategyProvider,
	vectorCacheProvider VectorCacheProvider,
	embeddingProvider EmbeddingProvider,
) *DefaultCapabilityBroker {
	return &DefaultCapabilityBroker{
		storageProvider:       storageProvider,
		memoryProvider:        memoryProvider,
		secretsProvider:       secretsProvider,
		httpConfig:            httpConfig,
		permissionChecker:     &defaultPermissionChecker{},
		cacheStrategyProvider: cacheStrategyProvider,
		vectorCacheProvider:   vectorCacheProvider,
		embeddingProvider:     embeddingProvider,
	}
}

// SetCacheStrategyProvider 设置缓存策略提供者
func (b *DefaultCapabilityBroker) SetCacheStrategyProvider(provider CacheStrategyProvider) {
	b.cacheStrategyProvider = provider
}

// SetVectorCacheProvider 设置向量缓存提供者
func (b *DefaultCapabilityBroker) SetVectorCacheProvider(provider VectorCacheProvider) {
	b.vectorCacheProvider = provider
}

// SetEmbeddingProvider 设置嵌入服务提供者
func (b *DefaultCapabilityBroker) SetEmbeddingProvider(provider EmbeddingProvider) {
	b.embeddingProvider = provider
}

// SetLLMProvider 设置 LLM 提供者
func (b *DefaultCapabilityBroker) SetLLMProvider(provider LLMProvider) {
	b.llmProvider = provider
}

// GetLLMClient 根据权限返回受控的 LLM 客户端
func (b *DefaultCapabilityBroker) GetLLMClient(ctx context.Context, permissions []string) (LLMClient, error) {
	if !b.permissionChecker.HasPermission(permissions, "llm.call") {
		return nil, fmt.Errorf("permission denied: llm.call required")
	}

	// 如果配置了 LLM 提供者，使用它创建客户端
	if b.llmProvider != nil {
		// 从权限中提取 backend ID 和 model
		backendID := extractBackendFromPermissions(permissions)
		model := extractModelFromLLMPermissions(permissions)
		
		if backendID == "" || model == "" {
			return nil, fmt.Errorf("llm.call requires backend ID and model in permissions (e.g., llm.call:backend-id:model-name)")
		}
		
		return b.llmProvider.CreateClient(backendID, model)
	}

	return nil, fmt.Errorf("llm.call not available: LLM provider not configured")
}

// GetLLMStreamClient 根据权限返回受控的 LLM 流式客户端
func (b *DefaultCapabilityBroker) GetLLMStreamClient(ctx context.Context, permissions []string) (LLMStreamClient, error) {
	if !b.permissionChecker.HasPermission(permissions, "llm.call") {
		return nil, fmt.Errorf("permission denied: llm.call required")
	}

	if b.llmProvider != nil {
		backendID := extractBackendFromPermissions(permissions)
		model := extractModelFromLLMPermissions(permissions)
		
		if backendID == "" || model == "" {
			return nil, fmt.Errorf("llm.call requires backend ID and model in permissions (e.g., llm.call:backend-id:model-name)")
		}
		
		client, err := b.llmProvider.CreateClient(backendID, model)
		if err != nil {
			return nil, err
		}
		// llmClient 同时实现了 LLMClient 和 LLMStreamClient
		if streamClient, ok := client.(LLMStreamClient); ok {
			return streamClient, nil
		}
		return nil, fmt.Errorf("created client does not support streaming")
	}

	return nil, fmt.Errorf("llm.call not available: LLM provider not configured")
}

// GetStorage 根据权限返回受控的存储访问
func (b *DefaultCapabilityBroker) GetStorage(ctx context.Context, permissions []string) (Storage, error) {
	if !b.permissionChecker.HasPermission(permissions, "storage.read") &&
		!b.permissionChecker.HasPermission(permissions, "storage.write") {
		return nil, fmt.Errorf("permission denied: storage.read or storage.write required")
	}

	if b.storageProvider == nil {
		return nil, fmt.Errorf("storage provider not configured")
	}

	// 根据权限确定命名空间
	namespace := extractNamespace(permissions, "storage")
	return b.storageProvider.GetStorage(namespace)
}

// GetMemory 根据权限返回受控的记忆访问
func (b *DefaultCapabilityBroker) GetMemory(ctx context.Context, permissions []string) (Memory, error) {
	if !b.permissionChecker.HasPermission(permissions, "memory.read") &&
		!b.permissionChecker.HasPermission(permissions, "memory.write") {
		return nil, fmt.Errorf("permission denied: memory.read or memory.write required")
	}

	if b.memoryProvider == nil {
		return nil, fmt.Errorf("memory provider not configured")
	}

	// 根据权限确定命名空间
	namespace := extractNamespace(permissions, "memory")
	return b.memoryProvider.GetMemory(namespace)
}

// GetSecretsResolver 根据权限返回密钥解析器
func (b *DefaultCapabilityBroker) GetSecretsResolver(ctx context.Context, permissions []string) (SecretsResolver, error) {
	if !b.permissionChecker.HasPermission(permissions, "secrets.read") {
		return nil, fmt.Errorf("permission denied: secrets.read required")
	}

	return &controlledSecretsResolver{
		secretsProvider: b.secretsProvider,
	}, nil
}

// GetHTTPClient 根据权限返回受控的 HTTP 客户端
func (b *DefaultCapabilityBroker) GetHTTPClient(ctx context.Context, permissions []string) (HTTPClient, error) {
	if !b.permissionChecker.HasPermission(permissions, "network.outbound") {
		return nil, fmt.Errorf("permission denied: network.outbound required")
	}

	return &controlledHTTPClient{
		allowlist: b.httpConfig.Allowlist,
		timeout:   b.httpConfig.Timeout,
		maxResponse: b.httpConfig.MaxResponse,
		tlsVerify: b.httpConfig.TLSVerify,
	}, nil
}

// GetCacheStrategy 获取缓存策略插件
func (b *DefaultCapabilityBroker) GetCacheStrategy(ctx context.Context, strategy string, permissions []string) (CacheStrategyCapability, error) {
	if !b.permissionChecker.HasPermission(permissions, "cache.read") &&
		!b.permissionChecker.HasPermission(permissions, "cache.write") {
		return nil, fmt.Errorf("permission denied: cache.read or cache.write required")
	}

	if b.cacheStrategyProvider == nil {
		return nil, fmt.Errorf("cache strategy provider not configured")
	}

	return b.cacheStrategyProvider.GetStrategy(strategy)
}

// GetVectorCache 根据权限返回向量缓存能力
func (b *DefaultCapabilityBroker) GetVectorCache(ctx context.Context, permissions []string) (VectorCacheCapability, error) {
	if !b.permissionChecker.HasPermission(permissions, "vector.read") &&
		!b.permissionChecker.HasPermission(permissions, "vector.write") {
		return nil, fmt.Errorf("permission denied: vector.read or vector.write required")
	}

	if b.vectorCacheProvider == nil {
		return nil, fmt.Errorf("vector cache provider not configured")
	}

	namespace := extractNamespace(permissions, "vector")
	return b.vectorCacheProvider.GetVectorCache(namespace)
}

// GetEmbeddingService 根据权限返回嵌入服务能力
func (b *DefaultCapabilityBroker) GetEmbeddingService(ctx context.Context, permissions []string) (EmbeddingCapability, error) {
	if !b.permissionChecker.HasPermission(permissions, "embedding.generate") {
		return nil, fmt.Errorf("permission denied: embedding.generate required")
	}

	if b.embeddingProvider == nil {
		return nil, fmt.Errorf("embedding provider not configured")
	}

	// 从权限中提取模型名称（可选）
	model := extractModelFromPermissions(permissions)
	return b.embeddingProvider.GetEmbeddingService(model)
}

// extractModelFromPermissions 从权限中提取模型名称
func extractModelFromPermissions(permissions []string) string {
	for _, p := range permissions {
		if len(p) > len("embedding.generate:") && p[:len("embedding.generate:")] == "embedding.generate:" {
			return p[len("embedding.generate:"):]
		}
	}
	return "" // 默认模型
}

// extractBackendFromPermissions 从权限中提取后端 ID
// 权限格式: llm.call:backend-id:model-name
func extractBackendFromPermissions(permissions []string) string {
	for _, p := range permissions {
		if strings.HasPrefix(p, "llm.call:") {
			parts := strings.SplitN(p[len("llm.call:"):], ":", 2)
			if len(parts) >= 1 && parts[0] != "" {
				return parts[0]
			}
		}
	}
	return ""
}

// extractModelFromLLMPermissions 从 llm.call 权限中提取模型名称
// 权限格式: llm.call:backend-id:model-name
func extractModelFromLLMPermissions(permissions []string) string {
	for _, p := range permissions {
		if strings.HasPrefix(p, "llm.call:") {
			parts := strings.SplitN(p[len("llm.call:"):], ":", 2)
			if len(parts) >= 2 && parts[1] != "" {
				return parts[1]
			}
		}
	}
	return ""
}

// controlledSecretsResolver 受控的密钥解析器
type controlledSecretsResolver struct {
	secretsProvider SecretsProvider
}

func (r *controlledSecretsResolver) Resolve(ref string) (string, error) {
	if r.secretsProvider == nil {
		return "", fmt.Errorf("secrets provider not configured")
	}
	return r.secretsProvider.ResolveSecret(ref)
}

// controlledHTTPClient 受控的 HTTP 客户端
type controlledHTTPClient struct {
	allowlist    []string
	timeout      int
	maxResponse  int64
	tlsVerify    bool
}

func (c *controlledHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// 检查 allowlist
	if len(c.allowlist) > 0 {
		host := req.URL.Hostname()
		allowed := false
		for _, allowedHost := range c.allowlist {
			if host == allowedHost || (allowedHost == "*") {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("host %s not in allowlist", host)
		}
	}

	// 创建 HTTP 客户端。
	// 优先使用请求 Context 的 deadline（节点 timeout，如 transparent_forward=120s），
	// 避免全局 Proxy.Timeout=30s 在读 SSE/长响应体时提前打断。
	//
	// 关键禁环境代理（HTTP_PROXY/HTTPS_PROXY）：网关自身出站若再走本机 MITM，
	// 会把 opencode.ai 请求环回 Centag，最终落到其它后端（如智谱 1211），表现为
	// 「默认可用模型总是被降级」。
	client := &http.Client{
		Transport: gatewayEgressTransport(c.tlsVerify),
		// 与 transparent_forward.redirect_policy=never 对齐：禁止自动跟随 3xx，
		// 避免误跳到其它厂商域名后把错误体当成原上游响应。
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if deadline, ok := req.Context().Deadline(); ok {
		d := time.Until(deadline)
		if d < time.Second {
			d = time.Second
		}
		client.Timeout = d + time.Second
	} else if c.timeout > 0 {
		client.Timeout = time.Duration(c.timeout) * time.Second
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// 检查响应体大小
	if c.maxResponse > 0 && resp.ContentLength > c.maxResponse {
		resp.Body.Close()
		return nil, fmt.Errorf("response body size %d exceeds limit %d", resp.ContentLength, c.maxResponse)
	}

	return resp, nil
}

// egressTransports 进程级共享出站 Transport，按 TLS 验证开关分为两个实例。
// 避免每次 Do() 调用 http.Transport.Clone() 导致连接池碎片化。
var (
	egressTransportWithTLS    *http.Transport
	egressTransportSkipVerify *http.Transport
	egressTransportOnce       sync.Once
)

func initEgressTransports() {
	base, _ := http.DefaultTransport.(*http.Transport)
	if base != nil {
		egressTransportWithTLS = base.Clone()
		egressTransportWithTLS.Proxy = nil
		egressTransportSkipVerify = base.Clone()
		egressTransportSkipVerify.Proxy = nil
		egressTransportSkipVerify.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		egressTransportWithTLS = &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		egressTransportSkipVerify = egressTransportWithTLS.Clone()
		egressTransportSkipVerify.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

// gatewayEgressTransport 构建网关出站 Transport：直连上游，不继承进程环境代理。
func gatewayEgressTransport(tlsVerify bool) http.RoundTripper {
	egressTransportOnce.Do(initEgressTransports)
	if tlsVerify {
		return egressTransportWithTLS
	}
	return egressTransportSkipVerify
}

// defaultPermissionChecker 默认权限检查器
type defaultPermissionChecker struct{}

func (c *defaultPermissionChecker) HasPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == required || p == "*" {
			return true
		}
		// 支持前缀匹配（例如 "llm.call:backend:model" 匹配 "llm.call"）
		if len(p) > len(required) && p[len(required)] == ':' && strings.HasPrefix(p, required) {
			return true
		}
	}
	return false
}

// extractNamespace 从权限中提取命名空间
func extractNamespace(permissions []string, prefix string) string {
	// 权限格式: storage.read:namespace 或 storage.read
	for _, p := range permissions {
		if len(p) > len(prefix) && p[:len(prefix)] == prefix {
			// 尝试提取命名空间部分
			parts := splitPermission(p)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return "default"
}

// splitPermission 分割权限字符串
// 格式: "resource.action:namespace" 或 "resource.action"
func splitPermission(p string) []string {
	// 简单分割实现
	for i, c := range p {
		if c == ':' {
			return []string{p[:i], p[i+1:]}
		}
	}
	return []string{p}
}

// backendIDContextKey context key for backend ID
type backendIDContextKey struct{}

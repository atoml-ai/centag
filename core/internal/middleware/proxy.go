package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/backend"
	"centag/core/internal/cache"
	"centag/core/pkg/config"
	"centag/core/internal/httpclient"
	"centag/core/pkg/logger"
	"centag/core/pkg/metrics"
	"centag/core/pkg/processor"
	"centag/core/internal/stats"
	"centag/core/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProxyMiddleware HTTP透明代理中间件
// 只截获大模型API请求,其他请求透明转发
func ProxyMiddleware(backendManager *backend.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否是API请求
		if isLLMRequest(c.Request) {
			// LLM请求,走我们的处理逻辑
			c.Next()
			return
		}

		// 非LLM请求,透明代理到目标服务器
		handleTransparentProxy(c)
	}
}

// isLLMRequest 判断是否是LLM API请求
func isLLMRequest(r *http.Request) bool {
	path := r.URL.Path

	// 检查路径是否包含LLM API标识
	llmPaths := []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/embeddings",
		"/v1/models",
		"/api/v1/chat/completions",
		"/api/v1/openai/chat/completions",
	}

	for _, llmPath := range llmPaths {
		if strings.Contains(path, llmPath) {
			return true
		}
	}

	// 检查Content-Type
	if r.Method == "POST" && r.Header.Get("Content-Type") == "application/json" {
		// 读取请求体(如果尚未读取)
		if r.Body != nil {
			var body map[string]interface{}
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			if json.Unmarshal(bodyBytes, &body) == nil {
				// 检查是否有模型字段
				if _, hasModel := body["model"]; hasModel {
					return true
				}
				// 检查是否有messages字段
				if _, hasMessages := body["messages"]; hasMessages {
					return true
				}
			}
		}
	}

	return false
}

// handleTransparentProxy 处理透明代理请求
func handleTransparentProxy(c *gin.Context) {
	// 获取目标URL(从查询参数或请求头)
	targetURL := c.GetHeader("X-Proxy-Target")
	if targetURL == "" {
		targetURL = c.Query("url")
	}

	if targetURL == "" {
		// 没有指定目标,返回404
		c.String(http.StatusNotFound, "Not Found")
		c.Abort()
		return
	}

	// 解析目标URL
	target, err := url.Parse(targetURL)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid target URL: %v", err)
		c.Abort()
		return
	}

	// 记录代理请求
	logger.Infof("Transparent proxy: %s %s -> %s", c.Request.Method, c.Request.URL.Path, targetURL)

	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(target)

	// 修改请求
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		// 转发原始请求路径
		req.URL.Path = c.Request.URL.Path
		req.URL.RawQuery = c.Request.URL.RawQuery

		// 保留原始请求头
		for k, v := range c.Request.Header {
			// 跳过一些不应该转发的头
			if shouldForwardHeader(k) {
				req.Header[k] = v
			}
		}

		// 添加代理标识头
		req.Header.Set("X-Forwarded-For", c.ClientIP())
		req.Header.Set("X-Real-IP", c.ClientIP())
	}

	// 处理响应
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Errorf("Proxy error: %v", err)
		http.Error(w, "Proxy error: "+err.Error(), http.StatusBadGateway)
	}

	// 执行代理
	proxy.ServeHTTP(c.Writer, c.Request)
	c.Abort()
}

// shouldForwardHeader 判断请求头是否应该转发
func shouldForwardHeader(header string) bool {
	// 不应该转发的头
	skipHeaders := map[string]bool{
		"Host":                true,
		"Content-Length":      true,
		"Connection":          true,
		"X-Proxy-Target":      true,
		"Authorization":       false, // 根据需要决定是否转发
		"Proxy-Authorization": true,
		"Proxy-Connection":    true,
		"Upgrade":             true,
	}

	return !skipHeaders[strings.ToLower(header)]
}

// LLMProxyHandler LLM代理处理器
// 直接调用OpenAI兼容的API
type LLMProxyHandler struct {
	backendManager *backend.Manager
	proxyCache     *cache.ProxyCache
	proxyConfig    *config.ProxyConfig // 添加代理配置
	cacheConfig    *config.CacheConfig // 添加缓存配置
}

// NewLLMProxyHandler 创建LLM代理处理器
func NewLLMProxyHandler(backendManager *backend.Manager, proxyCache *cache.ProxyCache, proxyConfig *config.ProxyConfig, cacheConfig *config.CacheConfig) *LLMProxyHandler {
	return &LLMProxyHandler{
		backendManager: backendManager,
		proxyCache:     proxyCache,
		proxyConfig:    proxyConfig,
		cacheConfig:    cacheConfig,
	}
}

// fallbackModelName 请求体未带 model 时用于缓存键等，与配置中的默认对话模型一致。
func (h *LLMProxyHandler) fallbackModelName() string {
	if h.proxyConfig != nil && strings.TrimSpace(h.proxyConfig.DefaultModel) != "" {
		return h.proxyConfig.DefaultModel
	}
	return "qwen2.5:1.5b"
}

// HandleOpenAIRequest 处理OpenAI兼容的请求
func (h *LLMProxyHandler) HandleOpenAIRequest(c *gin.Context) {
	start := time.Now()
	var cached bool
	var model string
	var statusCode int

	defer func() {
		// 记录请求统计
		latency := time.Since(start)
		metrics.GlobalMetrics.RecordRequest(model, statusCode, latency, cached)
	}()

	// 读取请求体
	var bodyBytes []byte
	var err error

	if c.Request.Body != nil {
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			h.recordCacheStats("", "BYPASS", false, http.StatusBadRequest, time.Since(start))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Failed to read request body",
					"type":    "invalid_request_error",
				},
			})
			statusCode = http.StatusBadRequest
			return
		}
	} else {
		bodyBytes = []byte{}
	}

	// 恢复请求体以供后续使用
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// 对于GET请求(如/models),不需要解析JSON
	if c.Request.Method == "GET" {
		// 获取租户ID
		tenantID := auth.GetTenantID(c)
		
		// 获取后端配置（租户隔离）
		var backendCfg *backend.BackendConfig
		var err error
		if tenantID != "" {
			backendCfg, err = h.backendManager.SelectBackendByTenant("openai", tenantID)
		} else {
			backendCfg, err = h.backendManager.SelectBackend("openai")
		}
		if err != nil {
			h.recordCacheStats("", "BYPASS", false, http.StatusServiceUnavailable, time.Since(start))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"message": "No available backend",
					"type":    "server_error",
				},
			})
			return
		}

		// 转发GET请求
		h.forwardGetRequest(c, backendCfg)
		return
	}

	// 解析POST请求体
	var req map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			h.recordCacheStats("", "BYPASS", false, http.StatusBadRequest, time.Since(start))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Failed to parse request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}
	}

	// 尝试从缓存获取响应
	// 如果配置了 EnableCacheRead=false，完全不走缓存命中流程，直接转发
	enableCacheRead := h.cacheConfig == nil || h.cacheConfig.EnableCacheRead
	if h.proxyCache != nil && h.proxyCache.IsEnabled() && enableCacheRead {
		// 解析请求参数
		model = getString(req, "model", "qwen/qwen3-4b-fp8")
		messages := getSlice(req, "messages", []interface{}{})
		temperature := getFloat(req, "temperature", 0.7)
		maxTokens := getInt(req, "max_tokens", 0)
		stream := getBool(req, "stream", false)

		// 生成缓存键（v0.2.8 R13：纳入 response_format/tool_choice/seed 防缓存污染）
		rf, tc, seed := cacheKeyProtocolFields(req)
		cacheKey, err := h.proxyCache.GetRequestKey(model, messages, temperature, maxTokens, rf, tc, seed)
		if err != nil {
			logger.Warn("Failed to generate cache key", zap.Error(err))
		} else {
			// 检查是否为流式请求
			if stream {
				// 流式请求也使用缓存
				logger.Infof("Stream request checking cache - model: %s, key: %s", model, cacheKey)

				// 尝试从缓存获取完整entry
				cachedEntry, found, err := h.proxyCache.TryGetEntry(c.Request.Context(), cacheKey)
				if err == nil && found {
					logger.Info("Stream cache hit, returning cached response", zap.String("key", cacheKey))
					cached = true
					statusCode = http.StatusOK

					// 记录统计
					h.recordCacheStats(model, "HIT-EXACT", true, http.StatusOK, time.Since(start))

					// 流式返回缓存数据
					h.streamCachedResponse(c, cachedEntry)
					return
				}

				logger.Info("Stream cache miss, calling backend", zap.String("key", cacheKey))
				// 继续执行后续代码,调用后端
			} else {
				// 非流式请求 — v0.3.3: 按 cache.backend 互斥召回；stacking 时 exact→semantic
				cacheCfg := config.CacheConfig{}
				if h.cacheConfig != nil {
					cacheCfg = *h.cacheConfig
				} else if cfg := config.Get(); cfg != nil {
					cacheCfg = cfg.Cache
				}
				config.NormalizeCacheConfig(&cacheCfg)
				threshold := cacheCfg.Semantic.Threshold
				if threshold <= 0 {
					threshold = 0.8
				}
				topK := cacheCfg.Semantic.TopK
				if topK <= 0 {
					topK = 5
				}

				tryExact := cacheCfg.Backend == config.CacheBackendExact || cacheCfg.AllowBackendStacking
				trySemantic := cacheCfg.Backend == config.CacheBackendSemantic ||
					(cacheCfg.AllowBackendStacking && cacheCfg.Backend == config.CacheBackendExact)

				if tryExact {
					cachedResp, found, err := h.proxyCache.TryGet(c.Request.Context(), cacheKey)
					if err != nil {
						logger.Warn("Failed to get exact cache", zap.Error(err))
					}
					if found {
						logger.Info("Exact cache hit, returning cached response", zap.String("key", cacheKey))
						cached = true
						statusCode = http.StatusOK
						h.recordCacheStats(model, "HIT-EXACT", false, http.StatusOK, time.Since(start))
						c.Header("X-Cache", "HIT-EXACT")
						c.Header("Content-Type", "application/json")
						c.String(http.StatusOK, cachedResp)
						return
					}
				}

				if trySemantic {
					query := h.proxyCache.GetRequestQuery(messages)
					if query != "" {
						semanticResult, found, err := h.proxyCache.TryGetSemantic(c.Request.Context(), query, threshold, topK)
						if err != nil {
							logger.Warn("Failed to get semantic cache", zap.Error(err))
						}
						if found {
							logger.Infof("✓ 语义缓存命中 - 查询: %s, 相似度: %.4f/%.2f, 缓存key: %s",
								query, semanticResult.Similarity, threshold, semanticResult.CacheKey)
							cached = true
							statusCode = http.StatusOK
							h.recordCacheStats(model, "HIT-SEMANTIC", false, http.StatusOK, time.Since(start))
							c.Header("X-Cache", "HIT-SEMANTIC")
							c.Header("Content-Type", "application/json")
							c.String(http.StatusOK, semanticResult.Response)
							return
						}
					}
				}

				logger.Debug("Cache miss", zap.String("key", cacheKey), zap.String("backend", cacheCfg.Backend))
				c.Header("X-Cache", "MISS")
			}
		}
	}

	// 选择后端配置
	// 1. 检查代理模式
	// 2. 支持环境变量强制指定后端
	var backendCfg *backend.BackendConfig
	var selectedModel string // 实际使用的模型

	// 获取租户ID（用于租户隔离）
	tenantID := auth.GetTenantID(c)

	// 获取请求的模型
	requestedModel := getString(req, "model", "")

	// 根据代理模式选择后端
	if h.proxyConfig != nil && h.proxyConfig.DefaultMode == "direct-backend" {
		// direct-backend 模式：直接使用配置的默认后端和模型
		logger.Info("Using direct-backend mode",
			zap.String("default_backend_id", h.proxyConfig.DefaultBackendID),
			zap.String("default_model", h.proxyConfig.DefaultModel),
			zap.String("requested_model", requestedModel),
			zap.String("tenant_id", tenantID))

		if h.proxyConfig.DefaultBackendID != "" {
			if tenantID != "" {
				backendCfg, err = h.backendManager.GetByTenant(tenantID, h.proxyConfig.DefaultBackendID)
			} else {
				backendCfg, err = h.backendManager.Get(h.proxyConfig.DefaultBackendID)
			}
			if err != nil {
				logger.Warn("Default backend not found, fallback to weight-based selection",
					zap.String("backend_id", h.proxyConfig.DefaultBackendID),
					zap.Error(err))
				if tenantID != "" {
					backendCfg, err = h.backendManager.SelectDefaultBackendByTenant(tenantID)
				} else {
					backendCfg, err = h.backendManager.SelectDefaultBackend()
				}
			} else {
				// 使用配置的默认模型；未配置时兜底取该后端的首选模型
				if h.proxyConfig.DefaultModel != "" {
					selectedModel = h.proxyConfig.DefaultModel
					logger.Info("Using configured default model",
						zap.String("model", selectedModel),
						zap.String("requested_model", requestedModel))
				} else if preferred := backend.PreferredDefaultModel(backendCfg); preferred != "" {
					selectedModel = preferred
					logger.Info("Using backend preferred model as default_model fallback",
						zap.String("model", selectedModel),
						zap.String("backend_id", backendCfg.ID),
						zap.String("requested_model", requestedModel))
				} else {
					selectedModel = requestedModel
				}
			}
		} else {
			if tenantID != "" {
				backendCfg, err = h.backendManager.SelectDefaultBackendByTenant(tenantID)
			} else {
				backendCfg, err = h.backendManager.SelectDefaultBackend()
			}
			selectedModel = requestedModel
		}
	} else {
		// 其他模式：检查环境变量强制指定
		forceBackendID := os.Getenv("FORCE_BACKEND_ID")
		if forceBackendID != "" {
			logger.Info("Attempting to use forced backend",
				zap.String("backend_id", forceBackendID))
			if tenantID != "" {
				backendCfg, err = h.backendManager.GetByTenant(tenantID, forceBackendID)
			} else {
				backendCfg, err = h.backendManager.Get(forceBackendID)
			}
			if err != nil {
				logger.Warn("Forced backend not found, fallback to weight-based selection",
					zap.String("backend_id", forceBackendID),
					zap.Error(err))
				if tenantID != "" {
					backendCfg, err = h.backendManager.SelectDefaultBackendByTenant(tenantID)
				} else {
					backendCfg, err = h.backendManager.SelectDefaultBackend()
				}
			} else {
				logger.Info("Using forced backend",
					zap.String("backend_id", backendCfg.ID),
					zap.String("backend_name", backendCfg.Name),
					zap.String("backend_type", backendCfg.Type))
			}
		} else {
			if tenantID != "" {
				backendCfg, err = h.backendManager.SelectDefaultBackendByTenant(tenantID)
			} else {
				backendCfg, err = h.backendManager.SelectDefaultBackend()
			}
		}
		selectedModel = requestedModel
	}

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "No available backend",
				"type":    "server_error",
			},
		})
		statusCode = http.StatusServiceUnavailable
		return
	}

	logger.Infof("Centag request to backend: %s (%s), type: %s, model: %s",
		backendCfg.ID, backendCfg.Name, backendCfg.Type, selectedModel)

	// 如果使用了默认模型，需要修改请求中的model字段
	if selectedModel != "" && selectedModel != requestedModel {
		req["model"] = selectedModel
		logger.Info("Model overridden",
			zap.String("requested_model", requestedModel),
			zap.String("actual_model", selectedModel))
	}

	// 构建目标URL - 根据后端类型调整路径
	var targetURL string
	baseURL := strings.TrimSuffix(backendCfg.BaseURL, "/")
	if backendCfg.Type == "ollama" {
		baseURL = backend.NormalizeOllamaAPIBase(backendCfg.BaseURL)
		// Ollama使用原生API路径，需要转换OpenAI格式路径
		switch c.Request.URL.Path {
		case "/v1/chat/completions", "/api/v1/openai/chat/completions":
			// OpenAI聊天完成接口 -> Ollama原生聊天接口
			targetURL = baseURL + "/api/chat"
			logger.Info("Ollama backend: converting path",
				zap.String("from", c.Request.URL.Path),
				zap.String("to", "/api/chat"),
				zap.String("targetURL", targetURL))
		case "/v1/models", "/api/v1/openai/models":
			// OpenAI模型列表接口 -> Ollama模型列表接口
			targetURL = baseURL + "/api/tags"
			logger.Info("Ollama backend: converting /v1/models -> /api/tags")
		case "/v1/completions":
			// OpenAI文本完成接口 -> Ollama生成接口
			targetURL = baseURL + "/api/generate"
			logger.Info("Ollama backend: converting /v1/completions -> /api/generate")
		default:
			// 其他路径保持不变（避免 BaseURL 与路径前缀重复，仅精确 /v1 或 /v1/ 前缀）
			path := c.Request.URL.Path
			lowBase := strings.ToLower(baseURL)
			if strings.HasSuffix(lowBase, "/v1") && (path == "/v1" || strings.HasPrefix(path, "/v1/")) {
				path = path[3:]
				if path == "" {
					path = "/"
				}
			}
			targetURL = baseURL + path
		}
	} else {
		// OpenAI兼容后端，保持原路径（避免 BaseURL 末尾 /v1 与请求路径前缀重复）
		path := c.Request.URL.Path
		lowBase := strings.ToLower(baseURL)
		if strings.HasSuffix(lowBase, "/v1") && (path == "/v1" || strings.HasPrefix(path, "/v1/")) {
			path = path[3:]
			if path == "" {
				path = "/"
			}
		}
		targetURL = baseURL + path
	}

	// 转换请求体（如果是Ollama后端）
	var requestBody []byte
	if backendCfg.Type == "ollama" && (c.Request.URL.Path == "/v1/chat/completions" || c.Request.URL.Path == "/api/v1/openai/chat/completions") {
		// 转换OpenAI格式到Ollama格式
		logger.Info("Converting OpenAI request to Ollama format")
		requestBody, err = convertOpenAIToOllamaRequest(req)
		if err != nil {
			logger.Warn("Failed to convert request to Ollama format, using original", zap.Error(err))
			requestBody = bodyBytes
		} else {
			logger.Info("Request converted successfully", zap.Int("body_size", len(requestBody)))
		}
	} else {
		requestBody = bodyBytes
	}

	// 使用 HTTP 客户端工具发送请求
	client := httpclient.NewClient(30 * time.Second)
	reqConfig := &httpclient.RequestConfig{
		Method:  c.Request.Method,
		URL:     targetURL,
		Body:    requestBody,
		Headers: map[string]string{},
	}

	// 设置认证
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey)
	if apiKey != "" {
		if c.Request.Header.Get("Authorization") == "" {
			reqConfig.Headers["Authorization"] = "Bearer " + apiKey
		}
	}

	// 执行请求,转发原始请求头
	logger.Info("Sending request to backend",
		zap.String("url", targetURL),
		zap.String("method", c.Request.Method),
		zap.Int("body_size", len(requestBody)))

	resp, err := client.DoWithHeaders(c.Request.Context(), reqConfig, c.Request.Header)
	if err != nil {
		logger.Error("Failed to call backend", zap.Error(err), zap.String("url", targetURL))
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Failed to call backend: %v", err),
				"type":    "api_error",
			},
		})
		statusCode = http.StatusBadGateway
		return
	}

	logger.Info("Backend response received",
		zap.Int("status", resp.StatusCode),
		zap.Int("body_size", len(resp.Body)))

	// 记录状态码
	statusCode = resp.StatusCode

	// 检查是否为流式请求
	isStream := getBool(req, "stream", false) || isStreamRequest(c.Request)

	// 转换响应体（如果是Ollama后端）
	var responseBody []byte
	if backendCfg.Type == "ollama" && resp.StatusCode == http.StatusOK &&
		(c.Request.URL.Path == "/v1/chat/completions" || c.Request.URL.Path == "/api/v1/openai/chat/completions") {

		if isStream {
			// 流式响应：需要转换Ollama SSE格式到OpenAI SSE格式
			logger.Info("Converting Ollama streaming response to OpenAI format")
			h.convertOllamaStreamToOpenAI(c, resp.Body, getString(req, "model", "unknown"))
			return // 流式响应直接返回，不继续处理
		} else {
			// 非流式响应：转换JSON格式
			logger.Info("Converting Ollama response to OpenAI format")
			responseBody, err = convertOllamaToOpenAIResponse(resp.Body, getString(req, "model", "unknown"))
			if err != nil {
				logger.Error("Failed to convert Ollama response to OpenAI format", zap.Error(err))
				responseBody = resp.Body
			} else {
				logger.Info("Response converted successfully", zap.Int("converted_size", len(responseBody)))
			}
		}
	} else {
		responseBody = resp.Body
		if backendCfg.Type == "ollama" && resp.StatusCode != http.StatusOK {
			logger.Warn("Ollama response not OK, not converting",
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(resp.Body)))
		}
	}

	// 返回响应
	// 设置响应头
	if isStream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")
	} else {
		// 非流式响应，设置JSON content type
		c.Header("Content-Type", "application/json")
	}

	// 复制响应头 - 但如果是Ollama后端且已转换，不复制原始headers
	if !(backendCfg.Type == "ollama" && resp.StatusCode == http.StatusOK &&
		(c.Request.URL.Path == "/v1/chat/completions" || c.Request.URL.Path == "/api/v1/openai/chat/completions")) {
		// 只在没有转换时复制原始headers
		for k, v := range resp.Headers {
			if k != "Content-Length" { // Content-Length会自动处理
				c.Header(k, v[0])
			}
		}
	}

	// 缓存响应(非流式且成功响应)
	// 检查是否启用缓存写入
	enableCacheWrite := h.cacheConfig == nil || h.cacheConfig.EnableCacheWrite
	saveOnlyMode := h.cacheConfig != nil && h.cacheConfig.SaveOnlyMode

	if h.proxyCache != nil && h.proxyCache.IsEnabled() && enableCacheWrite && !isStream && resp.StatusCode == http.StatusOK {
		// 添加panic recovery，防止缓存错误导致整个请求失败
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic in cache operation",
					zap.Any("panic", r),
					zap.Stack("stack"))
			}
		}()

		// 解析请求元数据
		reqMetadata, err := cache.ParseRequestMetadata(req)
		if err == nil {
			// 生成缓存键
			model := getString(req, "model", h.fallbackModelName())
			messages := getSlice(req, "messages", []interface{}{})
			temperature := getFloat(req, "temperature", 0.7)
			maxTokens := getInt(req, "max_tokens", 0)

			rf, tc, seed := cacheKeyProtocolFields(req)
			cacheKey, err := h.proxyCache.GetRequestKey(model, messages, temperature, maxTokens, rf, tc, seed)
			if err == nil {
				// 构建请求文本(用于语义匹配的embedding)
				requestText := h.proxyCache.GetRequestQuery(messages)
				// 将原始请求文本存入metadata，用于语义缓存
				reqMetadata["request_text"] = requestText
				reqMetadata = attachProxyCacheContext(c, reqMetadata)
				// 标记是否为仅保存模式
				if saveOnlyMode {
					reqMetadata["save_only"] = true
				}

				// 仅保存模式：跳过正常的缓存写入和拆分流程
				if saveOnlyMode {
					logger.Info("Save-only mode: saving QA data without cache hit capability",
						zap.String("cache_key", cacheKey),
						zap.Int("response_size", len(responseBody)))
					// 直接保存到数据库，不走缓存命中流程（ttl=0 永久保存）
					if err := h.proxyCache.SetSaveOnlyResponse(c.Request.Context(), cacheKey, requestText, string(responseBody), reqMetadata, 0); err != nil {
						logger.Warn("Failed to save-only cache response", zap.Error(err))
					} else {
						logger.Info("Save-only mode: response saved successfully", zap.String("cache_key", cacheKey))
					}
					// 不进行缓存命中流程，也不进行QA拆分
					c.Status(resp.StatusCode)
					_, err = c.Writer.Write(responseBody)
					if err != nil {
						logger.Error("Failed to write response body", zap.Error(err))
					}
					return
				}

				logger.Debug("Attempting to cache response",
					zap.String("cache_key", cacheKey),
					zap.Int("response_size", len(responseBody)))
				ttl := 3600 * time.Second
				if err := h.proxyCache.SetResponse(c.Request.Context(), cacheKey, string(responseBody), reqMetadata, ttl); err != nil {
					logger.Warn("Failed to cache response", zap.Error(err))
				} else {
					logger.Info("Response cached successfully", zap.String("cache_key", cacheKey))

					// 缓存成功后，如果启用了问答拆分，进行拆分处理
					// 仅保存模式下不进行问答拆分
					if h.proxyCache.ShouldSplitQA(saveOnlyMode) && requestText != "" {
						// 使用独立的context,避免请求结束导致context被取消
						qaCtx := context.Background()
						go h.processQASplit(qaCtx, requestText, string(responseBody), model, cacheKey)
					}
				}
			}
		}
	} else if h.proxyCache != nil && h.proxyCache.IsEnabled() && saveOnlyMode && !isStream && resp.StatusCode == http.StatusOK {
		// SaveOnlyMode 模式下，即使 EnableCacheWrite 为 false，也执行仅保存
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic in save-only operation",
					zap.Any("panic", r),
					zap.Stack("stack"))
			}
		}()

		reqMetadata, err := cache.ParseRequestMetadata(req)
		if err == nil {
			model := getString(req, "model", h.fallbackModelName())
			messages := getSlice(req, "messages", []interface{}{})
			temperature := getFloat(req, "temperature", 0.7)
			maxTokens := getInt(req, "max_tokens", 0)

			rf, tc, seed := cacheKeyProtocolFields(req)
			cacheKey, err := h.proxyCache.GetRequestKey(model, messages, temperature, maxTokens, rf, tc, seed)
			if err == nil {
				requestText := h.proxyCache.GetRequestQuery(messages)
				reqMetadata["request_text"] = requestText
				reqMetadata = attachProxyCacheContext(c, reqMetadata)
				reqMetadata["save_only"] = true

				logger.Info("Save-only mode (cache disabled): saving QA data",
					zap.String("cache_key", cacheKey),
					zap.Int("response_size", len(responseBody)))
				if err := h.proxyCache.SetSaveOnlyResponse(c.Request.Context(), cacheKey, requestText, string(responseBody), reqMetadata, 0); err != nil {
					logger.Warn("Failed to save-only cache response", zap.Error(err))
				} else {
					logger.Info("Save-only mode: response saved successfully", zap.String("cache_key", cacheKey))
				}
			}
		}
	}

	// 返回状态码和响应体 - 使用转换后的响应体
	logger.Info("Returning response",
		zap.Int("status", resp.StatusCode),
		zap.Int("body_size", len(responseBody)),
		zap.String("body_preview", string(responseBody[:min(100, len(responseBody))])))

	// 直接写入状态码和响应体，不再设置Content-Type (已在上面设置)
	c.Status(resp.StatusCode)
	_, err = c.Writer.Write(responseBody)
	if err != nil {
		logger.Error("Failed to write response body", zap.Error(err))
	}

	// 仅保存模式 + 流式响应：从 SSE 中提取完整内容并写入仅保存列表（与 /v1/chat/completions 行为一致）
	if saveOnlyMode && isStream && resp.StatusCode == http.StatusOK && h.proxyCache != nil && h.proxyCache.IsEnabled() && len(responseBody) > 0 {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic in save-only stream", zap.Any("panic", r))
				}
			}()
			fullContent := extractContentFromSSE(responseBody)
			if fullContent == "" {
				return
			}
			model := getString(req, "model", h.fallbackModelName())
			messages := getSlice(req, "messages", []interface{}{})
			temperature := getFloat(req, "temperature", 0.7)
			maxTokens := getInt(req, "max_tokens", 0)
			rf, tc, seed := cacheKeyProtocolFields(req)
			cacheKey, err := h.proxyCache.GetRequestKey(model, messages, temperature, maxTokens, rf, tc, seed)
			if err != nil {
				logger.Warn("Save-only stream: failed to get cache key", zap.Error(err))
				return
			}
			requestText := h.proxyCache.GetRequestQuery(messages)
			fullResponse := buildOpenAIResponseFromContent(fullContent, model)
			metadata := map[string]interface{}{
				"model":        model,
				"temperature":  getFloat(req, "temperature", 0.7),
				"max_tokens":   maxTokens,
				"request_text": requestText,
				"save_only":    true,
			}
			if err := h.proxyCache.SetSaveOnlyResponse(context.Background(), cacheKey, requestText, fullResponse, metadata, 0); err != nil {
				logger.Warn("Save-only stream: failed to save", zap.Error(err))
			} else {
				logger.Info("Save-only mode: stream response saved successfully", zap.String("cache_key", cacheKey))
			}
		}()
	}

	// 记录未命中统计
	if !cached {
		h.recordCacheStats(model, "MISS", isStream, statusCode, time.Since(start))
	}
}

// extractContentFromSSE 从 OpenAI SSE 流中提取并拼接所有 delta.content
func extractContentFromSSE(body []byte) string {
	var buf strings.Builder
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		first, _ := choices[0].(map[string]interface{})
		delta, _ := first["delta"].(map[string]interface{})
		if c, ok := delta["content"].(string); ok && c != "" {
			buf.WriteString(c)
		}
	}
	return buf.String()
}

// buildOpenAIResponseFromContent 将完整内容包装成 OpenAI 非流式响应 JSON
func buildOpenAIResponseFromContent(content, model string) string {
	resp := map[string]interface{}{
		"id":      "chatcmpl-saveonly-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
	}
	raw, _ := json.Marshal(resp)
	return string(raw)
}

// isStreamRequest 判断是否是流式请求
func isStreamRequest(r *http.Request) bool {
	if r.Method == "POST" && r.Header.Get("Content-Type") == "application/json" {
		var body map[string]interface{}
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		if json.Unmarshal(bodyBytes, &body) == nil {
			if stream, ok := body["stream"].(bool); ok && stream {
				return true
			}
		}
	}
	return false
}

// forwardGetRequest 转发GET请求
func (h *LLMProxyHandler) forwardGetRequest(c *gin.Context, backendCfg *backend.BackendConfig) {
	logger.Infof("Centag GET request to backend: %s (%s)", backendCfg.ID, backendCfg.Name)

	// 构建目标URL（避免 BaseURL 末尾 /v1 与请求路径前缀重复）
	baseURL := strings.TrimSuffix(backendCfg.BaseURL, "/")
	path := c.Request.URL.Path
	lowBase := strings.ToLower(baseURL)
	// 精确匹配 /v1 或 /v1/...，避免误伤 /v1beta/... 等路径
	if strings.HasSuffix(lowBase, "/v1") && (path == "/v1" || strings.HasPrefix(path, "/v1/")) {
		path = path[3:]
		if path == "" {
			path = "/"
		}
	}
	targetURL := baseURL + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// 使用 HTTP 客户端工具发送请求
	client := httpclient.NewClient(30 * time.Second)
	reqConfig := &httpclient.RequestConfig{
		Method:  "GET",
		URL:     targetURL,
		Headers: map[string]string{},
	}

	// 设置认证
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey)
	if apiKey != "" {
		if c.Request.Header.Get("Authorization") == "" {
			reqConfig.Headers["Authorization"] = "Bearer " + apiKey
		}
	}

	// 执行请求,转发原始请求头
	resp, err := client.DoWithHeaders(c.Request.Context(), reqConfig, c.Request.Header)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Failed to call backend: %v", err),
				"type":    "api_error",
			},
		})
		return
	}

	// 复制响应头
	for k, v := range resp.Headers {
		c.Header(k, v[0])
	}

	// 返回状态码和响应体
	c.Status(resp.StatusCode)
	c.Writer.Write(resp.Body)
}

// 辅助函数
func getString(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getInt(m map[string]interface{}, key string, defaultValue int) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}

func getFloat(m map[string]interface{}, key string, defaultValue float64) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return defaultValue
}

func getBool(m map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func getSlice(m map[string]interface{}, key string, defaultValue []interface{}) []interface{} {
	if v, ok := m[key]; ok {
		if slice, ok := v.([]interface{}); ok {
			return slice
		}
	}
	return defaultValue
}

// cacheKeyProtocolFields 提取缓存键所需的 P0 协议字段（v0.2.8 R13/G4）。
// response_format / tool_choice / seed 必须纳入缓存键，防止不同参数命中相同缓存。
// 未提供的字段返回 nil，由 GetRequestKey 省略（key 与旧版一致）。
func cacheKeyProtocolFields(req map[string]interface{}) (responseFormat interface{}, toolChoice interface{}, seed interface{}) {
	if v, ok := req["response_format"]; ok && v != nil {
		responseFormat = v
	}
	if v, ok := req["tool_choice"]; ok && v != nil {
		toolChoice = v
	}
	if v, ok := req["seed"]; ok && v != nil {
		seed = v
	}
	return
}

// convertOpenAIToOllamaRequest 转换OpenAI格式请求到Ollama格式
func convertOpenAIToOllamaRequest(openaiReq map[string]interface{}) ([]byte, error) {
	messages := getSlice(openaiReq, "messages", []interface{}{})

	// 转换消息格式：将多模态content转换为字符串
	convertedMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		convertedMsg := map[string]interface{}{
			"role": msgMap["role"],
		}

		// 处理 content 字段：支持字符串或数组格式
		content := msgMap["content"]
		switch v := content.(type) {
		case string:
			// 已经是字符串，直接使用
			convertedMsg["content"] = v
		case []interface{}:
			// 数组格式（多模态），提取文本内容
			var texts []string
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if text, ok := itemMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
			convertedMsg["content"] = strings.Join(texts, " ")
		default:
			// 其他类型，转为字符串
			convertedMsg["content"] = fmt.Sprintf("%v", v)
		}

		convertedMessages[i] = convertedMsg
	}

	ollamaReq := map[string]interface{}{
		"model":    getString(openaiReq, "model", ""),
		"messages": convertedMessages,
		"stream":   getBool(openaiReq, "stream", false), // 保留原始stream参数
	}

	// 可选参数
	if temp, ok := openaiReq["temperature"]; ok {
		ollamaReq["temperature"] = temp
	}
	if topP, ok := openaiReq["top_p"]; ok {
		ollamaReq["top_p"] = topP
	}
	if maxTokens, ok := openaiReq["max_tokens"]; ok {
		ollamaReq["num_predict"] = maxTokens
	}

	return json.Marshal(ollamaReq)
}

// convertOllamaToOpenAIResponse 转换Ollama格式响应到OpenAI格式
func convertOllamaToOpenAIResponse(ollamaRespBody []byte, model string) ([]byte, error) {
	var ollamaResp map[string]interface{}
	if err := json.Unmarshal(ollamaRespBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ollama response: %w", err)
	}

	// 提取消息内容
	content := ""
	if msg, ok := ollamaResp["message"].(map[string]interface{}); ok {
		content = getString(msg, "content", "")
	}

	// 构建OpenAI格式响应
	openaiResp := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
	}

	return json.Marshal(openaiResp)
}

// convertOllamaStreamToOpenAI 转换Ollama流式响应到OpenAI SSE格式
func (h *LLMProxyHandler) convertOllamaStreamToOpenAI(c *gin.Context, ollamaStream []byte, model string) {
	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	// 创建flush writer
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("Streaming not supported")
		return
	}

	// 解析Ollama流式响应
	lines := strings.Split(string(ollamaStream), "\n")
	chatID := fmt.Sprintf("chatcmpl-%d", time.Now().Unix())
	isFirstChunk := true // 标记是否第一个chunk

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析Ollama的JSON行
		var ollamaChunk map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ollamaChunk); err != nil {
			logger.Warn("Failed to parse Ollama chunk", zap.Error(err), zap.String("line", line))
			continue
		}

		// 提取内容
		content := ""
		if msg, ok := ollamaChunk["message"].(map[string]interface{}); ok {
			content = getString(msg, "content", "")
		}

		// 检查是否完成
		done := getBool(ollamaChunk, "done", false)
		finishReason := ""
		if done {
			finishReason = "stop"
		}

		// 构建delta对象
		delta := map[string]interface{}{
			"content": content,
		}

		// 只在第一个有内容的chunk中添加role
		if isFirstChunk && content != "" && !done {
			delta["role"] = "assistant"
			isFirstChunk = false
		}

		// 构建OpenAI SSE格式
		openaiChunk := map[string]interface{}{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         delta,
					"finish_reason": finishReason,
				},
			},
		}

		// 序列化并发送
		chunkJSON, err := json.Marshal(openaiChunk)
		if err != nil {
			logger.Error("Failed to marshal OpenAI chunk", zap.Error(err))
			continue
		}

		// SSE格式: data: {json}\n\n
		fmt.Fprintf(w, "data: %s\n\n", string(chunkJSON))
		flusher.Flush()

		// 如果完成，发送[DONE]
		if done {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			break
		}
	}
}

// streamCachedResponse 流式返回缓存的响应
func (h *LLMProxyHandler) streamCachedResponse(c *gin.Context, entry *cache.CacheEntry) {
	// 从缓存数据中提取内容
	var content string

	// 尝试从完整响应中解析
	var openaiResp map[string]interface{}
	if err := json.Unmarshal([]byte(entry.Response), &openaiResp); err == nil {
		if choices, ok := openaiResp["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if contentStr, ok := message["content"].(string); ok {
						content = contentStr
					}
				}
			}
		}
	}

	// 如果解析失败,从流式数据合并
	if content == "" && len(entry.StreamData) > 0 {
		for _, chunk := range entry.StreamData {
			if !chunk.Done {
				content += chunk.Content
			}
		}
	}

	// 按字符分割流式返回
	// 注意: 使用 rune 而不是字节索引，避免切割 UTF-8 字符导致乱码
	runes := []rune(content)
	chunkSize := 10
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		data := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   getMetadataValue(entry.Metadata, "model", "unknown"),
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": string(runes[i:end]),
					},
					"finish_reason": nil,
				},
			},
		}

		c.Writer.WriteString("data: ")
		c.Writer.WriteString(toJSON(data))
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()
		time.Sleep(10 * time.Millisecond)
	}

	// 发送[DONE]
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

// recordCacheStats 记录缓存统计(统一入口)
func (h *LLMProxyHandler) recordCacheStats(model, cacheStatus string, isStream bool, statusCode int, latency time.Duration) {
	// 记录到统一统计
	stats.GlobalUnifiedStats.RecordRequest(cacheStatus, isStream, statusCode, model)

	logger.Debugf("Cache stats recorded - model: %s, status: %s, isStream: %t, statusCode: %d",
		model, cacheStatus, isStream, statusCode)
}

// getMetadataValue 从metadata中获取值
func getMetadataValue(metadata map[string]interface{}, key string, defaultValue string) string {
	if metadata == nil {
		return defaultValue
	}
	if value, ok := metadata[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

// toJSON 将对象转换为JSON字符串
func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// processQASplit 处理问答拆分（异步）
func (h *LLMProxyHandler) processQASplit(ctx context.Context, question, answer, model, originalCacheKey string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in processQASplit", zap.Any("recover", r))
		}
	}()

	// 获取拆分器
	qaSplitter := h.proxyCache.GetQASplitter()
	splitter, ok := qaSplitter.(*processor.QASplitter)
	if !ok {
		logger.Warn("Invalid QA splitter type")
		return
	}

	logger.Info("Starting QA split process",
		zap.String("question", utils.TruncateString(question, 100)),
		zap.String("original_cache_key", originalCacheKey))

	// 调用拆分器
	result, err := splitter.SplitQA(ctx, question, answer)
	if err != nil {
		logger.Warn("Failed to split QA", zap.Error(err))
		return
	}

	// 如果没有拆分，直接返回
	if !result.Split {
		logger.Debug("QA not split by model")
		return
	}

	logger.Info("QA split completed",
		zap.Int("qa_pairs_count", len(result.QAPairs)),
		zap.String("original_cache_key", originalCacheKey))

	// 将拆分后的问答对缓存
	for i, pair := range result.QAPairs {
		if err := h.cacheSplitQAPair(ctx, pair, model, originalCacheKey, i); err != nil {
			logger.Warn("Failed to cache split QA pair",
				zap.Int("index", i),
				zap.Error(err))
		}
	}
}

// cacheSplitQAPair 缓存拆分后的问答对
func (h *LLMProxyHandler) cacheSplitQAPair(ctx context.Context, pair processor.QAPair, model, originalCacheKey string, index int) error {
	// 构建响应（模拟OpenAI格式的响应）
	response := map[string]interface{}{
		"id":      "split-" + generateID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": pair.Answer,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     estimateTokens(pair.Question),
			"completion_tokens": estimateTokens(pair.Answer),
			"total_tokens":      estimateTokens(pair.Question) + estimateTokens(pair.Answer),
		},
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// 构建缓存键（使用拆分后的问题）
	splitMessages := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": pair.Question,
		},
	}

	splitCacheKey, err := h.proxyCache.GetRequestKey(model, splitMessages, 0.7, 0, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to generate split cache key: %w", err)
	}

	// 构建元数据
	reqMetadata := map[string]interface{}{
		"model":              model,
		"original_cache_key": originalCacheKey,
		"split_index":        index,
		"request_text":       pair.Question,
	}

	// 缓存拆分后的问答对
	ttl := 3600 * time.Second
	if err := h.proxyCache.SetResponse(ctx, splitCacheKey, string(responseJSON), reqMetadata, ttl); err != nil {
		return fmt.Errorf("failed to cache split QA pair: %w", err)
	}

	logger.Info("Split QA pair cached",
		zap.String("split_cache_key", splitCacheKey),
		zap.Int("split_index", index),
		zap.String("question", utils.TruncateString(pair.Question, 50)))

	return nil
}

// attachProxyCacheContext fills session_id / request_id for cache management filters (best-effort).
func attachProxyCacheContext(c *gin.Context, metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	sessionID := ""
	if c != nil {
		if v, ok := c.Get("conversation_session_id"); ok {
			if s, ok := v.(string); ok {
				sessionID = strings.TrimSpace(s)
			}
		}
		if sessionID == "" {
			sessionID = strings.TrimSpace(c.GetHeader("X-Session-ID"))
		}
		metadata = cache.AttachRequestContextMetadata(metadata, sessionID, strings.TrimSpace(c.GetHeader("X-Request-ID")), "")
	}
	return metadata
}

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

// estimateTokens 估算Token数量（简单实现）
func estimateTokens(text string) int {
	// 简单估算：中文字符约等于1 token，英文单词约等于1.3 tokens
	runeCount := len([]rune(text))
	wordCount := len(strings.Fields(text))
	return int(float64(wordCount)*1.3) + runeCount/3
}

package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"centag/core/internal/agent"
	"centag/core/internal/auth"
	"centag/core/internal/cache"
	"centag/core/internal/stats"
	"centag/core/internal/tokenusage"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/metrics"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	"centag/core/pkg/processor"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 代理处理器
type Handler struct {
	proxy                   *Proxy
	pluginManager           *plugin.Manager
	proxyCache              *cache.ProxyCache
	questionProcessor       processor.QuestionProcessor      // 问题拆分处理器（可为 nil）
	tokenUsageService       *tokenusage.Service              // Token 计量（可为 nil）
	pipelineEngine          PipelineEngineInterface          // 流水线引擎（可为 nil）
	pipelineRegistry        *pipeline.PipelineRegistry       // 流水线注册表（可为 nil，供 /v1/models）
	modeDispatcher          *ModeDispatcher                  // 统一模式分发器（Phase 2 新增，可为 nil）
	businessPluginRegistry  *pipeline.BusinessPluginRegistry // 业务插件注册表（可为 nil）
	defaultPipelineResolver *DefaultPipelineResolver         // 默认流水线解析器（可为 nil）
}

// PipelineEngineInterface 流水线引擎接口（避免循环依赖）
type PipelineEngineInterface interface {
	Execute(ctx context.Context, pipelineID string, input *pipeline.PipelineInput) (*pipeline.PipelineOutput, error)
	HasPipeline(pipelineID string) bool
	RegisterPipeline(pipeline *pipeline.AgentPatternPipeline) error
	// ExecuteStream 流式执行流水线
	ExecuteStream(ctx context.Context, pipelineID string, input *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error)
}

// NewHandler 创建代理处理器
func NewHandler(proxy *Proxy, pluginManager *plugin.Manager, proxyCache *cache.ProxyCache) *Handler {
	return &Handler{
		proxy:         proxy,
		pluginManager: pluginManager,
		proxyCache:    proxyCache,
	}
}

// SetTokenUsageService 注入 Token 计量服务（可为 nil，表示不记录用量）
func (h *Handler) SetTokenUsageService(s *tokenusage.Service) {
	h.tokenUsageService = s
}

// SetQuestionProcessor 注入问题拆分处理器（热更新友好）
func (h *Handler) SetQuestionProcessor(qp processor.QuestionProcessor) {
	h.questionProcessor = qp
}

// SetPipelineEngine 注入流水线引擎
func (h *Handler) SetPipelineEngine(engine PipelineEngineInterface) {
	h.pipelineEngine = engine

	// Phase 2: 初始化 ModeDispatcher（如果引擎不为空）
	if engine != nil {
		h.modeDispatcher = NewModeDispatcher(engine, nil, nil)
		// 注入协议插件管理器（用于流式 SSE 格式化）
		if h.pluginManager != nil {
			h.modeDispatcher.SetPluginManager(h.pluginManager)
		}
		// 启用 Stream Fake 默认配置
		h.modeDispatcher.SetStreamFakeConfig(DefaultStreamFakeConfig())
		logger.Infof("[Handler] ModeDispatcher initialized with pipeline engine")
	}
}

// SetPipelineRegistry 注入流水线注册表（与策略管理列表同源，供动态解析快捷码与 /v1/models）。
func (h *Handler) SetPipelineRegistry(registry *pipeline.PipelineRegistry) {
	h.pipelineRegistry = registry
	if h.modeDispatcher != nil {
		h.modeDispatcher.SetRegistry(registry)
		logger.Infof("[Handler] ModeDispatcher registry injected")
	}
}

// SetPipelineStore 注入流水线存储（供 ModeDispatcher 动态解析快捷码）
func (h *Handler) SetPipelineStore(store pipeline.PipelineStore) {
	if h.modeDispatcher != nil {
		h.modeDispatcher.SetStore(store)
		logger.Infof("[Handler] ModeDispatcher store injected")
	}
}

// SetBusinessPluginRegistry 注入业务插件注册表（用于插件化问答拆分/合成/任务检测）
func (h *Handler) SetBusinessPluginRegistry(registry *pipeline.BusinessPluginRegistry) {
	h.businessPluginRegistry = registry
}

// SetDefaultPipelineResolver 注入默认流水线解析器
func (h *Handler) SetDefaultPipelineResolver(resolver *DefaultPipelineResolver) {
	h.defaultPipelineResolver = resolver
	logger.Infof("[Handler] DefaultPipelineResolver injected")
}

// HandleChatCompletions 处理 Chat Completions 请求
func (h *Handler) HandleChatCompletions(c *gin.Context) {
	startTime := time.Now()

	// 记录请求基本信息
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// 写回请求头，供 ModeDispatcher / 流水线 metadata 使用同一 request_id。
	c.Request.Header.Set("X-Request-ID", requestID)
	logRequestInfo(requestID, "[Chat Completions] request started",
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
	)

	// 1. 检测代理模式和缓存控制
	proxyMode, source := DetectProxyMode(c.Request)
	cacheControl := DetectCacheControl(c.Request)

	logRequestInfo(requestID, "[Config] proxy mode",
		zap.String("proxy_mode", string(proxyMode)),
		zap.String("source", source),
		zap.Bool("cache_read", cacheControl.Read),
		zap.Bool("cache_write", cacheControl.Write),
		zap.Bool("qa_split", cacheControl.QASplit),
		zap.Bool("save_only", cacheControl.SaveOnly),
	)

	// 2. 如果是默认模式，解析实际应该使用的流水线
	if proxyMode == ModeDefault && h.defaultPipelineResolver != nil {
		model := extractModelFromRequest(c.Request)
		userID := extractUserIDFromContext(c)
		tenantID := extractTenantIDFromContext(c)

		resolvedMode, pipelineSource, err := h.defaultPipelineResolver.ResolveProxyMode(
			c.Request.Context(), model, userID, tenantID,
		)
		if err == nil {
			proxyMode = resolvedMode
			source = pipelineSource
			logRequestInfo(requestID, fmt.Sprintf("[Config] Resolved pipeline: %s (source: %s)", proxyMode, source),
				zap.String("resolved_mode", string(proxyMode)),
				zap.String("resolved_source", source),
			)
		} else {
			logRequestWarn(requestID, fmt.Sprintf("[Config] Failed to resolve default pipeline: %v, using configured default", err))
			proxyMode, _ = h.defaultPipelineResolver.FallbackMode()
		}
	}

	// 3. 记录代理模式到响应头
	c.Header("X-Proxy-Mode", string(proxyMode))
	c.Header("X-Cache-Read", fmt.Sprintf("%v", cacheControl.Read))
	c.Header("X-Cache-Write", fmt.Sprintf("%v", cacheControl.Write))
	c.Header("X-QA-Split", fmt.Sprintf("%v", cacheControl.QASplit))

	// 根据请求路径选择协议插件
	protocolPluginName := "openai-protocol"
	switch {
	case strings.HasSuffix(c.Request.URL.Path, "/messages"):
		protocolPluginName = "anthropic-protocol"
	case strings.HasSuffix(c.Request.URL.Path, "/responses"):
		protocolPluginName = "responses-protocol"
	case strings.Contains(c.Request.URL.Path, "/v1beta/models/"):
		protocolPluginName = "gemini-protocol"
	}
	logger.Infof("[Protocol] Using protocol plugin: %s", protocolPluginName)

	// 存储协议名称到上下文，供 ModeDispatcher 选择 SSE 格式化器
	c.Set("protocol_plugin", protocolPluginName)

	// Phase 3: 根据 Agent 类型查找供应商配置，注入 backend_id 和 pipeline_id
	h.applyAgentProviderConfig(c, protocolPluginName)

	// 缓存原始 body（协议解析会消费 Body；#t 透明转发需要原始 JSON）
	cacheRawRequestBody(c)

	// 获取协议插件
	protocol, err := h.pluginManager.GetProtocol(protocolPluginName)
	if err != nil {
		logger.Errorf("[Protocol] Failed to get protocol plugin %s: %v", protocolPluginName, err)
		c.JSON(500, gin.H{
			"error": fmt.Sprintf("Failed to get protocol plugin: %v", err),
		})
		h.recordCacheStats("", "BYPASS", false, 500, time.Since(startTime))
		return
	}

	// 解析请求
	req, err := protocol.ParseRequest(c)
	if err != nil {
		logRequestError(requestID, "[Request] failed to parse request", zap.Error(err))
		c.JSON(400, gin.H{
			"error": fmt.Sprintf("Failed to parse request: %v", err),
		})
		h.recordCacheStats("", "BYPASS", false, 400, time.Since(startTime))
		return
	}

	messagesPreview := formatMessagesPreview(req.Messages, defaultMessagesPreviewMax)
	logRequestInfo(requestID, "[Request] details",
		zap.String("model", req.Model),
		zap.Bool("stream", req.Stream),
		zap.Float64("temperature", req.Temperature),
		zap.Int("max_tokens", req.MaxTokens),
		zap.Int("message_count", len(req.Messages)),
		zap.Bool("has_system_message", len(req.Messages) > 0 && req.Messages[0].Role == "system"),
		zap.String("messages_preview", messagesPreview),
	)

	// 验证请求
	if err := protocol.ValidateRequest(req); err != nil {
		logRequestError(requestID, "[Request] invalid request", zap.String("model", req.Model), zap.Error(err))
		c.JSON(400, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		h.recordCacheStats(req.Model, "BYPASS", false, 400, time.Since(startTime))
		return
	}

	// 入口门禁：无可用大模型后端时直接返回规范提示（避免落入流水线后出现难懂的 not found）
	if mgr := backend.GetManager(); mgr != nil && !mgr.HasUsableLLMBackend() {
		var ce *backend.ClientError
		if !mgr.HasEnabledBackends() {
			ce = backend.ClassifyClientError(backend.NewNoUsableBackendError(nil))
			logRequestWarn(requestID, "[Backend] no usable llm backend configured")
		} else {
			ce = backend.ClassifyClientError(backend.NewNoBackendAPIKeyError(""))
			logRequestWarn(requestID, "[Backend] enabled backends exist but none usable (missing API key?)")
		}
		writeBackendClientError(c, ce)
		h.recordCacheStats(req.Model, "BYPASS", false, ce.HTTPStatus, time.Since(startTime))
		return
	}

	// Phase 3: 统一使用 ModeDispatcher 作为唯一入口（移除 legacy fallback 路径）
	if h.modeDispatcher != nil {
		c.Header("X-Dispatch-Path", "mode-dispatcher")
		// [v0.2.8 R05] 集中到 helper 拷贝字段，避免分散遗漏新增显式字段
		proxyReq := copyProxyRequestFields(req)

		if req.Stream {
			// 流式请求：走流式分发路径
			if err := h.modeDispatcher.DispatchStream(c, proxyMode, proxyReq); err == nil {
				logRequestComplete(requestID, req.Model, "", c.Writer.Status(), startTime, "")
				return
			} else {
				logRequestError(requestID, "[ModeDispatcher] stream failed",
					zap.String("model", req.Model), zap.String("proxy_mode", string(proxyMode)), zap.Error(err))
				// 流式路径可能已写出 SSE（含 FormatError）；再写 JSON 500 会触发
				// "Headers were already written"，客户端则一直挂起。
				if c.Writer.Written() {
					h.recordCacheStats(req.Model, "BYPASS", false, c.Writer.Status(), time.Since(startTime))
					return
				}
				if writeClassifiedBackendError(c, err) {
					h.recordCacheStats(req.Model, "BYPASS", false, c.Writer.Status(), time.Since(startTime))
					return
				}
				c.JSON(500, gin.H{"error": fmt.Sprintf("ModeDispatcher stream failed: %v", err)})
				h.recordCacheStats(req.Model, "BYPASS", false, 500, time.Since(startTime))
				return
			}
		} else {
			// 非流式请求：走原有非流式路径
			if err := h.modeDispatcher.Dispatch(c, proxyMode, proxyReq); err == nil {
				logRequestComplete(requestID, req.Model, "", c.Writer.Status(), startTime, "")
				return
			} else {
				logRequestError(requestID, "[ModeDispatcher] failed",
					zap.String("model", req.Model), zap.String("proxy_mode", string(proxyMode)), zap.Error(err))
				if writeClassifiedBackendError(c, err) {
					h.recordCacheStats(req.Model, "BYPASS", false, c.Writer.Status(), time.Since(startTime))
					return
				}
				c.JSON(500, gin.H{"error": fmt.Sprintf("ModeDispatcher failed: %v", err)})
				h.recordCacheStats(req.Model, "BYPASS", false, 500, time.Since(startTime))
				return
			}
		}
	}

	// ModeDispatcher 不可用：返回错误（不再 fallback 到 legacy 路径）
	logger.Errorf("[ModeDispatcher] ModeDispatcher is not initialized, cannot process request")
	c.JSON(500, gin.H{"error": "ModeDispatcher is not initialized"})
	h.recordCacheStats(req.Model, "BYPASS", false, 500, time.Since(startTime))
}

func buildReconstructedStreamChunkData(model string, chunk plugin.StreamChunk) map[string]interface{} {
	// 优先在原始 chunk 基础上做“最小改写”，保留 tool_calls / role / finish_reason 等结构字段。
	if len(chunk.RawData) > 0 {
		var raw map[string]interface{}
		if err := json.Unmarshal(chunk.RawData, &raw); err == nil {
			ensureStreamChunkShape(raw, model, chunk)
			return raw
		}
	}

	// 回退：无法解析原始 JSON 时构造最小可用 OpenAI chunk。
	return map[string]interface{}{
		"id":      "chatcmpl-" + generateRandomID(),
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": chunk.Content,
				},
				"finish_reason": normalizeFinishReason(chunk.FinishReason),
			},
		},
	}
}

func ensureStreamChunkShape(raw map[string]interface{}, model string, chunk plugin.StreamChunk) {
	if _, ok := raw["id"]; !ok {
		raw["id"] = "chatcmpl-" + generateRandomID()
	}
	if _, ok := raw["object"]; !ok {
		raw["object"] = "chat.completion.chunk"
	}
	if _, ok := raw["created"]; !ok {
		raw["created"] = 0
	}
	if _, ok := raw["model"]; !ok || raw["model"] == "" {
		raw["model"] = model
	}

	choices, ok := raw["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		raw["choices"] = []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": chunk.Content,
				},
				"finish_reason": normalizeFinishReason(chunk.FinishReason),
			},
		}
		return
	}

	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		choice0 = map[string]interface{}{}
		choices[0] = choice0
	}

	delta, ok := choice0["delta"].(map[string]interface{})
	if !ok || delta == nil {
		delta = map[string]interface{}{}
		choice0["delta"] = delta
	}
	delta["content"] = chunk.Content
	// 避免客户端同时看到 content + reasoning_* 重复展示。
	delete(delta, "reasoning_content")
	delete(delta, "reasoning_details")
	delete(delta, "thinking")
	delete(delta, "thought")

	choice0["finish_reason"] = normalizeFinishReason(chunk.FinishReason)
}

func normalizeFinishReason(finishReason string) interface{} {
	if finishReason == "" {
		return nil
	}
	return finishReason
}

// recordCacheStats 记录缓存统计(统一入口)
func (h *Handler) recordCacheStats(model, cacheStatus string, isStream bool, statusCode int, latency time.Duration) {
	// 记录到统一统计
	stats.GlobalUnifiedStats.RecordRequest(cacheStatus, isStream, statusCode, model)

	// 记录到旧版metrics(向后兼容)
	if metrics.GlobalMetrics != nil {
		cached := (cacheStatus == "HIT-EXACT" || cacheStatus == "HIT-SEMANTIC")
		metrics.GlobalMetrics.RecordRequest(model, statusCode, latency, cached)
	}

	logger.Debugf("Cache stats recorded - model: %s, status: %s, isStream: %t, statusCode: %d",
		model, cacheStatus, isStream, statusCode)
}

// generateRandomID 生成随机ID
func generateRandomID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

// CustomStrategyWeights 自定义策略权重（用于解耦 proxy → strategy 包依赖）
type CustomStrategyWeights struct {
	NameSimilarity float64
	CapacityMatch  float64
	FamilyMatch    float64
	Strictness     int
	Tolerance      float64
}

// CustomStrategyResolver 自定义策略解析函数类型
type CustomStrategyResolver func(id string) *CustomStrategyWeights

var globalCustomStrategyResolver CustomStrategyResolver

// SetCustomStrategyResolver 注册自定义策略解析函数（由 server 在初始化时调用）
func SetCustomStrategyResolver(fn CustomStrategyResolver) {
	globalCustomStrategyResolver = fn
}

func resolveCustomStrategy(id string) *CustomStrategyWeights {
	if globalCustomStrategyResolver != nil {
		return globalCustomStrategyResolver(id)
	}
	return nil
}

// buildMatchingConfig 根据策略 ID 构建 ModelMatchingConfig
func buildMatchingConfig(strategyID string) backend.ModelMatchingConfig {
	// 从全局配置读取基础参数
	cfg := config.Get()
	base := backend.DefaultModelMatchingConfig()
	if cfg != nil {
		if cfg.ModelMatching.Strategy != "" {
			base.Strategy = backend.ModelMatchStrategy(cfg.ModelMatching.Strategy)
		}
		if cfg.ModelMatching.DefaultStrictness > 0 {
			base.DefaultStrictness = cfg.ModelMatching.DefaultStrictness
		}
		if cfg.ModelMatching.CapacityTolerance > 0 {
			base.CapacityTolerance = cfg.ModelMatching.CapacityTolerance
		}
		if w := cfg.ModelMatching.HybridWeights; w.NameSimilarity+w.CapacityMatch+w.FamilyMatch > 0 {
			base.HybridWeights = backend.HybridWeights{
				NameSimilarity: w.NameSimilarity,
				CapacityMatch:  w.CapacityMatch,
				FamilyMatch:    w.FamilyMatch,
			}
		}
	}

	if strategyID == "" {
		return base
	}

	// 请求头指定了策略，覆盖策略 ID
	switch backend.ModelMatchStrategy(strategyID) {
	case backend.StrategyExact, backend.StrategyFamily,
		backend.StrategyCapacity, backend.StrategyHybrid:
		base.Strategy = backend.ModelMatchStrategy(strategyID)
		return base
	}

	// 尝试自定义策略（需要导入 strategy 包）
	// 通过接口解耦：在 proxy 包中不直接导入 strategy 包，而是由外部注入
	// 此处通过全局注册函数解析
	if custom := resolveCustomStrategy(strategyID); custom != nil {
		base.Strategy = backend.StrategyHybrid
		base.HybridWeights = backend.HybridWeights{
			NameSimilarity: custom.NameSimilarity,
			CapacityMatch:  custom.CapacityMatch,
			FamilyMatch:    custom.FamilyMatch,
		}
		if custom.Strictness > 0 {
			base.DefaultStrictness = custom.Strictness
		}
		if custom.Tolerance > 0 {
			base.CapacityTolerance = custom.Tolerance
		}
		logger.Infof("[Smart Scheduling] Resolved custom strategy %q: name=%.2f cap=%.2f family=%.2f",
			strategyID, custom.NameSimilarity, custom.CapacityMatch, custom.FamilyMatch)
	} else {
		logger.Warnf("[Smart Scheduling] Unknown strategy %q, using config default", strategyID)
	}
	return base
}

// ListBackends 列出所有可用的后端
func (h *Handler) ListBackends(c *gin.Context) {
	backendMgr := backend.GetManager()
	backends := backendMgr.GetAll()

	// 转换为响应格式
	data := make([]gin.H, len(backends))
	for i, b := range backends {
		data[i] = gin.H{
			"id":               b.ID,
			"name":             b.Name,
			"type":             b.Type,
			"base_url":         b.BaseURL,
			"api_key":          "****", // 隐藏API密钥
			"is_enabled":       b.Enabled,
			"supported_models": b.SupportedModels,
		}
	}

	c.JSON(200, gin.H{
		"object": "list",
		"count":  len(backends),
		"data":   data,
	})
}

// copyProxyRequestFields 构造 ModeDispatcher 入口的 ProxyRequest，集中管理字段拷贝。
// [v0.2.8 R05] 新增显式字段时强制同步更新该处；G7：Modalities/TopK 为 P2 占位，本轮不拷贝。
func copyProxyRequestFields(req *plugin.ProxyRequest) *plugin.ProxyRequest {
	return &plugin.ProxyRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		RawBody:     req.RawBody,
		// P0/P1 显式字段
		Tools:             req.Tools,
		ToolChoice:        req.ToolChoice,
		ResponseFormat:    req.ResponseFormat,
		Seed:              req.Seed,
		N:                 req.N,
		User:              req.User,
		ParallelToolCalls: req.ParallelToolCalls,
		Reasoning:         req.Reasoning,
	}
}

// ListModels 列出当前可用的流水线与后端模型，以 OpenAI models 格式返回。
// 客户端应使用 id（如 pipeline.direct-backend 或 gpt-4o）作为 chat/completions 的 model。
// [v0.2.8] 同时输出 Pipeline ID（pipeline.*）和配置后端实际支持的模型列表，支持 ?type=chat|embedding|all 过滤。
func (h *Handler) ListModels(c *gin.Context) {
	modelType := c.DefaultQuery("type", "chat") // chat | embedding | all

	pipelines := h.listModelsPipelines()
	data := make([]gin.H, 0, len(pipelines))
	for _, p := range pipelines {
		if p == nil || strings.TrimSpace(p.ID) == "" {
			continue
		}
		data = append(data, gin.H{
			"id":       "pipeline." + p.ID,
			"object":   "model",
			"created":  0,
			"owned_by": "centag",
		})
	}

	// [v0.2.8] 追加启用后端的 supported_models（去重，owned_by=后端名）
	if mgr := backend.GetManager(); mgr != nil {
		seen := make(map[string]struct{})
		for _, b := range mgr.GetAll() {
			if b == nil || !b.Enabled {
				continue
			}
			for _, sm := range b.SupportedModels {
				name := sm.ActualModel
				if name == "" {
					name = sm.RequestedModel
				}
				if name == "" {
					continue
				}
				if _, dup := seen[name]; dup {
					continue
				}
				// 根据 modelType 过滤
				if modelType != "all" {
					if modelType == "embedding" && !isEmbeddingModelName(name) {
						continue
					}
					if modelType == "chat" && isEmbeddingModelName(name) {
						continue
					}
				}
				seen[name] = struct{}{}
				data = append(data, gin.H{
					"id":       name,
					"object":   "model",
					"created":  0,
					"owned_by": b.Name,
				})
			}
		}
	}

	c.JSON(200, gin.H{
		"object": "list",
		"data":   data,
	})
}

// isEmbeddingModelName 判断是否是向量化模型（与 server.isEmbeddingModel 等价，proxy 层内联实现避免跨层依赖）
func isEmbeddingModelName(modelName string) bool {
	modelLower := strings.ToLower(modelName)
	embeddingKeywords := []string{
		"embedding",
		"embed",
		"bge",
		"gte",
		"e5",
		"sentence",
		"nomic-embed",
		"mxbai-embed",
		"all-minilm",
	}
	for _, keyword := range embeddingKeywords {
		if strings.Contains(modelLower, keyword) {
			return true
		}
	}
	return false
}

func (h *Handler) listModelsPipelines() []*pipeline.AgentPatternPipeline {
	if h == nil {
		return nil
	}
	if h.pipelineRegistry != nil {
		return h.pipelineRegistry.List()
	}
	if h.modeDispatcher != nil && h.modeDispatcher.registry != nil {
		return h.modeDispatcher.registry.List()
	}
	return nil
}

// processQASplit 处理问答拆分
// processQASplit 执行 QA 拆分，并根据结果决定语义缓存策略：
//   - 拆分成功（多对）：子问题各自通过 cacheSplitQAPair 写入语义缓存，主请求不重复写入
//   - 未拆分（原子问题）或拆分失败：将主请求补写到语义缓存（因 manager.Set 中已被跳过）
//

const (
	TaskTypeCodeGeneration   = "code_generation"
	TaskTypeTranslation      = "translation"
	TaskTypeComplexReasoning = "complex_reasoning"
	TaskTypeLongText         = "long_text"
	TaskTypeCreative         = "creative"
	TaskTypeAnalysis         = "analysis"
	TaskTypeSimpleChat       = "simple_chat"
)

// detectTaskTypeFromContent 根据问题内容检测任务类型
// 优先使用 tasktype_detector 插件，不可用时回退到内置关键字匹配。
// 独立调用可使用 tasktype_detector.DetectTaskType(content) 函数。
func (h *Handler) detectTaskTypeFromContent(content string) string {
	// 优先尝试插件系统
	if h.businessPluginRegistry != nil {
		if taskType := h.detectTaskTypeFromPlugin(content); taskType != "" {
			return taskType
		}
	}

	// 回退到内置关键字匹配
	return h.detectTaskTypeFromKeywords(content)
}

// detectTaskTypeFromPlugin 从插件系统检测任务类型
func (h *Handler) detectTaskTypeFromPlugin(content string) string {
	bizPlugin := h.businessPluginRegistry.GetByImplementation("business.tasktype_detector")
	if bizPlugin == nil {
		return ""
	}

	req := &pipeline.NodeExecutionRequest{
		Config: pipeline.NodeConfig{CustomConfig: map[string]interface{}{}},
		Input:  &pipeline.NodeInput{Content: content},
	}

	resp, err := bizPlugin.Execute(context.Background(), req)
	if err != nil {
		logger.Debugf("[TaskType] tasktype_detector 插件执行失败，回退到关键字匹配: %v", err)
		return ""
	}
	if resp == nil || resp.Output == nil || resp.Output.Metadata == nil {
		return ""
	}

	taskType, ok := resp.Output.Metadata["task_type"].(string)
	if !ok || taskType == "" {
		return ""
	}

	logger.Debugf("[TaskType] 使用插件检测到任务类型: %s", taskType)
	return taskType
}

// detectTaskTypeFromKeywords 内置关键字匹配检测任务类型（原 detectTaskTypeFromContent 逻辑）
func (h *Handler) detectTaskTypeFromKeywords(content string) string {
	lowerContent := strings.ToLower(content)

	// 代码生成关键字
	codeKeywords := []string{"code", "代码", "写代码", "编程", "python", "javascript", "java", "go", "rust", "c++", "php", "ruby", "swift", "kotlin", "写个函数", "写个程序", "帮我写", "怎么写", "debug", "调试", "fix", "error", "bug", "函数", "算法"}
	for _, kw := range codeKeywords {
		if strings.Contains(lowerContent, kw) {
			return TaskTypeCodeGeneration
		}
	}

	// 翻译关键字
	transKeywords := []string{"翻译", "translate", "英译", "中译", "翻译成", "英文", "中文"}
	for _, kw := range transKeywords {
		if strings.Contains(lowerContent, kw) {
			return TaskTypeTranslation
		}
	}

	// 复杂推理关键字
	reasoningKeywords := []string{"证明", "推导", "为什么", "原因", "逻辑", "分析", "推理", "计算", "数学", "物理", "证明", "解释一下", "为什么是"}
	for _, kw := range reasoningKeywords {
		if strings.Contains(lowerContent, kw) {
			return TaskTypeComplexReasoning
		}
	}

	// 长文本关键字
	longTextKeywords := []string{"总结", "摘要", "概括", "长文", "文章", "文档", "报告", "文档分析", "全文", "整篇"}
	for _, kw := range longTextKeywords {
		if strings.Contains(lowerContent, kw) {
			return TaskTypeLongText
		}
	}

	// 创意写作关键字
	creativeKeywords := []string{"写故事", "创作", "写诗", "写歌", "创意", "小说", "散文", "剧本", "歌词"}
	for _, kw := range creativeKeywords {
		if strings.Contains(lowerContent, kw) {
			return TaskTypeCreative
		}
	}

	// 数据分析关键字
	analysisKeywords := []string{"分析", "数据", "图表", "表格", "统计", "报表", "可视化", "dashboard"}
	for _, kw := range analysisKeywords {
		if strings.Contains(lowerContent, kw) {
			return TaskTypeAnalysis
		}
	}

	// 默认返回简单对话
	return TaskTypeSimpleChat
}

// 策略：并行查缓存 + 并行调 LLM（miss），全部就绪后合成响应。
//   - 全部 miss（0 缓存命中）→ return false，交由正常流程处理（避免重复切分开销）
//   - 部分或全部命中 → 对 miss 子问题并行调 LLM，合成后返回
func (h *Handler) getQuestionSplitter(qsCfg config.QuestionSplitConfig) processor.QuestionSplitter {
	if !qsCfg.Enabled {
		return nil
	}

	// 优先尝试插件系统
	if h.businessPluginRegistry != nil {
		if splitter := h.getQuestionSplitterFromPlugin(qsCfg); splitter != nil {
			logger.Infof("[QuestionSplit] 使用插件 question_splitter 提供拆分器")
			return splitter
		}
	}

	// 回退到注入的 questionProcessor（支持 LLM 拆分）
	if h.questionProcessor != nil && qsCfg.LLMSplitEnabled {
		return h.questionProcessor.GetSplitter()
	}
	// 快速规则拆分（无 LLM，< 1ms）
	if qsCfg.FastSplitEnabled {
		splitCfg := &processor.SplitConfig{
			Enabled:             true,
			Strategy:            processor.StrategyRuleBased,
			ComplexityThreshold: qsCfg.ComplexityThreshold,
			MaxSplitCount:       qsCfg.MaxSubQuestions,
			MinSplitLength:      5,
			EnableAutoSplit:     true,
		}
		splitter, err := processor.NewRuleBasedSplitter(splitCfg)
		if err != nil {
			logger.Warnf("[QuestionSplit] 创建规则拆分器失败: %v", err)
			return nil
		}
		return splitter
	}
	return nil
}

// getQuestionSplitterFromPlugin 从插件系统获取问题拆分器
// 如果 question_splitter 插件已注册，则创建适配器包装为 processor.QuestionSplitter
func (h *Handler) getQuestionSplitterFromPlugin(qsCfg config.QuestionSplitConfig) processor.QuestionSplitter {
	bizPlugin := h.businessPluginRegistry.GetByImplementation("business.question_splitter")
	if bizPlugin == nil {
		return nil
	}

	// 构建插件配置
	strategy := "rule"
	if qsCfg.LLMSplitEnabled {
		switch qsCfg.SplitStrategy {
		case "semantic", "llm":
			strategy = "semantic"
		case "hybrid":
			strategy = "hybrid"
		default:
			if qsCfg.SplitStrategy != "" {
				strategy = qsCfg.SplitStrategy
			}
		}
	}

	customConfig := map[string]interface{}{
		"strategy":             strategy,
		"complexity_threshold": float64(qsCfg.ComplexityThreshold),
		"max_split_count":      qsCfg.MaxSubQuestions,
		"min_split_length":     5,
		"enabled":              true,
	}

	return &pluginSplitterAdapter{
		plugin:       bizPlugin,
		customConfig: customConfig,
	}
}

// pluginSplitterAdapter 将 BusinessPlugin 适配为 processor.QuestionSplitter
type pluginSplitterAdapter struct {
	plugin       pipeline.BusinessPlugin
	customConfig map[string]interface{}
	// 缓存上一次 Execute 的结果，避免 ShouldSplit + Split 重复调用
	cachedQuestion string
	cachedShould   bool
	cachedComplex  *processor.ComplexAnalysis
	cachedSubQs    []*processor.SubQuestion
	cachedStrategy processor.SplitStrategy
}

func (a *pluginSplitterAdapter) ShouldSplit(ctx context.Context, question string) (bool, *processor.ComplexAnalysis, error) {
	if err := a.ensureExecuted(ctx, question); err != nil {
		return false, nil, err
	}
	return a.cachedShould, a.cachedComplex, nil
}

func (a *pluginSplitterAdapter) Split(ctx context.Context, question string) ([]*processor.SubQuestion, error) {
	if err := a.ensureExecuted(ctx, question); err != nil {
		return nil, err
	}
	return a.cachedSubQs, nil
}

func (a *pluginSplitterAdapter) GetStrategy() processor.SplitStrategy {
	return a.cachedStrategy
}

// ensureExecuted 执行插件调用并缓存结果
func (a *pluginSplitterAdapter) ensureExecuted(ctx context.Context, question string) error {
	if a.cachedQuestion == question {
		return nil // 已缓存
	}

	req := &pipeline.NodeExecutionRequest{
		Config: pipeline.NodeConfig{
			CustomConfig: a.customConfig,
		},
		Input: &pipeline.NodeInput{
			Content: question,
		},
	}

	resp, err := a.plugin.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("question_splitter plugin execute failed: %w", err)
	}
	if resp == nil || resp.Output == nil {
		return fmt.Errorf("question_splitter plugin returned nil response")
	}

	// 解析响应
	a.cachedQuestion = question

	// should_split
	if v, ok := resp.Output.Metadata["should_split"].(bool); ok {
		a.cachedShould = v
	}

	// complexity_score
	a.cachedComplex = &processor.ComplexAnalysis{}
	if v, ok := resp.Output.Metadata["complexity_score"].(float64); ok {
		a.cachedComplex.ComplexityScore = float32(v)
	}
	if v, ok := resp.Output.Metadata["question_type"].(string); ok {
		a.cachedComplex.QuestionType = processor.QuestionType(v)
	}

	// strategy
	if v, ok := resp.Output.Metadata["strategy"].(string); ok {
		a.cachedStrategy = processor.SplitStrategy(v)
	}

	// sub_questions
	if sqList, ok := resp.Output.Metadata["sub_questions"].([]interface{}); ok {
		a.cachedSubQs = make([]*processor.SubQuestion, 0, len(sqList))
		for i, sq := range sqList {
			if m, ok := sq.(map[string]interface{}); ok {
				subQ := &processor.SubQuestion{Order: i}
				if v, ok := m["id"].(string); ok {
					subQ.ID = v
				}
				if v, ok := m["content"].(string); ok {
					subQ.Content = v
				}
				if v, ok := m["order"].(float64); ok {
					subQ.Order = int(v)
				}
				if deps, ok := m["dependencies"].([]interface{}); ok {
					subQ.Dependencies = make([]string, 0, len(deps))
					for _, d := range deps {
						if s, ok := d.(string); ok {
							subQ.Dependencies = append(subQ.Dependencies, s)
						}
					}
				}
				a.cachedSubQs = append(a.cachedSubQs, subQ)
			}
		}
	}

	return nil
}

// synthesizeSubAnswers 合成多个子答案为最终答案
// 优先使用插件系统中的 answer_synthesizer 插件，不可用时回退到内置实现。
func (h *Handler) synthesizeSubAnswers(strategy, originalQuestion string, subAnswers []*processor.SubAnswer) string {
	if len(subAnswers) == 0 {
		return ""
	}
	if len(subAnswers) == 1 {
		return subAnswers[0].Answer
	}

	// 尝试使用插件系统
	if h.businessPluginRegistry != nil {
		if result := h.synthesizeSubAnswersFromPlugin(strategy, originalQuestion, subAnswers); result != "" {
			return result
		}
	}

	// 回退到内置实现
	return synthesizeSubAnswersBuiltin(strategy, subAnswers)
}

// synthesizeSubAnswersFromPlugin 从插件系统合成子答案
func (h *Handler) synthesizeSubAnswersFromPlugin(strategy, originalQuestion string, subAnswers []*processor.SubAnswer) string {
	bizPlugin := h.businessPluginRegistry.GetByImplementation("business.answer_synthesizer")
	if bizPlugin == nil {
		return ""
	}

	// 构建子答案列表（必须是 []interface{} 才能被 resolveSubAnswers 正确解析）
	subAnswerList := make([]interface{}, 0, len(subAnswers))
	for _, sa := range subAnswers {
		subAnswerList = append(subAnswerList, map[string]interface{}{
			"question_id": sa.QuestionID,
			"question":    sa.Question,
			"answer":      sa.Answer,
			"from_cache":  sa.FromCache,
		})
	}

	customConfig := map[string]interface{}{
		"strategy": strategy,
	}

	req := &pipeline.NodeExecutionRequest{
		Config: pipeline.NodeConfig{
			CustomConfig: customConfig,
		},
		Input: &pipeline.NodeInput{
			Content: originalQuestion,
			Metadata: map[string]interface{}{
				"sub_answers":       subAnswerList,
				"original_question": originalQuestion,
			},
		},
	}

	resp, err := bizPlugin.Execute(context.Background(), req)
	if err != nil {
		logger.Warnf("[QuestionSplit] answer_synthesizer 插件执行失败，回退到内置合成: %v", err)
		return ""
	}
	if resp == nil || resp.Output == nil || resp.Output.Content == "" {
		return ""
	}

	logger.Infof("[QuestionSplit] 使用插件 answer_synthesizer 合成答案")
	return resp.Output.Content
}

// synthesizeSubAnswersBuiltin 内置子答案合成实现（原 synthesizeSubAnswers 函数）
func synthesizeSubAnswersBuiltin(strategy string, subAnswers []*processor.SubAnswer) string {
	switch strategy {
	case "concat", "":
		// 简单拼接（默认）
		var sb strings.Builder
		for i, sa := range subAnswers {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(sa.Answer)
		}
		return sb.String()
	default:
		// 其他策略暂时回退到拼接
		var sb strings.Builder
		for i, sa := range subAnswers {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(sa.Answer)
		}
		return sb.String()
	}
}

// extractQuestionFromMessages 从消息列表中提取用户问题
func extractQuestionFromMessages(messages []plugin.Message) string {
	if len(messages) == 0 {
		return ""
	}
	// 获取最后一条用户消息
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	// 如果没有用户消息，返回最后一条消息
	return messages[len(messages)-1].Content
}

// Legacy audit/optimize/fallback mode handlers have been removed.
// All modes now use pipeline-based implementation (handle*ModePipeline functions).
// See: config/initdata/pipeline-templates/{01-audit-mode,07-optimize-mode,04-fallback-mode}.json

// extractModelFromRequest 从请求中提取模型名
func extractModelFromRequest(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	// 读取请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}

	// 恢复请求体
	r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	// 解析 JSON
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		return ""
	}

	// 提取 model 字段
	model, _ := reqBody["model"].(string)
	return model
}

// extractUserIDFromContext 从上下文提取用户 ID
func extractUserIDFromContext(c *gin.Context) *int64 {
	if id, err := auth.GetUserID(c); err == nil && id != 0 {
		return &id
	}
	return nil
}

// extractTenantIDFromContext 从上下文提取租户 ID
func extractTenantIDFromContext(c *gin.Context) *string {
	if tenantID, exists := c.Get("tenant_id"); exists {
		if id, ok := tenantID.(string); ok {
			return &id
		}
	}
	return nil
}

// applyAgentProviderConfig 根据 Agent 类型查找供应商配置，注入 backend_id 和 pipeline_id
func (h *Handler) applyAgentProviderConfig(c *gin.Context, protocolPluginName string) {
	// 优先使用中间件识别结果
	var agentType string
	if v, ok := c.Get("agent_type"); ok {
		if s, ok := v.(string); ok {
			agentType = strings.ToLower(strings.TrimSpace(s))
		}
	}

	// 回退到协议名称推断
	if agentType == "" {
		switch protocolPluginName {
		case "anthropic-protocol":
			agentType = "claude-code" // /v1/messages 默认对应 Claude Code
		default:
			// OpenAI 协议在无识别结果时不设置 agent type（由模型名路由）
			return
		}
	}

	var tenantID string
	if tid, exists := c.Get("tenant_id"); exists {
		if v, ok := tid.(string); ok {
			tenantID = strings.TrimSpace(v)
		}
	}

	mgr := agent.GetProviderManager()
	cfg, ok := mgr.GetByAgentTypeAndTenant(agentType, tenantID)
	if !ok {
		return // 无供应商配置，使用默认路由
	}

	// 注入 backend_id（如果供应商配置指定了后端）
	if cfg.BackendID != "" {
		c.Request.Header.Set("X-Backend-ID", cfg.BackendID)
		logger.Debugf("[AgentProvider] Agent %s -> backend %s", agentType, cfg.BackendID)
	}

	// 注入 pipeline_id（如果供应商配置指定了流水线）
	if cfg.PipelineID != "" {
		c.Request.Header.Set("X-Pipeline-ID", cfg.PipelineID)
		c.Set("agent_pipeline_id", cfg.PipelineID)
		logger.Debugf("[AgentProvider] Agent %s -> pipeline %s", agentType, cfg.PipelineID)
	}

	// 记录 agent_type 供用量追踪使用
	c.Set("agent_type", agentType)
}

func writeClassifiedBackendError(c *gin.Context, err error) bool {
	ce := backend.ClassifyClientError(err)
	if ce == nil {
		return false
	}
	writeBackendClientError(c, ce)
	return true
}

func writeBackendClientError(c *gin.Context, ce *backend.ClientError) {
	if ce == nil {
		c.JSON(http.StatusServiceUnavailable, backend.OpenAIErrorBody(
			backend.ErrorCodeNoBackendConfigured,
			backend.ClientHintNoBackendConfigured,
		))
		return
	}
	status := ce.HTTPStatus
	if status == 0 {
		status = http.StatusServiceUnavailable
	}
	c.Header("X-Centag-Error-Code", ce.Code)
	c.JSON(status, backend.OpenAIErrorBody(ce.Code, ce.Message))
}

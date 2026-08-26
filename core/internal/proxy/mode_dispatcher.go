package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/metrics"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	"centag/core/pkg/protocol/thinksplit"
	"centag/core/pkg/proxymode"

	"github.com/gin-gonic/gin"
)

// ModeDispatcher 统一模式分发器
// 将所有代理模式请求按用户配置的流水线动态路由到流水线引擎（无内置流水线特例）。
type ModeDispatcher struct {
	pipelineEngine    PipelineEngineInterface
	registry          *pipeline.PipelineRegistry
	store             pipeline.PipelineStore
	resolver          *PipelineResolver
	pluginManager     ProtocolPluginGetter
	streamFakeHandler *StreamFakeHandler
	logger            Logger
}

// ProtocolPluginGetter 协议插件获取接口（避免循环依赖）
type ProtocolPluginGetter interface {
	GetProtocol(name string) (plugin.ProtocolPlugin, error)
}

// Logger 日志接口
type Logger interface {
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// NewModeDispatcher 创建模式分发器
func NewModeDispatcher(
	engine PipelineEngineInterface,
	registry *pipeline.PipelineRegistry,
	logger Logger,
) *ModeDispatcher {
	if logger == nil {
		logger = &defaultLogger{}
	}
	d := &ModeDispatcher{
		pipelineEngine: engine,
		registry:       registry,
		logger:         logger,
	}
	d.refreshResolver()
	return d
}

// SetPluginManager 注入协议插件管理器（用于流式响应格式化）
func (d *ModeDispatcher) SetPluginManager(mgr ProtocolPluginGetter) {
	d.pluginManager = mgr
}

func (d *ModeDispatcher) SetStreamFakeConfig(cfg StreamFakeConfig) {
	d.streamFakeHandler = NewStreamFakeHandler(cfg)
}

func (d *ModeDispatcher) refreshResolver() {
	if d == nil {
		return
	}
	d.resolver = NewPipelineResolver(d.pipelineEngine, d.registry, d.store)
}

// SetRegistry 注入流水线注册表（策略管理列表同源）。
func (d *ModeDispatcher) SetRegistry(registry *pipeline.PipelineRegistry) {
	d.registry = registry
	d.refreshResolver()
}

// defaultLogger 默认日志实现
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, fields ...interface{})  {}
func (l *defaultLogger) Warn(msg string, fields ...interface{})  {}
func (l *defaultLogger) Error(msg string, fields ...interface{}) {}
func (l *defaultLogger) Debug(msg string, fields ...interface{}) {}

// Dispatch 分发请求到对应的流水线
func (d *ModeDispatcher) Dispatch(
	c *gin.Context,
	mode ProxyMode,
	req *plugin.ProxyRequest,
) error {
	pipelineID := d.resolvePipelineID(c, mode)
	if pipelineID == "" {
		return fmt.Errorf("no pipeline configured for mode: %s", mode)
	}
	c.Header("X-Pipeline-ID", pipelineID)

	d.logger.Info("dispatching request to pipeline",
		"mode", mode,
		"pipeline_id", pipelineID,
	)

	execCtx := d.requestExecContext(c)

	// 2. 检查流水线是否存在，如果不存在则尝试从内置模板创建
	if !d.pipelineExists(execCtx, pipelineID) {
		// 尝试从内置模板注册
		if err := d.registerBuiltinPipeline(pipelineID); err != nil {
			return fmt.Errorf("pipeline not found: %s (%w)", pipelineID, err)
		}
		d.logger.Info("auto-registered builtin pipeline",
			"pipeline_id", pipelineID,
		)
	}

	// 3. 构建流水线输入
	input := d.buildPipelineInput(c, req, mode)
	sessionID := triggerConversationRequestHooks(c, req, mode, pipelineID)
	if sessionID != "" && input != nil {
		input.SessionID = sessionID
	}

	// 4. 执行流水线 (stream_fake: 非流式请求走流式管道再聚合)
	var err error
	var output *pipeline.PipelineOutput
	if h := d.streamFakeHandler; h != nil && h.IsEnabled() && !input.Stream {
		d.logger.Info("stream fake enabled, executing as stream pipeline",
			"mode", mode,
			"pipeline_id", pipelineID,
		)
		output, err = d.executeStreamToOutput(execCtx, pipelineID, input, h.config.MaxBytes)
	} else {
		output, err = d.pipelineEngine.Execute(execCtx, pipelineID, input)
	}
	if err != nil {
		d.logger.Error("pipeline execution failed",
			"mode", mode,
			"pipeline_id", pipelineID,
			"error", err,
		)
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// 应用 ThinkSplit: 从 content 中分离 <think>...</think> 推理内容
	if output != nil {
		visible, reasoning := thinksplit.Split(output.Content)
		output.Content = visible
		if reasoning != "" {
			output.ReasoningContent = reasoning
		}
	}

	maybeRecordTokenUsage(c, output, req.Model)
	maybeRecordRouteBackendMetrics(output)
	maybeRecordCacheSaving(c, output, req.Model)
	triggerConversationResponseHooks(c, output, req.Model, sessionID)

	// 5. 构建并返回响应
	return d.writeResponse(c, output, mode, pipelineID)
}

// executeStreamToOutput 执行流式管道并聚合为 PipelineOutput (stream_fake)
func (d *ModeDispatcher) executeStreamToOutput(
	ctx context.Context,
	pipelineID string,
	input *pipeline.PipelineInput,
	maxBytes int64,
) (*pipeline.PipelineOutput, error) {
	input.Stream = true
	resultCh, err := d.pipelineEngine.ExecuteStream(ctx, pipelineID, input)
	if err != nil {
		return nil, err
	}

	aggregator := NewStreamFakeAggregator(maxBytes)
	var finalOutput *pipeline.PipelineOutput

	for result := range resultCh {
		if result.Chunk != nil {
			if result.Chunk.Error != nil {
				return nil, fmt.Errorf("stream fake chunk error: %w", result.Chunk.Error)
			}
			if err := aggregator.Feed(*result.Chunk); err != nil {
				return nil, err
			}
		}
		if result.Output != nil {
			finalOutput = result.Output
		}
	}

	aggResult := aggregator.Result()
	if finalOutput == nil {
		finalOutput = &pipeline.PipelineOutput{}
	}
	// 透明 raw_passthrough 节点只下发 Output、不下发 chunk（streamEmitter 分支），
	// 聚合器结果为空；此时保留 finalOutput 的原始透传内容，避免非流式响应被清空。
	if aggResult.Content != "" {
		finalOutput.Content = aggResult.Content
	}
	if aggResult.ReasoningContent != "" {
		finalOutput.ReasoningContent = aggResult.ReasoningContent
	}
	if aggResult.FinishReason != "" {
		finalOutput.FinishReason = aggResult.FinishReason
	}
	if len(aggResult.ToolCalls) > 0 {
		finalOutput.ToolCalls = pluginToolCallsToPipeline(aggResult.ToolCalls)
	}

	return finalOutput, nil
}

// pluginToolCallsToPipeline 将 plugin.ToolCall 切片转为 pipeline.ToolCall 切片
func pluginToolCallsToPipeline(tcs []plugin.ToolCall) []pipeline.ToolCall {
	if len(tcs) == 0 {
		return nil
	}
	result := make([]pipeline.ToolCall, len(tcs))
	for i, tc := range tcs {
		result[i] = pipeline.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: pipeline.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// SetStore 注入 PipelineStore 以支持动态快捷码解析
func (d *ModeDispatcher) SetStore(store pipeline.PipelineStore) {
	d.store = store
	d.refreshResolver()
}

func (d *ModeDispatcher) resolvePipelineID(c *gin.Context, mode ProxyMode) string {
	if d.resolver == nil {
		d.refreshResolver()
	}
	headerPID := ""
	if c != nil {
		headerPID = c.GetHeader("X-Pipeline-ID")
	}
	basePID, _ := splitForcedRoutePipelineID(headerPID)
	return d.resolver.Resolve(mode, basePID)
}

// splitForcedRoutePipelineID 解析 X-Pipeline-ID 中约定的强制路由后缀。
// 形如 "centag-ops-router:status-check" → ("centag-ops-router", "status-check")。
// 无后缀时返回原值与空串（forced_route 供 router 节点跳过 LLM 分类）。
func splitForcedRoutePipelineID(header string) (base, forcedRoute string) {
	header = strings.TrimSpace(header)
	if idx := strings.Index(header, ":"); idx > 0 {
		base = strings.TrimSpace(header[:idx])
		forcedRoute = strings.TrimSpace(header[idx+1:])
		if base == "" {
			return "", forcedRoute
		}
		return base, forcedRoute
	}
	return header, ""
}

// registerBuiltinPipeline 从内置模板注册流水线
func (d *ModeDispatcher) registerBuiltinPipeline(pipelineID string) error {
	return fmt.Errorf("pipeline %s not found in registry; create it via API or initdata before use", pipelineID)
}

// IsPipelineMode 检查模式是否可解析到已注册的流水线。
func (d *ModeDispatcher) IsPipelineMode(mode ProxyMode) bool {
	return d.resolvePipelineID(nil, mode) != ""
}

// buildPipelineInput 构建流水线输入
func (d *ModeDispatcher) buildPipelineInput(
	c *gin.Context,
	req *plugin.ProxyRequest,
	mode ProxyMode,
) *pipeline.PipelineInput {
	// 提取请求头中的动态配置
	headers := extractHeaders(c)

	// 提取查询参数
	queryParams := extractQueryParams(c)

	// 构建元数据
	metadata := d.buildMetadata(c, mode, headers, queryParams)
	attachTransparentRequestMetadata(c, metadata, req.RawBody)

	// 转换消息格式（透传 tool_calls 和 tool_call_id，避免后端报错）
	messages := make([]pipeline.Message, len(req.Messages))
	for i, msg := range req.Messages {
		m := pipeline.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ToolCallID:       msg.ToolCallID,
			ReasoningContent: msg.ReasoningContent,
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]pipeline.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				m.ToolCalls[j] = pipeline.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: pipeline.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		messages[i] = m
	}

	// 使用最后一条用户消息作为当前问题（多轮对话时不能用首条）
	content := extractQuestionFromMessages(req.Messages)

	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
	}
	input := &pipeline.PipelineInput{
		Content:   content,
		Messages:  messages,
		Stream:    req.Stream,
		Metadata:  metadata,
		UserID:    extractUserID(c),
		SessionID: sessionID,
	}

	// 透传 tools 和 tool_choice（支持 function calling）
	if rawBody, ok := req.RawBody.(map[string]interface{}); ok && rawBody != nil {
		if tools, ok := rawBody["tools"]; ok {
			input.Tools = tools
		}
		if tc, ok := rawBody["tool_choice"]; ok {
			input.ToolChoice = tc
		}
		// 从 centag 扩展字段提取 scene（body > header 优先级覆盖）
		if centag, ok := rawBody["centag"].(map[string]interface{}); ok {
			if scene, ok := centag["scene"].(string); ok && strings.TrimSpace(scene) != "" {
				input.Metadata["scene"] = scene
			}
		}
	}

	return input
}

// buildMetadata 构建元数据
func (d *ModeDispatcher) buildMetadata(
	c *gin.Context,
	mode ProxyMode,
	headers map[string]string,
	queryParams map[string]string,
) map[string]interface{} {
	// 获取虚拟模型名（用于区分流水线模式和直接请求）
	virtualModel := proxymode.GetPipelineModel(proxymode.ExecutionMode(mode))

	metadata := map[string]interface{}{
		"mode":          mode,
		"request_id":    c.GetHeader("X-Request-ID"),
		"client_ip":     c.ClientIP(),
		"user_agent":    c.Request.UserAgent(),
		"timestamp":     time.Now().Format(time.RFC3339),
		"virtual_model": virtualModel, // 虚拟模型名，用于日志和追踪
	}

	// 从请求头提取配置。
	// 注意必须用 requestHeaderValue 大小写不敏感读取：Go MIME 规范化会把
	// X-Backend-ID 存为 X-Backend-Id，直接 headers["X-Backend-ID"] 索引永远落空。
	if backendID := requestHeaderValue(c, headers, "X-Backend-ID"); backendID != "" {
		metadata["backend_id"] = backendID
	}
	if executorBackend := requestHeaderValue(c, headers, "X-Executor-Backend-ID"); executorBackend != "" {
		metadata["executor_backend"] = executorBackend
	}
	if executorModel := requestHeaderValue(c, headers, "X-Executor-Model"); executorModel != "" {
		metadata["executor_model"] = executorModel
	}
	if auditorBackend := requestHeaderValue(c, headers, "X-Auditor-Backend-ID"); auditorBackend != "" {
		metadata["auditor_backend"] = auditorBackend
	}
	if auditorModel := requestHeaderValue(c, headers, "X-Auditor-Model"); auditorModel != "" {
		metadata["auditor_model"] = auditorModel
	}
	if optimizerBackend := requestHeaderValue(c, headers, "X-Optimizer-Backend-ID"); optimizerBackend != "" {
		metadata["optimizer_backend"] = optimizerBackend
	}
	if optimizerModel := requestHeaderValue(c, headers, "X-Optimizer-Model"); optimizerModel != "" {
		metadata["optimizer_model"] = optimizerModel
	}
	if targetURL := requestHeaderValue(c, headers, "X-Target-URL"); targetURL != "" {
		metadata["target_url"] = targetURL
	}
	if geoRegion := requestHeaderValue(c, headers, "X-Geo-Region"); geoRegion != "" {
		metadata["geo_region"] = geoRegion
	}
	if originalHost := requestHeaderValue(c, headers, "X-Original-Host"); originalHost != "" {
		metadata["original_host"] = originalHost
	}
	if originalPath := requestHeaderValue(c, headers, "X-Original-Path"); originalPath != "" {
		metadata["original_path"] = originalPath
	}
	if proxyType := headers["X-Proxy-Type"]; proxyType != "" {
		metadata["proxy_type"] = proxyType
	}
	if pipelineID := headers["X-Pipeline-ID"]; pipelineID != "" {
		basePID, forcedRoute := splitForcedRoutePipelineID(pipelineID)
		metadata["pipeline_id"] = basePID
		if forcedRoute != "" {
			metadata["forced_route"] = forcedRoute
		}
	}

	// Per-request cache switches (X-Cache-Read/Write); consumed by CacheNode via ExecutionContext.
	if c != nil && c.Request != nil {
		cc := DetectCacheControl(c.Request)
		metadata["cache_read"] = cc.Read
		metadata["cache_write"] = cc.Write
		metadata["cache_qa_split"] = cc.QASplit
	}

	if uid := extractUserID(c); uid != "" {
		metadata["user_id"] = uid
	}
	if keyID := auth.GetAPIKeyID(c); keyID > 0 {
		metadata["api_key_id"] = keyID
	}
	if dept := strings.TrimSpace(headers["X-Dept-Tag"]); dept != "" {
		metadata["dept_tag"] = dept
	}
	if tenantID, ok := c.Get("tenant_id"); ok {
		if tid := fmt.Sprintf("%v", tenantID); strings.TrimSpace(tid) != "" {
			metadata["tenant_id"] = tid
		}
	}
	if at, ok := c.Get("agent_type"); ok {
		if agentType := fmt.Sprintf("%v", at); strings.TrimSpace(agentType) != "" {
			metadata["agent_type"] = agentType
		}
	}

	// 从请求头提取 scene（教育等场景路由参数）
	if scene := requestHeaderValue(c, headers, "X-Scene"); scene != "" {
		metadata["scene"] = scene
	}

	// 从查询参数提取配置
	for k, v := range queryParams {
		if strings.EqualFold(strings.TrimSpace(k), "scene") && strings.TrimSpace(v) != "" && metadata["scene"] == nil {
			metadata["scene"] = v
		}
		metadata["param_"+k] = v
	}

	return metadata
}

// writeResponse 写入响应
func (d *ModeDispatcher) writeResponse(
	c *gin.Context,
	output *pipeline.PipelineOutput,
	mode ProxyMode,
	pipelineID string,
) error {
	// 设置响应头
	c.Header("X-Proxy-Mode", string(mode))
	c.Header("X-Pipeline-Executed", "true")

	if output.ExecutionLog != nil {
		c.Header("X-Pipeline-Duration-Ms", fmt.Sprintf("%d", output.ExecutionLog.Duration))
		c.Header("X-Pipeline-Success", fmt.Sprintf("%v", output.ExecutionLog.Success))
	}

	pipeline.ApplyResponseTraceBanner(output, pipelineID)
	setPipelineExecutionHeaders(c, output, pipelineID)
	setPipelineOutputHeaders(c, output)

	if shouldPassthroughTransparentResponse(mode, output) {
		statusCode, contentType := transparentPassthroughStatusAndType(output)
		c.Data(statusCode, contentType, []byte(output.Content))
		return nil
	}

	// 构建响应
	resp := &plugin.ProxyResponse{
		Content:    output.Content,
		TokensUsed: 0,
	}

	totalTokens := 0
	if output.ExecutionLog != nil {
		totalTokens = output.ExecutionLog.TotalTokens
		resp.TokensUsed = totalTokens
	}

	// 空响应防护：上游失败被降级/旁路逻辑聚合成「成功但空」的 output 时，
	// 不得再向客户端返回空 200（会表现为"很快返回但无输出"）。视为上游失败，
	// 返回 502 并透出失败原因，避免客户端把空响应当成功结果消费。
	if isEmptyPipelineOutput(output, totalTokens) {
		hint := pipelineEmptyOutputHint(output)
		if hint == "" {
			hint = "upstream returned an empty response after exhausting retries and fallbacks"
		}
		c.JSON(http.StatusBadGateway, backend.OpenAIErrorBody("upstream_empty_response", hint))
		return nil
	}

	requestID := c.GetHeader("X-Request-ID")
	backendID := extractBackendFromPipelineOutput(output)
	responseModel := effectiveResponseModel(c, "", output)
	logRequestResponse(requestID, responseModel, backendID, http.StatusOK, resp.Content)

	if responseModel == "" {
		responseModel = "pipeline-mode"
	}

	// Responses 客户端（OpenCode/Codex /v1/responses）必须拿到 {object:"response", output:[...]}，
	// 不能静默落成 chat.completion（与 getStreamFormatter 的协议分支对齐）。
	if isResponsesProtocol(c) {
		writeResponsesAPIJSON(c, output, responseModel, totalTokens)
		return nil
	}

	// Anthropic 客户端（Claude Code /v1/messages）必须拿到 Messages API 格式，
	// 不能静默落成 chat.completion，否则客户端 schema 校验失败。
	if isAnthropicProtocol(c) {
		writeAnthropicMessagesJSON(c, output, responseModel, totalTokens)
		return nil
	}

	// 构建 message：有 tool_calls 时输出 tool_calls（function calling），否则输出 content
	message := gin.H{
		"role": "assistant",
	}
	// 透传 reasoning_content（DeepSeek thinking 模式要求多轮回传）
	if output.ReasoningContent != "" {
		message["reasoning_content"] = output.ReasoningContent
	}
	if len(output.ToolCalls) > 0 {
		toolCalls := make([]gin.H, 0, len(output.ToolCalls))
		for _, tc := range output.ToolCalls {
			toolCalls = append(toolCalls, gin.H{
				"id":   tc.ID,
				"type": tc.Type,
				"function": gin.H{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}
		message["tool_calls"] = toolCalls
		// tool_calls 场景下 content 可能为空，OpenAI 规范允许 content 为 null
		if resp.Content != "" {
			message["content"] = resp.Content
		} else {
			message["content"] = nil
		}
	} else {
		message["content"] = resp.Content
	}

	// finish_reason：优先使用 LLM 返回的实际原因，回退到 stop
	finishReason := output.FinishReason
	if finishReason == "" {
		finishReason = "stop"
	}

	// 返回 JSON 响应
	c.JSON(http.StatusOK, gin.H{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   responseModel,
		"choices": []gin.H{
			{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
		"usage": gin.H{
			"prompt_tokens":     totalTokens / 2,
			"completion_tokens": totalTokens / 2,
			"total_tokens":      totalTokens,
		},
	})

	return nil
}

func isResponsesProtocol(c *gin.Context) bool {
	if c == nil {
		return false
	}
	name, _ := c.Get("protocol_plugin")
	s, _ := name.(string)
	return s == "responses-protocol"
}

func isAnthropicProtocol(c *gin.Context) bool {
	if c == nil {
		return false
	}
	name, _ := c.Get("protocol_plugin")
	s, _ := name.(string)
	return s == "anthropic-protocol"
}

func isGeminiProtocol(c *gin.Context) bool {
	if c == nil {
		return false
	}
	name, _ := c.Get("protocol_plugin")
	s, _ := name.(string)
	return s == "gemini-protocol"
}

func isOpenAIProtocol(c *gin.Context) bool {
	if c == nil {
		return true
	}
	return !isResponsesProtocol(c) && !isAnthropicProtocol(c) && !isGeminiProtocol(c)
}

// writeResponsesAPIJSON writes a non-stream OpenAI Responses API envelope.
func writeResponsesAPIJSON(c *gin.Context, output *pipeline.PipelineOutput, model string, totalTokens int) {
	items := make([]gin.H, 0, 1+len(output.ToolCalls))
	content := ""
	if output != nil {
		content = output.Content
	}
	hasTools := output != nil && len(output.ToolCalls) > 0

	// 纯 tool 轮次不伪造空 message；仅文本或空响应保留 message item。
	if content != "" || !hasTools {
		msgID := fmt.Sprintf("msg-%d", time.Now().UnixNano())
		items = append(items, gin.H{
			"id":     msgID,
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []gin.H{
				{"type": "output_text", "text": content, "annotations": []string{}},
			},
		})
	}
	if hasTools {
		for _, tc := range output.ToolCalls {
			callID := strings.TrimSpace(tc.ID)
			if callID == "" {
				callID = fmt.Sprintf("call_%d", time.Now().UnixNano())
			}
			args := tc.Function.Arguments
			if args == "" {
				args = "{}"
			}
			items = append(items, gin.H{
				"id":        "fc_" + callID,
				"type":      "function_call",
				"status":    "completed",
				"call_id":   callID,
				"name":      tc.Function.Name,
				"arguments": args,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         fmt.Sprintf("resp-%d", time.Now().UnixNano()),
		"object":     "response",
		"status":     "completed",
		"model":      model,
		"output":     items,
		"created_at": time.Now().Unix(),
		"usage":      buildResponsesUsage(totalTokens/2, totalTokens/2, totalTokens),
	})
}

// buildResponsesUsage 构建 Responses API 规范的 usage 对象。
// 由于部分客户端（如 Grok Build）使用严格的 serde 反序列化，
// 缺失任何 usage details 字段都会报错；因此全部显式置 0 并发送。
func buildResponsesUsage(inputTokens, outputTokens, totalTokens int) map[string]interface{} {
	emptyDetails := map[string]interface{}{
		"cached_tokens":              0,
		"reasoning_tokens":           0,
		"accepted_prediction_tokens": 0,
		"rejected_prediction_tokens": 0,
		"audio_tokens":               0,
		"text_tokens":                0,
	}
	return map[string]interface{}{
		"input_tokens":          inputTokens,
		"output_tokens":         outputTokens,
		"total_tokens":          totalTokens,
		"input_tokens_details":  emptyDetails,
		"output_tokens_details": emptyDetails,
	}
}

// writeAnthropicMessagesJSON writes a non-stream Anthropic Messages API envelope.
func writeAnthropicMessagesJSON(c *gin.Context, output *pipeline.PipelineOutput, model string, totalTokens int) {
	contentBlocks := make([]gin.H, 0, 1+len(output.ToolCalls))
	if output != nil && output.Content != "" {
		contentBlocks = append(contentBlocks, gin.H{
			"type": "text",
			"text": output.Content,
		})
	}
	for _, tc := range output.ToolCalls {
		var input interface{}
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		if input == nil {
			input = map[string]interface{}{}
		}
		contentBlocks = append(contentBlocks, gin.H{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": input,
		})
	}
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, gin.H{"type": "text", "text": ""})
	}

	stopReason := "end_turn"
	if output != nil {
		switch output.FinishReason {
		case "tool_calls":
			stopReason = "tool_use"
		case "length":
			stopReason = "max_tokens"
		case "stop", "":
			stopReason = "end_turn"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		"type":        "message",
		"role":        "assistant",
		"content":     contentBlocks,
		"model":       model,
		"stop_reason": stopReason,
		"usage": gin.H{
			"input_tokens":  totalTokens / 2,
			"output_tokens": totalTokens / 2,
		},
	})
}

// Helper functions

// requestHeaderValue reads a request header case-insensitively (Go canonicalizes MIME keys).
func requestHeaderValue(c *gin.Context, headers map[string]string, name string) string {
	if c != nil {
		if v := strings.TrimSpace(c.GetHeader(name)); v != "" {
			return v
		}
	}
	for k, v := range headers {
		if strings.EqualFold(k, name) && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func extractHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}

func attachTransparentRequestMetadata(c *gin.Context, metadata map[string]interface{}, rawBody any) {
	if c == nil || c.Request == nil || metadata == nil {
		return
	}
	metadata["request_path"] = c.Request.URL.Path
	metadata["request_method"] = c.Request.Method
	if sid := strings.TrimSpace(c.GetHeader("X-Session-ID")); sid != "" {
		metadata["session_id"] = sid
	}
	// MITM 会把 Centag egress Key 写入 Authorization，原厂 Key 放在 X-Original-Authorization。
	// 透明转发打 Centag 后端时用后端 Key；跳板/固定出站用后端 Key 改写鉴权。
	if orig := strings.TrimSpace(c.GetHeader("X-Original-Authorization")); orig != "" {
		metadata["forward_authorization"] = orig
	} else if auth := strings.TrimSpace(c.GetHeader("Authorization")); auth != "" {
		metadata["forward_authorization"] = auth
	}

	bodyBytes := rawRequestBodyFromContext(c)
	if len(bodyBytes) == 0 && c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
			c.Set(contextKeyRawRequestBody, bodyBytes)
		}
	}
	if len(bodyBytes) == 0 {
		if serialized := rawBodyJSONFromProxyRequest(rawBody); serialized != "" {
			metadata["raw_request_body"] = serialized
		}
		return
	}
	metadata["raw_request_body"] = string(bodyBytes)
}

func shouldPassthroughTransparentResponse(mode ProxyMode, output *pipeline.PipelineOutput) bool {
	if output == nil || strings.TrimSpace(output.Content) == "" {
		return false
	}
	// Explicit false wins (e.g. Responses→Chat rewrite needs protocol formatter).
	if output.Metadata != nil {
		if v, ok := output.Metadata["raw_passthrough"].(bool); ok {
			return v
		}
	}
	// 无显式 metadata 时，仅透明/跳板默认按透传处理；直连若已设 raw_passthrough 则走上面的显式分支。
	switch mode {
	case ModeTransparentProxy, ModeTransparentFast, ModeFixedEgress:
		return true
	default:
		return false
	}
}

func transparentPassthroughStatusAndType(output *pipeline.PipelineOutput) (int, string) {
	statusCode := http.StatusOK
	contentType := "application/json"
	if output != nil && output.Metadata != nil {
		if sc, ok := output.Metadata["status_code"].(int); ok && sc > 0 {
			statusCode = sc
		} else if scf, ok := output.Metadata["status_code"].(float64); ok && scf > 0 {
			statusCode = int(scf)
		}
		if ct, ok := output.Metadata["content_type"].(string); ok && strings.TrimSpace(ct) != "" {
			contentType = strings.TrimSpace(ct)
		}
	}
	// 上游 SSE 被缓冲进 Content 时，必须按 event-stream 写出，不能再包一层 chat.completion.chunk。
	if looksLikeOpenAISSE(output.Content) && !strings.Contains(strings.ToLower(contentType), "event-stream") {
		contentType = "text/event-stream"
	}
	return statusCode, contentType
}

func looksLikeOpenAISSE(body string) bool {
	trim := strings.TrimSpace(body)
	return strings.HasPrefix(trim, "data:") || strings.Contains(body, "\ndata:")
}

// shouldRawWriteTransparentStream：流式路径下是否原样写出 Content。
// 显式 raw_passthrough=false / responses_to_chat / anthropic_to_chat 时必须走协议 FormatChunk
// （/v1/responses、/v1/messages 不能收到 chat.completion.chunk，否则客户端一直转圈或解析失败）。
func shouldRawWriteTransparentStream(output *pipeline.PipelineOutput) bool {
	if output == nil || strings.TrimSpace(output.Content) == "" {
		return false
	}
	if output.Metadata != nil {
		if v, ok := output.Metadata["responses_to_chat"].(bool); ok && v {
			return false
		}
		if v, ok := output.Metadata["anthropic_to_chat"].(bool); ok && v {
			return false
		}
		if rp, ok := output.Metadata["request_path"].(string); ok {
			trimmed := strings.TrimRight(strings.TrimSpace(rp), "/")
			if strings.HasSuffix(trimmed, "/responses") || strings.HasSuffix(trimmed, "/messages") {
				return false
			}
			// Gemini API paths must be re-encoded by the gemini protocol formatter;
			// raw OpenAI SSE/JSON cannot be passed to a Gemini client.
			if strings.Contains(trimmed, "/v1beta/models/") {
				return false
			}
		}
		if v, ok := output.Metadata["raw_passthrough"].(bool); ok {
			return v
		}
	}
	return looksLikeOpenAISSE(output.Content)
}

func extractQueryParams(c *gin.Context) map[string]string {
	params := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	return params
}

func extractUserID(c *gin.Context) string {
	if id, err := auth.GetUserID(c); err == nil && id != 0 {
		return fmt.Sprintf("%d", id)
	}
	return ""
}

// requestExecContext attaches owner scope + per-user proxy defaults for Team users.
func (d *ModeDispatcher) requestExecContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if auth.IsAdmin(c) {
		return ctx
	}
	uid, err := auth.GetUserID(c)
	if err != nil || uid == 0 {
		return ctx
	}
	scope := fmt.Sprintf("user:%d", uid)
	if database.IsInitialized() {
		if user, err := database.Get().UserStore().GetByID(ctx, uid); err == nil && user != nil {
			if user.TenantID != nil && strings.TrimSpace(*user.TenantID) != "" {
				scope = strings.TrimSpace(*user.TenantID)
			}
		}
		if uc, err := database.Get().UserConfigStore().Get(ctx, uid); err == nil && uc != nil && strings.TrimSpace(uc.ProxySettings) != "" {
			var ps struct {
				DefaultBackendID string `json:"default_backend_id"`
				DefaultModel     string `json:"default_model"`
			}
			if json.Unmarshal([]byte(uc.ProxySettings), &ps) == nil {
				ctx = config.WithProxyDefaults(ctx, config.ProxyDefaults{
					DefaultBackendID: strings.TrimSpace(ps.DefaultBackendID),
					DefaultModel:     strings.TrimSpace(ps.DefaultModel),
				})
			}
		}
	}
	return pipeline.WithOwnerScope(ctx, scope)
}

func (d *ModeDispatcher) pipelineExists(ctx context.Context, pipelineID string) bool {
	if d == nil || d.pipelineEngine == nil {
		return false
	}
	type scoped interface {
		HasPipelineContext(context.Context, string) bool
	}
	if s, ok := d.pipelineEngine.(scoped); ok {
		return s.HasPipelineContext(ctx, pipelineID)
	}
	return d.pipelineEngine.HasPipeline(pipelineID)
}

// GetModeMappings 获取所有模式映射
func GetModeMappings() []ModeToPipelineMapping {
	return defaultModeMappings
}

// GetModeMapping 获取指定模式的映射
func GetModeMapping(mode ProxyMode) *ModeToPipelineMapping {
	for _, mapping := range defaultModeMappings {
		if mapping.Mode == mode {
			return &mapping
		}
	}
	return nil
}

// EnableMode 启用指定模式
func EnableMode(mode ProxyMode) {
	for i := range defaultModeMappings {
		if defaultModeMappings[i].Mode == mode {
			defaultModeMappings[i].Enabled = true
			break
		}
	}
}

// DisableMode 禁用指定模式
func DisableMode(mode ProxyMode) {
	for i := range defaultModeMappings {
		if defaultModeMappings[i].Mode == mode {
			defaultModeMappings[i].Enabled = false
			break
		}
	}
}

// SetModePipeline 设置模式的流水线ID
func SetModePipeline(mode ProxyMode, pipelineID string) {
	for i := range defaultModeMappings {
		if defaultModeMappings[i].Mode == mode {
			defaultModeMappings[i].PipelineID = pipelineID
			break
		}
	}
}

// DispatchStream 流式分发请求到流水线，返回 SSE 流式响应
func (d *ModeDispatcher) DispatchStream(
	c *gin.Context,
	mode ProxyMode,
	req *plugin.ProxyRequest,
) error {
	pipelineID := d.resolvePipelineID(c, mode)
	if pipelineID == "" {
		return fmt.Errorf("no pipeline configured for mode: %s", mode)
	}
	c.Header("X-Pipeline-ID", pipelineID)

	d.logger.Info("dispatching stream request to pipeline",
		"mode", mode,
		"pipeline_id", pipelineID,
	)

	execCtx := d.requestExecContext(c)
	if !d.pipelineExists(execCtx, pipelineID) {
		return fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	input := d.buildPipelineInput(c, req, mode)
	sessionID := triggerConversationRequestHooks(c, req, mode, pipelineID)
	if sessionID != "" && input != nil {
		input.SessionID = sessionID
	}

	// 执行流式流水线
	resultCh, err := d.pipelineEngine.ExecuteStream(execCtx, pipelineID, input)
	if err != nil {
		return fmt.Errorf("pipeline stream execution failed: %w", err)
	}

	// 写入 SSE 流式响应（直连/透明等忽略客户端 model 时，回写系统默认模型名，避免 Agent 显示错误模型）
	return d.writeStreamResponse(c, resultCh, mode, effectiveResponseModel(c, req.Model, nil))
}

// shouldSkipPipelineStreamChunk 判断是否跳过流水线流式 chunk。
// 仅跳过无正文且无 tool_calls 的 done 标记；带内容或 tool_calls 的 done chunk 仍需下发给客户端。
func shouldSkipPipelineStreamChunk(chunk *plugin.StreamChunk) bool {
	if chunk == nil {
		return true
	}
	return chunk.Done && strings.TrimSpace(chunk.Content) == "" && len(chunk.ToolCalls) == 0
}

// writeStreamResponse 写入 SSE 流式响应
func (d *ModeDispatcher) writeStreamResponse(
	c *gin.Context,
	resultCh <-chan pipeline.PipelineStreamResult,
	mode ProxyMode,
	model string,
) error {
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Proxy-Mode", string(mode))
	c.Header("X-Pipeline-Executed", "true")
	if pipelineID := c.GetHeader("X-Pipeline-ID"); pipelineID != "" {
		c.Header("X-Pipeline-ID", pipelineID)
	}

	// 确保写入 flush 接口
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported: response writer does not support flushing")
	}

	// 获取协议格式化器（从上下文检测协议类型）
	formatter := d.getStreamFormatter(c)

	responseID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	chunkIndex := 0
	var finalOutput *pipeline.PipelineOutput
	var responseContent strings.Builder
	sseHeadersReady := false

	ensureSSEHeaders := func() {
		if sseHeadersReady {
			return
		}
		c.Header("Content-Type", "text/event-stream")
		sseHeadersReady = true
	}

	splitter := thinksplit.NewStreamSplitter()

	// 逐块消费流式结果
	for result := range resultCh {
		// 透明透传：上游响应（SSE 或 JSON）已在 Content 中，禁止再 FormatChunk 包一层。
		if result.Output != nil {
			pipeline.ApplyResponseTraceBanner(result.Output, c.GetHeader("X-Pipeline-ID"))
			finalOutput = result.Output
			// [二次防护] 即使 metadata 标记缺失，若客户端协议非 OpenAI，禁止直接透传上游 SSE/JSON，
			// 否则 Anthropic/Responses/Gemini 客户端会解析失败或一直转圈。
			if shouldRawWriteTransparentStream(result.Output) && isOpenAIProtocol(c) {
				return d.writeRawPassthroughStream(c, flusher, finalOutput, mode, model)
			}
		}

		if result.Chunk != nil {
			ensureSSEHeaders()
			if result.Chunk.Error != nil {
				// 错误返回 - 按当前协议的 SSE 错误事件规范格式化，避免客户端 schema 校验失败把上游错误吞掉
				errorLine := formatter.FormatError(model, result.Chunk.Error, responseID, created)
				if errorLine != "" {
					fmt.Fprint(c.Writer, errorLine)
					flusher.Flush()
				}
				return result.Chunk.Error
			}

			// 应用 ThinkSplit 流式分离
			if result.Chunk.Content != "" {
				visibleDelta, reasoningDelta := splitter.Feed(result.Chunk.Content)
				result.Chunk.Content = visibleDelta
				if reasoningDelta != "" {
					if result.Chunk.ReasoningContent != "" {
						result.Chunk.ReasoningContent += reasoningDelta
					} else {
						result.Chunk.ReasoningContent = reasoningDelta
					}
				}
			}

			// 跳过无内容的 done 标记；保留带正文的 done chunk（单块响应或 StreamAdapter 末块）
			if shouldSkipPipelineStreamChunk(result.Chunk) {
				continue
			}

			if result.Chunk.Content != "" {
				responseContent.WriteString(result.Chunk.Content)
			}

			// 使用协议格式化器生成 SSE 数据
			dataLine := formatter.FormatChunk(model, result.Chunk, chunkIndex, responseID, created)
			if dataLine != "" {
				fmt.Fprint(c.Writer, dataLine)
				flusher.Flush()
				chunkIndex++
			}
		}

		if result.Output != nil {
			finalOutput = result.Output
		}
	}

	// 透明透传兜底：仅 Output、无 chunk（streamEmitter 跳过 Adapt）时在此写出。
	// [二次防护] 协议非 OpenAI 时禁止直接透传，避免客户端格式不匹配。
	if finalOutput != nil && shouldRawWriteTransparentStream(finalOutput) && isOpenAIProtocol(c) && responseContent.Len() == 0 && !sseHeadersReady {
		pipeline.ApplyResponseTraceBanner(finalOutput, c.GetHeader("X-Pipeline-ID"))
		return d.writeRawPassthroughStream(c, flusher, finalOutput, mode, model)
	}

	ensureSSEHeaders()

	// 刷新 StreamSplitter 剩余内容
	if flushVisible, flushReasoning := splitter.Flush(); flushVisible != "" || flushReasoning != "" {
		flushChunk := &plugin.StreamChunk{
			Content:          flushVisible,
			ReasoningContent: flushReasoning,
		}
		dataLine := formatter.FormatChunk(model, flushChunk, chunkIndex, responseID, created)
		if dataLine != "" {
			fmt.Fprint(c.Writer, dataLine)
			flusher.Flush()
			chunkIndex++
		}
		if flushVisible != "" {
			responseContent.WriteString(flushVisible)
		}
	}

	// 兜底：流式 chunk 全部被跳过但流水线有完整输出时，补发一条正文
	// 注意：raw_passthrough 的 Content 是上游原始 SSE/JSON，绝不能塞进 delta.content。
	if responseContent.Len() == 0 && finalOutput != nil && strings.TrimSpace(finalOutput.Content) != "" &&
		!shouldRawWriteTransparentStream(finalOutput) {
		fallbackChunk := &plugin.StreamChunk{
			Content: finalOutput.Content,
			Done:    true,
		}
		dataLine := formatter.FormatChunk(model, fallbackChunk, 0, responseID, created)
		if dataLine != "" {
			fmt.Fprint(c.Writer, dataLine)
			flusher.Flush()
		}
		responseContent.WriteString(finalOutput.Content)
	}

	// 空响应防护（流式）：一个 chunk 都没写出且聚合 output 为空时，写 SSE 错误
	// 而非空的 [DONE]，避免客户端把"上游失败聚合成空"当成功消费（表现为无输出）。
	if chunkIndex == 0 && responseContent.Len() == 0 && isEmptyPipelineOutput(finalOutput, pipelineOutputTotalTokens(finalOutput)) {
		hint := pipelineEmptyOutputHint(finalOutput)
		if hint == "" {
			hint = "upstream returned an empty response after exhausting retries and fallbacks"
		}
		errorLine := formatter.FormatError(model, fmt.Errorf("%s", hint), responseID, created)
		if errorLine != "" {
			fmt.Fprint(c.Writer, errorLine)
			flusher.Flush()
		}
		return nil
	}

	// 发送 usage 信息和结束标记
	usage := formatter.BuildUsage(finalOutput)
	// 从 finalOutput 提取实际 finish_reason，避免 FormatDone 硬编码 stop
	actualFinishReason := ""
	if finalOutput != nil {
		actualFinishReason = finalOutput.FinishReason
	}
	doneModel := effectiveResponseModel(c, model, finalOutput)
	doneLine := formatter.FormatDone(doneModel, usage, actualFinishReason)
	if doneLine != "" {
		fmt.Fprint(c.Writer, doneLine)
	}

	// Centag 元数据仅对显式订阅读取的客户端注入（WebUI / Centag TUI）。
	// 禁止对 OpenCode 等第三方以 data: JSON 注入 —— 会被当成 chat.completion 校验失败。
	if finalOutput != nil && shouldInjectCentagMetaSSE(c) {
		meta := buildStreamCentagMeta(finalOutput, c.GetHeader("X-Pipeline-ID"), mode)
		if d.logger != nil {
			d.logger.Debug("writeStreamResponse: injecting SSE centag_meta event",
				"pipeline_id", meta["pipeline_id"],
				"has_node_results", meta["node_results"] != nil,
				"duration_ms", meta["duration_ms"],
				"executor_model", meta["executor_model"],
				"last_node", meta["last_node"],
			)
		}
		if metaJSON, err := json.Marshal(meta); err == nil {
			fmt.Fprintf(c.Writer, "event: centag.meta\ndata: %s\n\n", metaJSON)
			flusher.Flush()
		}
	}

	// [DONE] 必须是最后一个 SSE 事件，客户端遇到此标记后停止读取
	if shouldEmitOpenAIDone(c) {
		fmt.Fprint(c.Writer, "data: [DONE]\n\n")
	}
	flusher.Flush()

	// 流结束后补设输出级响应头（WebUI 等在 fetch 完成后读 header 可见）
	if finalOutput != nil {
		setPipelineExecutionHeaders(c, finalOutput, c.GetHeader("X-Pipeline-ID"))
		setPipelineOutputHeaders(c, finalOutput)
	}

	requestID := c.GetHeader("X-Request-ID")
	responseText := responseContent.String()
	if finalOutput != nil && responseText == "" {
		responseText = finalOutput.Content
	}
	backendID := extractBackendFromPipelineOutput(finalOutput)
	logRequestResponse(requestID, model, backendID, http.StatusOK, responseText)

	d.finishStreamSideEffects(c, finalOutput, model)

	return nil
}

// finishStreamSideEffects records metering / conversation hooks after a stream completes.
// Must also run on transparent raw_passthrough early-return paths.
func (d *ModeDispatcher) finishStreamSideEffects(c *gin.Context, finalOutput *pipeline.PipelineOutput, model string) {
	maybeRecordTokenUsage(c, finalOutput, model)
	maybeRecordRouteBackendMetrics(finalOutput)
	maybeRecordCacheSaving(c, finalOutput, model)
	triggerConversationResponseHooks(c, finalOutput, model, "")
}

// writeRawPassthroughStream 原样写出透明节点上游 SSE/JSON。
// 关键缺陷：此前先写 body 再设降级头，且提前 return 跳过了 centag.meta，
// 导致对话测试无法感知 FallbackGroup / billing fallback。
func (d *ModeDispatcher) writeRawPassthroughStream(
	c *gin.Context,
	flusher http.Flusher,
	finalOutput *pipeline.PipelineOutput,
	mode ProxyMode,
	model string,
) error {
	if finalOutput == nil {
		return fmt.Errorf("raw passthrough: empty output")
	}
	statusCode, contentType := transparentPassthroughStatusAndType(finalOutput)

	// 必须在 Write 之前设头，否则流式响应客户端读不到 X-Fallback-* / X-Backend-ID。
	setPipelineExecutionHeaders(c, finalOutput, c.GetHeader("X-Pipeline-ID"))
	setPipelineOutputHeaders(c, finalOutput)
	c.Header("Content-Type", contentType)
	c.Status(statusCode)

	content := finalOutput.Content
	injectMeta := shouldInjectCentagMetaSSE(c)
	if injectMeta {
		// 上游 SSE 常自带 data: [DONE]；客户端遇 DONE 即停读，meta 必须插在 DONE 之前。
		content = stripTrailingOpenAISSEDone(content)
	}
	if _, err := c.Writer.Write([]byte(content)); err != nil {
		return err
	}
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		fmt.Fprint(c.Writer, "\n")
	}
	flusher.Flush()

	if injectMeta {
		meta := buildStreamCentagMeta(finalOutput, c.GetHeader("X-Pipeline-ID"), mode)
		if d.logger != nil {
			d.logger.Debug("writeRawPassthroughStream: injecting SSE centag_meta event",
				"pipeline_id", meta["pipeline_id"],
				"fallback_used", meta["fallback_used"],
				"has_node_results", meta["node_results"] != nil,
				"executor_model", meta["executor_model"],
				"last_node", meta["last_node"],
			)
		}
		if metaJSON, err := json.Marshal(meta); err == nil {
			fmt.Fprintf(c.Writer, "event: centag.meta\ndata: %s\n\n", metaJSON)
			flusher.Flush()
		}
		if shouldEmitOpenAIDone(c) {
			fmt.Fprint(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
		}
	}

	requestID := c.GetHeader("X-Request-ID")
	respModel := effectiveResponseModel(c, model, finalOutput)
	backendID := extractBackendFromPipelineOutput(finalOutput)
	logRequestResponse(requestID, respModel, backendID, statusCode, finalOutput.Content)
	d.finishStreamSideEffects(c, finalOutput, respModel)
	return nil
}

// stripTrailingOpenAISSEDone 去掉正文末尾的 data: [DONE]，便于在其后注入 centag.meta 再补 DONE。
func stripTrailingOpenAISSEDone(body string) string {
	trimmed := strings.TrimRight(body, " \t\r\n")
	for _, suffix := range []string{"data: [DONE]", "data:[DONE]", "data: [done]", "data:[done]"} {
		if len(trimmed) >= len(suffix) && strings.EqualFold(trimmed[len(trimmed)-len(suffix):], suffix) {
			return strings.TrimRight(trimmed[:len(trimmed)-len(suffix)], " \t\r\n")
		}
	}
	return body
}

// nodeResultSummary 单个流水线节点的执行摘要。
type nodeResultSummary struct {
	NodeID     string `json:"node_id"`
	NodeType   string `json:"type"`
	Model      string `json:"model,omitempty"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
	Tokens     int    `json:"tokens,omitempty"`
}

// buildNodeResultsSummary 从 PipelineOutput 构建节点执行摘要列表。
// 优先使用 ExecutionLog.NodeLogs（含精确的节点级耗时/token），
// 回退到 NodeOutputs（仅含输出数据元信息）。
//
// 同一 node_id 可能先失败再被「补成功日志」：若 Metadata 标明 FallbackGroup 恢复，
// 主节点摘要保留失败态，避免对话测试把主路画成成功、备用画成跳过。
func buildNodeResultsSummary(output *pipeline.PipelineOutput) []nodeResultSummary {
	if output == nil {
		return nil
	}

	fallbackFromNode := ""
	fallbackUsed := false
	if output.Metadata != nil {
		if v, ok := output.Metadata["fallback_from_node"].(string); ok {
			fallbackFromNode = strings.TrimSpace(v)
		}
		switch v := output.Metadata["fallback_used"].(type) {
		case bool:
			fallbackUsed = v
		case string:
			fallbackUsed = strings.EqualFold(strings.TrimSpace(v), "true") || v == "1"
		}
		if output.Metadata["billing_fallback_used"] == true {
			fallbackUsed = true
		}
	}

	// 用 NodeLogs 做索引（包含准确耗时/模型/token 信息）
	if output.ExecutionLog != nil && len(output.ExecutionLog.NodeLogs) > 0 {
		type agg struct {
			sum        nodeResultSummary
			sawFail    bool
			sawSuccess bool
		}
		byNode := map[string]*agg{}
		order := make([]string, 0)
		for _, nl := range output.ExecutionLog.NodeLogs {
			a, ok := byNode[nl.NodeID]
			if !ok {
				a = &agg{sum: nodeResultSummary{NodeID: nl.NodeID, NodeType: string(nl.NodeType)}}
				byNode[nl.NodeID] = a
				order = append(order, nl.NodeID)
			}
			if nl.Model != "" {
				a.sum.Model = nl.Model
			}
			a.sum.DurationMs += nl.Duration
			a.sum.Tokens += nl.InputTokens + nl.OutputTokens
			if nl.Success {
				a.sawSuccess = true
			} else {
				a.sawFail = true
			}
			a.sum.Success = nl.Success
			if nl.NodeType != "" {
				a.sum.NodeType = string(nl.NodeType)
			}
		}
		summaries := make([]nodeResultSummary, 0, len(order))
		for _, id := range order {
			a := byNode[id]
			// FallbackGroup 恢复：主节点曾失败又被补成功日志 → 对 UI 仍报失败（真实走了备用）
			if fallbackUsed && fallbackFromNode != "" && id == fallbackFromNode && a.sawFail {
				a.sum.Success = false
			}
			summaries = append(summaries, a.sum)
		}
		return summaries
	}

	// 回退：仅从 NodeOutputs 推断（无精确耗时）
	if len(output.NodeOutputs) > 0 {
		summaries := make([]nodeResultSummary, 0, len(output.NodeOutputs))
		for nodeID, no := range output.NodeOutputs {
			success := true
			if no.Passed != nil {
				success = *no.Passed
			}
			summaries = append(summaries, nodeResultSummary{
				NodeID:  nodeID,
				Success: success,
			})
		}
		return summaries
	}

	return nil
}

// shouldInjectCentagMetaSSE reports whether the client opted into Centag SSE metadata.
// Third-party Agents (OpenCode, Cursor, …) must not receive _centag_meta as data: payloads.
func shouldInjectCentagMetaSSE(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if v := strings.TrimSpace(c.GetHeader("X-Centag-Include-Meta")); v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	// 仅识别本产品 UA 前缀；避免 "llm-proxy" 子串误伤第三方严格客户端。
	ua := strings.ToLower(strings.TrimSpace(c.GetHeader("User-Agent")))
	return strings.HasPrefix(ua, "centag/") || strings.HasPrefix(ua, "opencode-centag/")
}

// buildStreamCentagMeta 从流水线输出构建 SSE 代理元数据事件体。
// 客户端通过识别 {"_centag_meta":true} 字段区分此事件与普通 OpenAI chunk。
func buildStreamCentagMeta(output *pipeline.PipelineOutput, pipelineID string, mode ProxyMode) map[string]interface{} {
	meta := map[string]interface{}{
		"_centag_meta": true,
		"mode":         string(mode),
		"pipeline_id":  pipelineID,
	}

	if output.ExecutionLog != nil {
		meta["duration_ms"] = output.ExecutionLog.Duration
		meta["total_tokens"] = output.ExecutionLog.TotalTokens
		meta["success"] = output.ExecutionLog.Success
	}

	// 注入每个流水线节点的执行摘要（TUI 可展示各阶段进度）
	if len(output.NodeOutputs) > 0 || (output.ExecutionLog != nil && len(output.ExecutionLog.NodeLogs) > 0) {
		meta["node_results"] = buildNodeResultsSummary(output)
	}
	// 最后执行的节点 ID
	if output.LastNode != "" {
		meta["last_node"] = output.LastNode
	}

	// executor_model / executor_backend 优先从 Metadata 读取，回退到 NodeLogs 中最后节点的模型信息
	hasExecutorModel := false
	if output.Metadata != nil {
		for key, headerKey := range map[string]string{
			"executor_backend":         "executor_backend",
			"executor_model":           "executor_model",
			"auditor_backend":          "auditor_backend",
			"auditor_model":            "auditor_model",
			"optimizer_backend":        "optimizer_backend",
			"optimizer_model":          "optimizer_model",
			"cache_hit":                "cache_hit",
			"selected_route":           "selected_route",
			"routing_strategy":         "routing_strategy",
			"bypass":                   "bypass",
			"bypass_node":              "bypass_node",
			"bypass_reason":            "bypass_reason",
			"fallback_used":            "fallback_used",
			"fallback_from_model":      "fallback_from_model",
			"fallback_to_model":        "fallback_to_model",
			"fallback_notice":          "fallback_notice",
			"fallback_reason":          "fallback_reason",
			"target_base_url":          "target_base_url",
			"backend_id":               "backend_id",
			"billing_fallback_used":    "billing_fallback_used",
			"billing_fallback_backend": "billing_fallback_backend",
			"billing_fallback_model":   "billing_fallback_model",
		} {
			if v, ok := output.Metadata[key]; ok {
				meta[headerKey] = v
				if key == "executor_model" {
					hasExecutorModel = true
				}
			}
		}
	}

	// 回退：如果 Metadata 中没有 executor_model，从 ExecutionLog.NodeLogs 中最后一个 LLM 节点的模型信息提取
	if !hasExecutorModel && output.ExecutionLog != nil && len(output.ExecutionLog.NodeLogs) > 0 {
		for i := len(output.ExecutionLog.NodeLogs) - 1; i >= 0; i-- {
			nl := output.ExecutionLog.NodeLogs[i]
			if nl.Model != "" {
				meta["executor_model"] = nl.Model
				hasExecutorModel = true
				break
			}
		}
	}

	// 审核结果
	if output.Passed != nil {
		meta["audit_passed"] = *output.Passed
	}
	if output.Score != nil {
		meta["audit_score"] = *output.Score
	}
	if output.Feedback != "" {
		meta["audit_feedback"] = output.Feedback
	}

	return meta
}

// StreamFormatter 流式响应格式化器接口
type StreamFormatter interface {
	// FormatChunk 格式化一个流式 chunk 为 SSE 数据行（含 "data: " 前缀和尾部 \n\n）
	FormatChunk(model string, chunk *plugin.StreamChunk, chunkIndex int, responseID string, created int64) string
	// FormatDone 格式化 usage chunk + 结束标记
	// finishReason 为实际完成原因（如 tool_calls/stop/length），空字符串时回退到 stop
	FormatDone(model string, usage map[string]interface{}, finishReason string) string
	// FormatError 在流式过程中发生错误时输出协议相容的错误事件，避免客户端 schema 校验失败把上游错误吞掉
	FormatError(model string, err error, responseID string, created int64) string
	// BuildUsage 从 finalOutput 构建 usage map
	BuildUsage(finalOutput *pipeline.PipelineOutput) map[string]interface{}
}

// openaiStreamFormatter OpenAI SSE 格式化器
type openaiStreamFormatter struct{}

func (f *openaiStreamFormatter) FormatChunk(model string, chunk *plugin.StreamChunk, chunkIndex int, responseID string, created int64) string {
	delta := map[string]interface{}{
		"content": chunk.Content,
	}
	if chunkIndex == 0 {
		delta["role"] = "assistant"
	}
	// 透传 reasoning_content（DeepSeek thinking 模式要求多轮回传）
	if chunk.ReasoningContent != "" {
		delta["reasoning_content"] = chunk.ReasoningContent
	}
	// 透传 tool_calls（function calling）
	if len(chunk.ToolCalls) > 0 {
		toolCalls := make([]interface{}, len(chunk.ToolCalls))
		for i, tc := range chunk.ToolCalls {
			toolCalls[i] = map[string]interface{}{
				"index": i,
				"id":    tc.ID,
				"type":  "function",
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		delta["tool_calls"] = toolCalls
	}

	choice := map[string]interface{}{
		"index": 0,
		"delta": delta,
	}
	// 最后一块携带 finish_reason
	if chunk.Done && chunk.FinishReason != "" {
		choice["finish_reason"] = chunk.FinishReason
	}

	chunkData := map[string]interface{}{
		"id":      responseID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []interface{}{choice},
	}

	dataBytes, _ := json.Marshal(chunkData)
	return fmt.Sprintf("data: %s\n\n", string(dataBytes))
}

func (f *openaiStreamFormatter) FormatDone(model string, usage map[string]interface{}, finishReason string) string {
	// finishReason 空时回退到 stop，保持向后兼容
	fr := finishReason
	if fr == "" {
		fr = "stop"
	}
	finalChunk := map[string]interface{}{
		"id":      "chatcmpl-done",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": fr,
			},
		},
	}
	if usage != nil {
		finalChunk["usage"] = usage
	}
	dataBytes, _ := json.Marshal(finalChunk)
	return fmt.Sprintf("data: %s\n\n", string(dataBytes))
}

func (f *openaiStreamFormatter) BuildUsage(finalOutput *pipeline.PipelineOutput) map[string]interface{} {
	usage := map[string]interface{}{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
	if finalOutput != nil && finalOutput.ExecutionLog != nil {
		usage["prompt_tokens"] = finalOutput.ExecutionLog.TotalTokens / 2
		usage["completion_tokens"] = finalOutput.ExecutionLog.TotalTokens / 2
		usage["total_tokens"] = finalOutput.ExecutionLog.TotalTokens
	}
	return usage
}

// FormatError 输出 OpenAI Chat Completions 风格的 SSE 错误块，并以 [DONE] 终止。
// OpenAI 流式协议未定义错误事件类型，客户端按 `data: {error:...}` 兼容。
func (f *openaiStreamFormatter) FormatError(model string, err error, responseID string, created int64) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if msg == "" {
		msg = "internal error"
	}
	payload := map[string]interface{}{
		"error": map[string]interface{}{
			"message": msg,
			"type":    "server_error",
			"code":    nil,
		},
	}
	if model != "" {
		payload["model"] = model
	}
	dataBytes, _ := json.Marshal(payload)
	return fmt.Sprintf("data: %s\n\ndata: [DONE]\n\n", string(dataBytes))
}

// anthropicStreamFormatter Anthropic SSE 格式化器
type anthropicStreamFormatter struct{}

func (f *anthropicStreamFormatter) FormatChunk(model string, chunk *plugin.StreamChunk, chunkIndex int, responseID string, created int64) string {
	if chunk == nil {
		return ""
	}
	// 无内容、无 tool_calls、且非 done 标记的空 chunk 跳过
	if chunk.Done && chunk.Content == "" && len(chunk.ToolCalls) == 0 {
		return ""
	}

	var sb strings.Builder

	// 第一个 chunk: 发送 message_start
	if chunkIndex == 0 {
		msgStart := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":          "msg-" + responseID,
				"type":        "message",
				"role":        "assistant",
				"content":     []interface{}{},
				"model":       model,
				"stop_reason": nil,
				"usage": map[string]interface{}{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		}
		dataBytes, _ := json.Marshal(msgStart)
		sb.WriteString(fmt.Sprintf("event: message_start\ndata: %s\n\n", string(dataBytes)))
	}

	// 文本内容：用 text content block（index=0）
	// tool_calls：每个工具调用用独立的 tool_use content block（index 从 1 开始递增）
	// Anthropic 规范：text block 在前，tool_use block 在后

	// 文本内容 delta
	if chunk.Content != "" {
		// 首次出现文本时先发 content_block_start(type=text)
		if chunkIndex == 0 {
			cbStart := map[string]interface{}{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]interface{}{
					"type": "text",
					"text": "",
				},
			}
			dataBytes, _ := json.Marshal(cbStart)
			sb.WriteString(fmt.Sprintf("event: content_block_start\ndata: %s\n\n", string(dataBytes)))
		}
		delta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": chunk.Content,
			},
		}
		dataBytes, _ := json.Marshal(delta)
		sb.WriteString(fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", string(dataBytes)))
	}

	// tool_calls：每个工具调用输出 content_block_start(tool_use) + content_block_delta(input_json_delta) + content_block_stop
	// index 规则：若有文本则从 1 开始，否则从 0 开始
	toolBlockStartIndex := 0
	if chunk.Content != "" {
		toolBlockStartIndex = 1
	}
	for i, tc := range chunk.ToolCalls {
		blockIndex := toolBlockStartIndex + i
		// content_block_start: tool_use
		cbStart := map[string]interface{}{
			"type":  "content_block_start",
			"index": blockIndex,
			"content_block": map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": map[string]interface{}{},
			},
		}
		dataBytes, _ := json.Marshal(cbStart)
		sb.WriteString(fmt.Sprintf("event: content_block_start\ndata: %s\n\n", string(dataBytes)))

		// content_block_delta: input_json_delta（arguments 作为 partial JSON）
		delta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": blockIndex,
			"delta": map[string]interface{}{
				"type":         "input_json_delta",
				"partial_json": tc.Function.Arguments,
			},
		}
		dataBytes, _ = json.Marshal(delta)
		sb.WriteString(fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", string(dataBytes)))

		// content_block_stop
		cbStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": blockIndex,
		}
		dataBytes, _ = json.Marshal(cbStop)
		sb.WriteString(fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", string(dataBytes)))
	}

	// done: 关闭文本 block（若有）+ message_delta + message_stop
	if chunk.Done {
		// 若有文本内容，关闭 text content block（index=0）
		if chunk.Content != "" {
			cbStop := map[string]interface{}{
				"type":  "content_block_stop",
				"index": 0,
			}
			dataBytes, _ := json.Marshal(cbStop)
			sb.WriteString(fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", string(dataBytes)))
		}

		stopReason := "end_turn"
		if chunk.FinishReason == "tool_calls" {
			stopReason = "tool_use"
		} else if chunk.FinishReason == "length" {
			stopReason = "max_tokens"
		}
		msgDelta := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": chunk.TokensUsed,
			},
		}
		dataBytes, _ := json.Marshal(msgDelta)
		sb.WriteString(fmt.Sprintf("event: message_delta\ndata: %s\n\n", string(dataBytes)))

		msgStop := map[string]interface{}{
			"type": "message_stop",
		}
		dataBytes, _ = json.Marshal(msgStop)
		sb.WriteString(fmt.Sprintf("event: message_stop\ndata: %s\n\n", string(dataBytes)))
	}

	return sb.String()
}

func (f *anthropicStreamFormatter) FormatDone(model string, usage map[string]interface{}, finishReason string) string {
	// Anthropic 不使用 [DONE] 标记，结束由 message_stop 事件处理
	return ""
}

func (f *anthropicStreamFormatter) BuildUsage(finalOutput *pipeline.PipelineOutput) map[string]interface{} {
	usage := map[string]interface{}{
		"output_tokens": 0,
	}
	if finalOutput != nil && finalOutput.ExecutionLog != nil {
		usage["output_tokens"] = finalOutput.ExecutionLog.TotalTokens / 2
	}
	return usage
}

// FormatError 输出 Anthropic 风格的 `event: error` 块。
// Anthropic 流式协议要求 error 事件携带 {type:"error", error:{type,message}}。
func (f *anthropicStreamFormatter) FormatError(model string, err error, responseID string, created int64) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if msg == "" {
		msg = "internal error"
	}
	payload := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": msg,
		},
	}
	dataBytes, _ := json.Marshal(payload)
	return fmt.Sprintf("event: error\ndata: %s\n\n", string(dataBytes))
}

// geminiStreamFormatter Gemini SSE 格式化器
type geminiStreamFormatter struct{}

func (f *geminiStreamFormatter) FormatChunk(model string, chunk *plugin.StreamChunk, chunkIndex int, responseID string, created int64) string {
	if chunk == nil {
		return ""
	}
	if chunk.Done && chunk.Content == "" && chunk.ReasoningContent == "" {
		return ""
	}
	candidate := map[string]interface{}{
		"content": map[string]interface{}{
			"role": "model",
			"parts": []map[string]interface{}{
				{"text": chunk.Content},
			},
		},
	}
	if chunk.FinishReason != "" {
		candidate["finishReason"] = chunk.FinishReason
	}
	chunkData := map[string]interface{}{
		"candidates": []interface{}{candidate},
	}
	dataBytes, _ := json.Marshal(chunkData)
	return fmt.Sprintf("data: %s\n\n", string(dataBytes))
}

func (f *geminiStreamFormatter) FormatDone(model string, usage map[string]interface{}, finishReason string) string {
	return ""
}

// FormatError 输出 Gemini 风格的流式错误块。
// Gemini 流式 SSE 用 `data: {"error":{code,message,status}}` 表示失败。
func (f *geminiStreamFormatter) FormatError(model string, err error, responseID string, created int64) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if msg == "" {
		msg = "internal error"
	}
	payload := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    500,
			"message": msg,
			"status":  "INTERNAL",
		},
	}
	dataBytes, _ := json.Marshal(payload)
	return fmt.Sprintf("data: %s\n\n", string(dataBytes))
}

func (f *geminiStreamFormatter) BuildUsage(finalOutput *pipeline.PipelineOutput) map[string]interface{} {
	if finalOutput != nil && finalOutput.ExecutionLog != nil {
		return map[string]interface{}{
			"promptTokenCount":     finalOutput.ExecutionLog.TotalTokens / 2,
			"candidatesTokenCount": finalOutput.ExecutionLog.TotalTokens / 2,
			"totalTokenCount":      finalOutput.ExecutionLog.TotalTokens,
		}
	}
	return nil
}

// responsesStreamFormatter OpenAI Responses SSE 格式化器.
//
// OpenCode / Codex 等客户端要求完整生命周期（不能只发 created+delta+completed）：
//
// Text:
//
//	response.created
//	response.output_item.added (message)
//	response.content_part.added
//	response.output_text.delta (+...)
//	response.output_text.done / content_part.done / output_item.done
//
// Tools:
//
//	response.output_item.added (function_call)
//	response.function_call_arguments.delta / .done
//	response.output_item.done
//	response.completed
//
// 缺少 output_item/content_part 时，文本 delta 会被静默丢弃；缺少 function_call
// 事件时 Agent 无法进入工具轮次。
type responsesStreamFormatter struct {
	seq           int
	opened        bool
	messageOpened bool
	messageOutIdx int
	responseID    string
	itemID        string
	model         string
	created       int64
	text          strings.Builder
	nextOutputIdx int
	functionItems []map[string]interface{}
}

func (f *responsesStreamFormatter) nextSeq() int {
	f.seq++
	return f.seq
}

func (f *responsesStreamFormatter) writeEvent(sb *strings.Builder, typ string, payload map[string]interface{}) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["type"] = typ
	payload["sequence_number"] = f.nextSeq()
	dataBytes, _ := json.Marshal(payload)
	sb.WriteString(fmt.Sprintf("event: %s\ndata: %s\n\n", typ, string(dataBytes)))
}

func (f *responsesStreamFormatter) ensureResponse(sb *strings.Builder, model, responseID string, created int64) {
	if f.opened {
		return
	}
	f.opened = true
	f.responseID = responseID
	f.model = model
	f.created = created
	f.writeEvent(sb, "response.created", map[string]interface{}{
		"response": map[string]interface{}{
			"id":         responseID,
			"object":     "response",
			"created_at": created,
			"status":     "in_progress",
			"model":      model,
			"output":     []interface{}{},
			"usage":      buildResponsesUsage(0, 0, 0),
		},
	})
}

func (f *responsesStreamFormatter) ensureMessage(sb *strings.Builder) {
	if f.messageOpened {
		return
	}
	f.messageOpened = true
	f.itemID = "msg_" + f.responseID
	f.messageOutIdx = f.nextOutputIdx
	f.nextOutputIdx++

	f.writeEvent(sb, "response.output_item.added", map[string]interface{}{
		"output_index": f.messageOutIdx,
		"item": map[string]interface{}{
			"id":      f.itemID,
			"type":    "message",
			"role":    "assistant",
			"status":  "in_progress",
			"content": []interface{}{},
		},
	})
	f.writeEvent(sb, "response.content_part.added", map[string]interface{}{
		"item_id":       f.itemID,
		"output_index":  f.messageOutIdx,
		"content_index": 0,
		"part": map[string]interface{}{
			"type":        "output_text",
			"text":        "",
			"annotations": []string{},
		},
	})
}

func (f *responsesStreamFormatter) FormatChunk(model string, chunk *plugin.StreamChunk, chunkIndex int, responseID string, created int64) string {
	if chunk == nil {
		return ""
	}
	var sb strings.Builder
	f.ensureResponse(&sb, model, responseID, created)

	deltaText := chunk.Content
	if deltaText == "" {
		deltaText = chunk.ReasoningContent
	}
	if deltaText != "" {
		f.ensureMessage(&sb)
		f.text.WriteString(deltaText)
		f.writeEvent(&sb, "response.output_text.delta", map[string]interface{}{
			"item_id":       f.itemID,
			"output_index":  f.messageOutIdx,
			"content_index": 0,
			"delta":         deltaText,
		})
	}

	for _, tc := range chunk.ToolCalls {
		f.writeFunctionCallEvents(&sb, tc)
	}
	return sb.String()
}

func (f *responsesStreamFormatter) writeFunctionCallEvents(sb *strings.Builder, tc plugin.ToolCall) {
	callID := strings.TrimSpace(tc.ID)
	if callID == "" {
		callID = fmt.Sprintf("call_%d", f.nextOutputIdx)
	}
	itemID := "fc_" + callID
	name := strings.TrimSpace(tc.Function.Name)
	if name == "" {
		return
	}
	args := tc.Function.Arguments
	if args == "" {
		args = "{}"
	}
	outIdx := f.nextOutputIdx
	f.nextOutputIdx++

	item := map[string]interface{}{
		"id":        itemID,
		"type":      "function_call",
		"status":    "in_progress",
		"call_id":   callID,
		"name":      name,
		"arguments": "",
	}
	f.writeEvent(sb, "response.output_item.added", map[string]interface{}{
		"output_index": outIdx,
		"item":         item,
	})
	f.writeEvent(sb, "response.function_call_arguments.delta", map[string]interface{}{
		"item_id":      itemID,
		"output_index": outIdx,
		"delta":        args,
	})
	f.writeEvent(sb, "response.function_call_arguments.done", map[string]interface{}{
		"item_id":      itemID,
		"output_index": outIdx,
		"name":         name,
		"arguments":    args,
	})
	doneItem := map[string]interface{}{
		"id":        itemID,
		"type":      "function_call",
		"status":    "completed",
		"call_id":   callID,
		"name":      name,
		"arguments": args,
	}
	f.writeEvent(sb, "response.output_item.done", map[string]interface{}{
		"output_index": outIdx,
		"item":         doneItem,
	})
	f.functionItems = append(f.functionItems, doneItem)
}

func (f *responsesStreamFormatter) FormatDone(model string, usage map[string]interface{}, finishReason string) string {
	var sb strings.Builder
	if !f.opened {
		// 无 chunk 时也要给出完整信封，避免客户端挂起
		rid := fmt.Sprintf("resp-%d", time.Now().UnixNano())
		f.ensureResponse(&sb, model, rid, time.Now().Unix())
	}
	if model != "" {
		f.model = model
	}

	output := make([]interface{}, 0, 1+len(f.functionItems))
	fullText := f.text.String()

	// 纯工具调用：不要伪造空 message。仅文本/空响应：保持 message 生命周期。
	if f.messageOpened || (len(f.functionItems) == 0) {
		if !f.messageOpened {
			f.ensureMessage(&sb)
		}
		f.writeEvent(&sb, "response.output_text.done", map[string]interface{}{
			"item_id":       f.itemID,
			"output_index":  f.messageOutIdx,
			"content_index": 0,
			"text":          fullText,
		})
		f.writeEvent(&sb, "response.content_part.done", map[string]interface{}{
			"item_id":       f.itemID,
			"output_index":  f.messageOutIdx,
			"content_index": 0,
			"part": map[string]interface{}{
				"type":        "output_text",
				"text":        fullText,
				"annotations": []string{},
			},
		})
		item := map[string]interface{}{
			"id":     f.itemID,
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []interface{}{
				map[string]interface{}{
					"type":        "output_text",
					"text":        fullText,
					"annotations": []string{},
				},
			},
		}
		f.writeEvent(&sb, "response.output_item.done", map[string]interface{}{
			"output_index": f.messageOutIdx,
			"item":         item,
		})
		output = append(output, item)
	}
	for _, fi := range f.functionItems {
		output = append(output, fi)
	}

	resp := map[string]interface{}{
		"id":         f.responseID,
		"object":     "response",
		"created_at": f.created,
		"status":     "completed",
		"model":      f.model,
		"output":     output,
	}
	if usage != nil {
		resp["usage"] = usage
	}
	if finishReason != "" {
		resp["incomplete_details"] = nil
	}
	f.writeEvent(&sb, "response.completed", map[string]interface{}{
		"response": resp,
	})
	return sb.String()
}

func (f *responsesStreamFormatter) BuildUsage(finalOutput *pipeline.PipelineOutput) map[string]interface{} {
	inputTokens := 0
	outputTokens := 0
	totalTokens := 0
	if finalOutput != nil && finalOutput.ExecutionLog != nil {
		inputTokens = finalOutput.ExecutionLog.TotalTokens / 2
		outputTokens = finalOutput.ExecutionLog.TotalTokens / 2
		totalTokens = finalOutput.ExecutionLog.TotalTokens
	}
	return buildResponsesUsage(inputTokens, outputTokens, totalTokens)
}

// FormatError 输出 Responses 流式协议的 `response.failed` 事件。
// OpenAI Responses SSE 严格要求事件类型命中已知 union；裸 `{error:...}` 数据行
// 会触发 `type: expected "response.output_text.delta"` 等校验失败并把上游错误吞掉。
// 因此必须先发 response.created（必要时），再发 response.failed。
func (f *responsesStreamFormatter) FormatError(model string, err error, responseID string, created int64) string {
	if err == nil {
		return ""
	}
	if !f.opened {
		if responseID == "" {
			responseID = fmt.Sprintf("resp-%d", time.Now().UnixNano())
		}
		if created == 0 {
			created = time.Now().Unix()
		}
	}
	var sb strings.Builder
	f.ensureResponse(&sb, model, responseID, created)

	msg := err.Error()
	if msg == "" {
		msg = "internal error"
	}
	resp := map[string]interface{}{
		"id":         f.responseID,
		"object":     "response",
		"created_at": f.created,
		"status":     "failed",
		"model":      f.model,
		"error": map[string]interface{}{
			"code":    "server_error",
			"message": msg,
		},
		"output": []interface{}{},
	}
	f.writeEvent(&sb, "response.failed", map[string]interface{}{
		"response": resp,
	})
	return sb.String()
}

// getStreamFormatter 根据请求上下文获取对应的流式格式化器
func (d *ModeDispatcher) getStreamFormatter(c *gin.Context) StreamFormatter {
	protocolName, _ := c.Get("protocol_plugin")
	if name, ok := protocolName.(string); ok {
		switch name {
		case "anthropic-protocol":
			if d.pluginManager != nil {
				if _, err := d.pluginManager.GetProtocol(name); err != nil {
					d.logger.Warn("anthropic protocol plugin not found, using anthropic stream formatter from context",
						"protocol", name,
						"error", err,
					)
				}
			}
			return &anthropicStreamFormatter{}
		case "gemini-protocol":
			return &geminiStreamFormatter{}
		case "responses-protocol":
			return &responsesStreamFormatter{}
		}
	}
	return &openaiStreamFormatter{}
}

func shouldEmitOpenAIDone(c *gin.Context) bool {
	if c == nil {
		return true
	}
	protocolName, _ := c.Get("protocol_plugin")
	if name, ok := protocolName.(string); ok {
		switch name {
		case "anthropic-protocol", "gemini-protocol", "responses-protocol":
			return false
		}
	}
	return true
}

func extractBackendFromPipelineOutput(output *pipeline.PipelineOutput) string {
	if output == nil || output.Metadata == nil {
		return ""
	}
	for _, key := range []string{"backend_id", "executor_backend", "backend"} {
		if v, ok := output.Metadata[key].(string); ok && v != "" {
			if !strings.Contains(v, "{{") {
				return v
			}
		}
	}
	if output.NodeOutputs != nil {
		for _, nodeOutput := range output.NodeOutputs {
			if nodeOutput == nil || nodeOutput.Metadata == nil {
				continue
			}
			if v, ok := nodeOutput.Metadata["backend_id"].(string); ok && v != "" {
				if !strings.Contains(v, "{{") {
					return v
				}
			}
		}
	}
	return ""
}

// effectiveResponseModel prefers the model actually used by the pipeline over the
// Agent-declared name (e.g. OpenCode hy3-free vs Centag default glm-4-flash / mimo).
func effectiveResponseModel(c *gin.Context, clientModel string, output *pipeline.PipelineOutput) string {
	if m := extractModelFromPipelineOutput(output); m != "" {
		return m
	}
	pipelineID := ""
	if c != nil {
		pipelineID = c.GetHeader("X-Pipeline-ID")
	}
	switch pipelineID {
	case "direct-backend", "transparent-proxy", "transparent-fast", "fixed-egress":
		if cfg := config.Get(); cfg != nil {
			if m := strings.TrimSpace(cfg.Proxy.DefaultModel); m != "" {
				return m
			}
		}
	}
	return strings.TrimSpace(clientModel)
}

func extractSelectedRouteFromPipelineOutput(output *pipeline.PipelineOutput) string {
	if output == nil {
		return ""
	}
	if output.Metadata != nil {
		if v, ok := output.Metadata["selected_route"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if output.NodeOutputs != nil {
		for _, nodeOutput := range output.NodeOutputs {
			if nodeOutput == nil || nodeOutput.Metadata == nil {
				continue
			}
			if v, ok := nodeOutput.Metadata["selected_route"].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func maybeRecordRouteBackendMetrics(output *pipeline.PipelineOutput) {
	if output == nil || metrics.GlobalRouteBackendMetrics == nil {
		return
	}
	backendID := extractBackendFromPipelineOutput(output)
	selectedRoute := extractSelectedRouteFromPipelineOutput(output)
	if backendID == "" && selectedRoute == "" {
		return
	}
	success := true
	latencyMs := int64(0)
	if output.ExecutionLog != nil {
		success = output.ExecutionLog.Success
		latencyMs = output.ExecutionLog.Duration
	}
	metrics.GlobalRouteBackendMetrics.Record(selectedRoute, backendID, success, latencyMs)
}

func extractModelFromPipelineOutput(output *pipeline.PipelineOutput) string {
	if output == nil {
		return ""
	}
	if output.ExecutionLog != nil {
		for i := len(output.ExecutionLog.NodeLogs) - 1; i >= 0; i-- {
			log := output.ExecutionLog.NodeLogs[i]
			m := strings.TrimSpace(log.Model)
			if m == "" || strings.Contains(m, "{{") {
				continue
			}
			if log.Success || i == len(output.ExecutionLog.NodeLogs)-1 {
				return m
			}
		}
	}
	if output.Metadata != nil {
		for _, key := range []string{"model", "response_model", "executor_model"} {
			if v, ok := output.Metadata[key].(string); ok {
				m := strings.TrimSpace(v)
				if m != "" && !strings.Contains(m, "{{") {
					return m
				}
			}
		}
	}
	return ""
}

func buildPipelineExecutionMeta(output *pipeline.PipelineOutput, fallbackPipelineID string) map[string]interface{} {
	if output == nil {
		return nil
	}
	model := extractModelFromPipelineOutput(output)
	backendID := extractBackendFromPipelineOutput(output)
	pipelineID := fallbackPipelineID
	if output.ExecutionLog != nil && strings.TrimSpace(output.ExecutionLog.PipelineID) != "" {
		pipelineID = strings.TrimSpace(output.ExecutionLog.PipelineID)
	}
	if model == "" && backendID == "" && pipelineID == "" {
		return nil
	}
	meta := map[string]interface{}{}
	if model != "" {
		meta["model"] = model
	}
	if backendID != "" {
		meta["backend_id"] = backendID
	}
	if pipelineID != "" {
		meta["pipeline_id"] = pipelineID
	}
	if output.ExecutionLog != nil && output.ExecutionLog.Duration > 0 {
		meta["pipeline_duration_ms"] = output.ExecutionLog.Duration
	}
	return meta
}

func setPipelineExecutionHeaders(c *gin.Context, output *pipeline.PipelineOutput, fallbackPipelineID string) {
	meta := buildPipelineExecutionMeta(output, fallbackPipelineID)
	if meta == nil {
		return
	}
	if v, ok := meta["model"].(string); ok && v != "" {
		c.Header("X-Model", v)
	}
	if v, ok := meta["backend_id"].(string); ok && v != "" {
		c.Header("X-Backend-ID", v)
	}
	if v, ok := meta["pipeline_id"].(string); ok && v != "" {
		c.Header("X-Pipeline-ID", v)
	}
	if output.ExecutionLog != nil && output.ExecutionLog.Duration > 0 {
		c.Header("X-Pipeline-Duration-Ms", fmt.Sprintf("%d", output.ExecutionLog.Duration))
	}
}

// setPipelineOutputHeaders 根据流水线输出动态设置响应头（不绑定特定内置流水线）。
func setPipelineOutputHeaders(c *gin.Context, output *pipeline.PipelineOutput) {
	if output == nil {
		return
	}
	if output.Passed != nil {
		c.Header("X-Audit-Passed", fmt.Sprintf("%v", *output.Passed))
	}
	if output.Score != nil {
		c.Header("X-Audit-Score", fmt.Sprintf("%.2f", *output.Score))
	}
	if output.Feedback != "" {
		c.Header("X-Audit-Feedback", output.Feedback)
	}
	if output.Metadata == nil {
		return
	}
	for metaKey, headerKey := range map[string]string{
		"executor_backend":      "X-Executor-Backend",
		"executor_model":        "X-Executor-Model",
		"auditor_backend":       "X-Auditor-Backend",
		"auditor_model":         "X-Auditor-Model",
		"optimizer_backend":     "X-Optimizer-Backend",
		"optimizer_model":       "X-Optimizer-Model",
		"cache_hit":             "X-Cache-Hit",
		"cache_saved":           "X-Cache-Saved",
		"target_base_url":       "X-Target-BaseURL",
		"bypass":                "X-Pipeline-Bypass",
		"bypass_node":           "X-Pipeline-Bypass-Node",
		"bypass_reason":         "X-Pipeline-Bypass-Reason",
		"fallback_used":         "X-Fallback-Used",
		"fallback_from_model":   "X-Fallback-From-Model",
		"fallback_to_model":     "X-Fallback-To-Model",
		"fallback_notice":       "X-Fallback-Notice",
		"fallback_reason":       "X-Fallback-Reason",
		"response_trace_banner": "X-Response-Trace",
		"pipeline_id":           "X-Pipeline-ID",
		"backend_id":            "X-Backend-ID",
	} {
		if v, ok := output.Metadata[metaKey]; ok && fmt.Sprintf("%v", v) != "" {
			c.Header(headerKey, fmt.Sprintf("%v", v))
		}
	}
}

// isEmptyPipelineOutput 判断流水线输出是否为「空」：无正文、无工具调用、
// 无推理内容、无 token、无消息。此类输出对客户端毫无价值，通常是上游失败
// 被降级/旁路逻辑聚合成空响应的结果，应作为错误返回而非空 200。
func isEmptyPipelineOutput(output *pipeline.PipelineOutput, totalTokens int) bool {
	if output == nil {
		return false
	}
	if strings.TrimSpace(output.Content) != "" ||
		len(output.ToolCalls) > 0 ||
		totalTokens > 0 ||
		strings.TrimSpace(output.ReasoningContent) != "" ||
		len(output.Messages) > 0 {
		return false
	}
	return true
}

// pipelineOutputTotalTokens 返回执行日志的总 token 数。
func pipelineOutputTotalTokens(output *pipeline.PipelineOutput) int {
	if output == nil || output.ExecutionLog == nil {
		return 0
	}
	return output.ExecutionLog.TotalTokens
}

// pipelineEmptyOutputHint 从执行日志中提取空响应的原因：
//   - 末节点成功但输出为空 → 上游/降级模型返回了空内容（限流/无输出），
//     此时不能只报主节点错误，否则客户端会误以为没有走降级。
//   - 否则取最后一个失败节点的错误信息。
func pipelineEmptyOutputHint(output *pipeline.PipelineOutput) string {
	if output == nil || output.ExecutionLog == nil {
		return ""
	}
	if strings.TrimSpace(output.ExecutionLog.ErrorMessage) != "" {
		return output.ExecutionLog.ErrorMessage
	}
	lastNodeID := strings.TrimSpace(output.LastNode)
	var lastModel string
	var lastFailed string
	var lastLog *pipeline.NodeExecutionLog
	for i := range output.ExecutionLog.NodeLogs {
		nl := &output.ExecutionLog.NodeLogs[i]
		if strings.TrimSpace(nl.Model) != "" {
			lastModel = nl.Model
		}
		if !nl.Success && strings.TrimSpace(nl.ErrorMessage) != "" {
			lastFailed = nl.ErrorMessage
		}
		if lastNodeID != "" && nl.NodeID == lastNodeID {
			lastLog = nl
		}
	}
	if lastLog != nil && lastLog.Success && lastModel != "" {
		primary := ""
		if lastFailed != "" {
			primary = "primary error: " + lastFailed
		}
		return fmt.Sprintf("model %q returned an empty response after fallback (%s)", lastModel, primary)
	}
	if lastFailed != "" {
		return lastFailed
	}
	return ""
}

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

// TransparentForwardNode 是直连/透明/跳板三条流水线共用的出站节点。
// 通过 custom_config 开关区分行为（route_policy / inject_system_prompt），而非多套实现。
type TransparentForwardNode struct {
	BaseNode
	DefaultScheme  string
	RedirectPolicy string // never, always, smart
	MaxRedirects   int
	// FixedEgress 固定出站：不做跨后端模型匹配（route_policy=fixed / fixed_egress=true）
	FixedEgress bool
	// InjectSystemPrompt 注入网关 system_prompt，替换客户端 system（直连 #d）
	InjectSystemPrompt bool
}

func NewTransparentForwardNode(config NodeConfig) (PipelineNode, error) {
	node := &TransparentForwardNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     120,
			retryConfig: DefaultRetryConfig(),
			permissions: []string{"network.outbound"},
		},
		DefaultScheme:  "https",
		RedirectPolicy: "never", // 默认不跟随重定向
		MaxRedirects:   5,
	}
	if config.CustomConfig != nil {
		if s, ok := config.CustomConfig["default_scheme"].(string); ok && strings.TrimSpace(s) != "" {
			node.DefaultScheme = strings.TrimSpace(s)
		}
		if s, ok := config.CustomConfig["redirect_policy"].(string); ok && strings.TrimSpace(s) != "" {
			node.RedirectPolicy = strings.TrimSpace(s)
		}
		// 处理 max_redirects，支持 float64 和 int 类型
		if v, ok := config.CustomConfig["max_redirects"].(float64); ok && v > 0 {
			node.MaxRedirects = int(v)
		} else if v, ok := config.CustomConfig["max_redirects"].(int); ok && v > 0 {
			node.MaxRedirects = v
		}
		if v, ok := config.CustomConfig["fixed_egress"].(bool); ok {
			node.FixedEgress = v
		}
		if s, ok := config.CustomConfig["route_policy"].(string); ok {
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "fixed":
				node.FixedEgress = true
			case "match_model", "model_match":
				node.FixedEgress = false
			}
		}
		if v, ok := config.CustomConfig["inject_system_prompt"].(bool); ok {
			node.InjectSystemPrompt = v
		}
	}
	return node, nil
}

func (n *TransparentForwardNode) Type() NodeType {
	return NodeTypeTransparentForward
}

func (n *TransparentForwardNode) Validate() error {
	return nil
}

func (n *TransparentForwardNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	meta := map[string]interface{}{}
	if input != nil && input.Metadata != nil {
		meta = input.Metadata
	}

	requestPath := stringMeta(meta, "request_path")
	method := strings.ToUpper(strings.TrimSpace(stringMeta(meta, "request_method")))
	if method == "" {
		method = http.MethodPost
	}

	body := []byte(strings.TrimSpace(stringMeta(meta, "raw_request_body")))
	// 真实代理场景：raw_request_body 由 attachTransparentRequestMetadata 填充（完整 JSON）
	// WebUI 测试场景：无 raw_request_body，用 input.Content 构造最小合法 JSON
	if len(body) == 0 && input != nil {
		model := strings.TrimSpace(n.config.Model)
		if model == "" && meta != nil {
			model = strings.TrimSpace(stringMeta(meta, "model"))
		}
		body = buildMinimalChatBody(strings.TrimSpace(input.Content), model)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("transparent_forward node %q: empty request body", n.id)
	}

	clientModel := extractJSONModel(body)
	backendID, resolvedModel, body := n.resolveTransparentRoute(meta, body, clientModel)

	targetURL, err := n.resolveTargetURL(meta, backendID, requestPath)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: %w", n.id, err)
	}

	// Responses clients (OpenCode / Codex) POST {input:...} to /v1/responses, but
	// configured backends are reached at /chat/completions which expects messages.
	// 必须先于 inject_system_prompt：否则 inject 会在 Responses 体上凭空写入 messages=[system]，
	// 导致 looksLikeResponsesBody 误判为已是 chat，responses_to_chat 桥接失败，
	// 上游 chat SSE（含 centag-fallback-notice）会原样打给 /v1/responses 客户端。
	responsesToChat := false
	if strings.Contains(targetURL, "/chat/completions") {
		if rewritten, ok := convertResponsesBodyToChatCompletions(body); ok {
			body = rewritten
			responsesToChat = true
		}
		// 客户端走 /v1/responses 时，即使 body 已是 chat 形态，响应也必须走 FormatChunk，
		// 不能 raw 透传 chat.completion.chunk。
		if !responsesToChat && isResponsesAPIPath(requestPath) {
			responsesToChat = true
		}
		// 无论是否走了 Responses 全文转换，都清洗 tools：
		// 已是 chat 形态但带 flat/hosted tools 时，智谱会报 tools[0].function 不能为空。
		if sanitized, ok := sanitizeChatCompletionsTools(body); ok {
			body = sanitized
		}
	}

	// 直连注入 gateway system：仅在 chat messages 形态上替换；须在 Responses→Chat 之后。
	if n.InjectSystemPrompt {
		systemPrompt := strings.TrimSpace(n.config.SystemPrompt)
		if systemPrompt == "" {
			if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil && execCtx.pipeline != nil {
				systemPrompt = strings.TrimSpace(execCtx.pipeline.GlobalConfig.SystemPrompt)
			}
		}
		if systemPrompt != "" {
			if rewritten, ok := injectSystemPromptIntoChatBody(body, systemPrompt); ok {
				body = rewritten
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: build request: %w", n.id, err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 已解析到配置后端时，优先用后端 API Key 鉴权上游。
	// 客户端 Authorization 是 Centag 网关鉴权（JWT / 网关 API Key），不能原样转发给上游，
	// 否则会出现直连正常、透明模式 AuthError: Invalid API key。
	// 无托管后端（高级旁路绝对 URL）时才透传 forward_authorization。
	if auth := resolveTransparentUpstreamAuth(backendID, meta); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	logger.Info("transparent_forward outbound",
		logger.GetField("node_id", n.id),
		logger.GetField("backend_id", backendID),
		logger.GetField("model", resolvedModel),
		logger.GetField("fixed_egress", n.FixedEgress),
		logger.GetField("target_url", targetURL),
	)

	client, err := n.getHTTPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: %w", n.id, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: upstream request failed: %w", n.id, err)
	}
	defer resp.Body.Close()

	// [+] 处理 301/302 重定向
	if (resp.StatusCode == 301 || resp.StatusCode == 302) && n.RedirectPolicy != "never" {
		location := resp.Header.Get("Location")
		if location != "" {
			// smart 模式：仅 GET/HEAD 跟随
			if n.RedirectPolicy == "smart" && method != http.MethodGet && method != http.MethodHead {
				// 不跟随，直接透传
			} else {
				// 跟随重定向
				resp.Body.Close()
				resp, err = n.followRedirect(ctx, req, location, client, method, body)
				if err != nil {
					return nil, fmt.Errorf("transparent_forward node %q: redirect failed: %w", n.id, err)
				}
				defer resp.Body.Close()
			}
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: read response: %w", n.id, err)
	}

	bodyStr := string(respBody)

	// 余额/额度不足：先尝试系统 fallback_model（同后端免费档或备用后端），再向上返回 error 供 FallbackGroups。
	// 纯鉴权 401/403（无计费关键词）仍透传给客户端。
	if config.IsBillingOrQuotaFailure(resp.StatusCode, bodyStr) {
		if out, ok := n.retryWithSystemBillingFallback(ctx, client, method, meta, body, backendID, resolvedModel, clientModel, requestPath, responsesToChat); ok {
			return out, nil
		}
		return nil, fmt.Errorf("transparent_forward node %q: backend=%s model=%s url=%s upstream returned %d: %s",
			n.id, backendID, resolvedModel, targetURL, resp.StatusCode, truncateBody(respBody, 512))
	}

	// 对可重试的 HTTP 错误码返回 error，触发上层重试/降级逻辑。
	if config.IsRetryableStatusCode(resp.StatusCode) {
		return nil, fmt.Errorf("transparent_forward node %q: backend=%s model=%s url=%s upstream returned %d: %s",
			n.id, backendID, resolvedModel, targetURL, resp.StatusCode, truncateBody(respBody, 512))
	}

	// 模型不存在 / 占位符未解析等：必须返回 error，避免策略降级把错误 JSON 当成成功。
	if resp.StatusCode >= 400 && isUpstreamModelOrPlaceholderError(bodyStr) {
		return nil, fmt.Errorf("transparent_forward node %q: backend=%s model=%s url=%s upstream returned %d: %s",
			n.id, backendID, resolvedModel, targetURL, resp.StatusCode, truncateBody(respBody, 512))
	}

	return n.buildTransparentOutput(targetURL, resp.StatusCode, resp.Header.Get("Content-Type"), respBody, backendID, resolvedModel, clientModel, requestPath, responsesToChat, nil), nil
}

func isUpstreamModelOrPlaceholderError(body string) bool {
	lower := strings.ToLower(body)
	if strings.Contains(lower, "modelerror") ||
		strings.Contains(lower, "model_not_found") ||
		strings.Contains(lower, "is not supported") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(body, "模型不存在") ||
		strings.Contains(body, "模型代码") ||
		strings.Contains(lower, `"code":"1211"`) ||
		strings.Contains(lower, `"code": 1211`) {
		return true
	}
	// 字面量占位符被当成模型名发给上游
	return strings.Contains(body, "{{requested_model}}") ||
		strings.Contains(body, "{{system.fallback_model}}") ||
		strings.Contains(body, "{{system.default_model}}")
}

// retryWithSystemBillingFallback 在余额/额度失败时按候选链再打：配置的 fallback_* → 同后端免费档模型。
func (n *TransparentForwardNode) retryWithSystemBillingFallback(
	ctx context.Context,
	client HTTPClient,
	method string,
	meta map[string]interface{},
	body []byte,
	primaryBackend, primaryModel, clientModel, requestPath string,
	responsesToChat bool,
) (*NodeOutput, bool) {
	// 只把「实际发出去的 model」标为失败。resolvedModel 可能已是免费档，但 body 仍是付费名。
	failedModels := map[string]bool{}
	bodyModel := extractJSONModel(body)
	for _, m := range []string{bodyModel, clientModel} {
		if t := strings.TrimSpace(m); t != "" {
			failedModels[strings.ToLower(t)] = true
		}
	}
	if t := strings.TrimSpace(primaryModel); t != "" && strings.EqualFold(t, bodyModel) {
		failedModels[strings.ToLower(t)] = true
	}
	cands := billingFallbackCandidates(primaryBackend, failedModels)
	if len(cands) == 0 {
		return nil, false
	}
	for _, cand := range cands {
		out, ok := n.doBillingFallbackAttempt(ctx, client, method, meta, body, primaryBackend, primaryModel, clientModel, requestPath, responsesToChat, cand.backendID, cand.model)
		if ok {
			return out, true
		}
	}
	return nil, false
}

type billingFallbackCandidate struct {
	backendID string
	model     string
}

func billingFallbackCandidates(primaryBackend string, failedModels map[string]bool) []billingFallbackCandidate {
	primaryBackend = strings.TrimSpace(primaryBackend)
	fbBackend, fbModel := ResolveVirtualVars("{{system.fallback_backend}}", "{{system.fallback_model}}")
	fbBackend = strings.TrimSpace(fbBackend)
	fbModel = strings.TrimSpace(fbModel)
	if fbBackend == "" {
		fbBackend = primaryBackend
	}

	seen := map[string]bool{}
	var out []billingFallbackCandidate
	add := func(be, model string) {
		be = strings.TrimSpace(be)
		model = strings.TrimSpace(model)
		if be == "" || model == "" || strings.Contains(be, "{{") || strings.Contains(model, "{{") {
			return
		}
		if failedModels[strings.ToLower(model)] {
			return
		}
		key := strings.ToLower(be) + "\x00" + strings.ToLower(model)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, billingFallbackCandidate{backendID: be, model: model})
	}

	add(fbBackend, fbModel)
	add(primaryBackend, pickFreeTierModel(primaryBackend))
	if fbBackend != "" && !strings.EqualFold(fbBackend, primaryBackend) && !strings.Contains(fbBackend, "{{") {
		add(fbBackend, pickFreeTierModel(fbBackend))
	}
	return out
}

func pickFreeTierModel(backendID string) string {
	backendID = strings.TrimSpace(backendID)
	if backendID == "" {
		return ""
	}
	mgr := backend.GetManager()
	if mgr == nil {
		return ""
	}
	b, err := mgr.Get(backendID)
	if err != nil || b == nil {
		return ""
	}
	for _, m := range b.SupportedModels {
		for _, name := range []string{m.ActualModel, m.RequestedModel} {
			if backend.ModelHasFreeTier(name) {
				if actual := strings.TrimSpace(m.ActualModel); actual != "" {
					return actual
				}
				return strings.TrimSpace(m.RequestedModel)
			}
		}
	}
	return ""
}

func (n *TransparentForwardNode) doBillingFallbackAttempt(
	ctx context.Context,
	client HTTPClient,
	method string,
	meta map[string]interface{},
	body []byte,
	primaryBackend, primaryModel, clientModel, requestPath string,
	responsesToChat bool,
	fbBackend, fbModel string,
) (*NodeOutput, bool) {
	retryBody := body
	if rewritten, ok := rewriteTransparentBodyModel(body, fbModel); ok {
		retryBody = rewritten
	} else if !strings.EqualFold(extractJSONModel(body), fbModel) {
		return nil, false
	}

	targetURL, err := n.resolveTargetURL(meta, fbBackend, requestPath)
	if err != nil || strings.TrimSpace(targetURL) == "" {
		return nil, false
	}
	attemptResponsesToChat := responsesToChat
	if strings.Contains(targetURL, "/chat/completions") {
		if rewritten, ok := convertResponsesBodyToChatCompletions(retryBody); ok {
			retryBody = rewritten
			attemptResponsesToChat = true
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(string(retryBody)))
	if err != nil {
		return nil, false
	}
	req.Header.Set("Content-Type", "application/json")
	if auth := resolveTransparentUpstreamAuth(fbBackend, meta); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	if config.IsBillingOrQuotaFailure(resp.StatusCode, string(respBody)) || config.IsRetryableStatusCode(resp.StatusCode) || resp.StatusCode >= 400 {
		return nil, false
	}

	fromModel := strings.TrimSpace(primaryModel)
	if bodyModel := extractJSONModel(body); bodyModel != "" {
		fromModel = bodyModel
	} else if strings.TrimSpace(clientModel) != "" {
		fromModel = strings.TrimSpace(clientModel)
	}
	extra := map[string]interface{}{
		"billing_fallback_used":         true,
		"billing_fallback_from_model":   fromModel,
		"billing_fallback_to_model":     fbModel,
		"billing_fallback_backend":      fbBackend,
		"billing_fallback_from_backend": primaryBackend,
		"fallback_from_model":           fromModel,
		"fallback_to_model":             fbModel,
		"fallback_used":                 true,
	}
	out := n.buildTransparentOutput(targetURL, resp.StatusCode, resp.Header.Get("Content-Type"), respBody, fbBackend, fbModel, clientModel, requestPath, attemptResponsesToChat, extra)
	AnnotateFallbackNotice(out)
	return out, true
}

func (n *TransparentForwardNode) buildTransparentOutput(
	targetURL string,
	statusCode int,
	contentType string,
	respBody []byte,
	backendID, resolvedModel, clientModel, requestPath string,
	responsesToChat bool,
	extraMeta map[string]interface{},
) *NodeOutput {
	if contentType == "" {
		contentType = "application/json"
	}

	baseURL := targetURL
	if idx := strings.Index(baseURL, "://"); idx >= 0 {
		rest := baseURL[idx+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			baseURL = baseURL[:idx+3+slash]
		}
	}

	outMeta := map[string]interface{}{
		"raw_passthrough": true,
		"target_url":      targetURL,
		"target_base_url": baseURL,
		"status_code":     statusCode,
		"content_type":    contentType,
		"forwarded":       true,
	}
	if rp := strings.TrimSpace(requestPath); rp != "" {
		outMeta["request_path"] = rp
	}
	if n.FixedEgress {
		outMeta["fixed_egress"] = true
		outMeta["route_policy"] = "fixed"
	} else {
		outMeta["route_policy"] = "match_model"
	}
	if n.InjectSystemPrompt {
		outMeta["inject_system_prompt"] = true
	}
	if resolvedModel != "" {
		outMeta["model"] = resolvedModel
		outMeta["executor_model"] = resolvedModel
	}
	if clientModel != "" {
		outMeta["requested_model"] = clientModel
	}
	if backendID != "" {
		outMeta["backend_id"] = backendID
	}
	for k, v := range extraMeta {
		outMeta[k] = v
	}

	content := string(respBody)
	var toolCalls []ToolCall
	finishReason := ""
	reasoning := ""
	if responsesToChat && statusCode < 400 {
		// 无论是否抽到正文，都必须关闭透传：否则 chat.completion SSE 会原样打给
		// /v1/responses 客户端，OpenCode 等不到 response.* 事件会一直转圈。
		extracted := extractChatCompletionResult(respBody)
		content = extracted.Text
		reasoning = extracted.Reasoning
		toolCalls = extracted.ToolCalls
		finishReason = extracted.FinishReason
		outMeta["raw_passthrough"] = false
		outMeta["responses_to_chat"] = true
		contentType = "text/plain"
		outMeta["content_type"] = contentType
		// 部分模型（如 deepseek）流式主要走 reasoning_content；正文为空时用推理文本兜底，避免空回复。
		if strings.TrimSpace(content) == "" && strings.TrimSpace(reasoning) != "" {
			content = reasoning
			reasoning = ""
		}
	}

	return &NodeOutput{
		Content:          content,
		Metadata:         outMeta,
		ToolCalls:        toolCalls,
		FinishReason:     finishReason,
		ReasoningContent: reasoning,
	}
}

// resolveTargetURL 解析上游 URL。
// 固定出站：优先托管后端；仅无后端时才允许显式 target_url 高级旁路（不用 original_host）。
func (n *TransparentForwardNode) resolveTargetURL(meta map[string]interface{}, backendID, requestPath string) (string, error) {
	if n.FixedEgress {
		bid := strings.TrimSpace(backendID)
		if bid == "" && meta != nil {
			bid = strings.TrimSpace(stringMeta(meta, "backend_id"))
		}
		if bid != "" && ResolveBackendEndpoint != nil {
			ep, err := ResolveBackendEndpoint(bid)
			if err != nil {
				return "", fmt.Errorf("resolve backend %q: %w", bid, err)
			}
			if ep != nil && strings.TrimSpace(ep.BaseURL) != "" {
				base := strings.TrimRight(strings.TrimSpace(ep.BaseURL), "/")
				return base + "/chat/completions", nil
			}
		}
		if meta != nil {
			if u := strings.TrimSpace(stringMeta(meta, "target_url")); u != "" {
				return normalizeTargetURL(u, requestPath, n.DefaultScheme)
			}
		}
		return "", fmt.Errorf("fixed egress: no default backend or target_url")
	}
	return ResolveTransparentTargetURL(meta, backendID, requestPath, n.DefaultScheme)
}

// resolveTransparentRoute picks backend/model:
// FixedEgress: 固定默认/钉死后端 + 默认模型，不做跨后端模型匹配
// 否则:
// 1) explicit X-Backend-ID / metadata backend_id → match client model within that backend
// 2) client model → loose/exact match across enabled backends
// 3) miss / unspecified → system default backend, else first usable enabled backend
// On hit: keep client model string when it already equals ActualModel; else rewrite to ActualModel.
func (n *TransparentForwardNode) resolveTransparentRoute(
	meta map[string]interface{},
	body []byte,
	clientModel string,
) (backendID, resolvedModel string, outBody []byte) {
	outBody = body

	pinnedBackend := strings.TrimSpace(stringMeta(meta, "backend_id"))

	if n.FixedEgress {
		// 直连固定出站：只用节点配置的后端与模型，忽略请求头或 Agent 注入的 X-Backend-ID。
		// 具体 ID（非 {{system.*}}）绝不回落到「第一个可用后端 / 系统默认」，避免钉死 zen 却打到 bigmodel。
		rawBackend := strings.TrimSpace(n.config.Backend)
		if rawBackend != "" && !strings.Contains(rawBackend, "{{") {
			backendID = rawBackend
		} else {
			backendID = resolveFallbackBackendID(rawBackend, "")
		}
		if backendID != "" {
			resolvedModel, outBody = applyFallbackModel(outBody, backendID, n.config.Model)
		}
		return backendID, resolvedModel, outBody
	}

	if !isUnspecifiedClientModel(clientModel) {
		if pinnedBackend != "" {
			if mapping := matchModelOnBackend(clientModel, pinnedBackend); mapping != nil {
				backendID = pinnedBackend
				resolvedModel, outBody = applyClientModelRewrite(outBody, clientModel, mapping)
				return backendID, resolvedModel, outBody
			}
			// pinned backend but model miss → keep backend, use default/preferred model
			backendID = pinnedBackend
			resolvedModel, outBody = applyFallbackModel(outBody, backendID, n.config.Model)
			return backendID, resolvedModel, outBody
		}

		if matchedBackend, mapping := matchClientModelAcrossBackends(clientModel); matchedBackend != nil && mapping != nil {
			backendID = matchedBackend.ID
			resolvedModel, outBody = applyClientModelRewrite(outBody, clientModel, mapping)
			return backendID, resolvedModel, outBody
		}
	}

	// Fallback: node/system default → first usable enabled backend
	backendID = resolveFallbackBackendID(n.config.Backend, pinnedBackend)
	if backendID != "" {
		resolvedModel, outBody = applyFallbackModel(outBody, backendID, n.config.Model)
	}
	return backendID, resolvedModel, outBody
}

// resolveFallbackBackendID resolves {{system.default_backend}} / empty to a concrete backend.
// When DefaultBackendID is unset, falls back to the first usable enabled backend.
func resolveFallbackBackendID(nodeBackend, pinnedBackend string) string {
	candidates := []string{
		strings.TrimSpace(pinnedBackend),
		strings.TrimSpace(nodeBackend),
	}
	for _, c := range candidates {
		if c == "" || strings.Contains(c, "{{system.") {
			resolved, _ := ResolveVirtualVars(c, "")
			c = strings.TrimSpace(resolved)
		}
		if c != "" && !strings.Contains(c, "{{system.") {
			return c
		}
	}
	if ListEnabledBackendsForMatch == nil {
		return ""
	}
	var firstEnabled string
	for _, cfg := range ListEnabledBackendsForMatch() {
		if cfg == nil || !cfg.Enabled || strings.TrimSpace(cfg.ID) == "" {
			continue
		}
		if firstEnabled == "" {
			firstEnabled = cfg.ID
		}
		if backend.IsUsableLLMBackend(cfg) {
			return cfg.ID
		}
	}
	return firstEnabled
}

func applyFallbackModel(body []byte, backendID, nodeModel string) (resolved string, out []byte) {
	out = body
	_, resolved = ResolveVirtualVars(backendID, nodeModel)
	if resolved == "" {
		_, resolved = ResolveVirtualVars("{{system.default_backend}}", "{{system.default_model}}")
	}
	if resolved == "" && ListEnabledBackendsForMatch != nil {
		for _, cfg := range ListEnabledBackendsForMatch() {
			if cfg != nil && cfg.ID == backendID {
				resolved = backend.PreferredDefaultModel(cfg)
				break
			}
		}
	}
	if resolved != "" {
		out = forceBodyModel(out, resolved)
	}
	return resolved, out
}

// forceBodyModel 确保 JSON body 的 model 字段被改写；rewrite 失败时强制写入。
func forceBodyModel(body []byte, model string) []byte {
	model = strings.TrimSpace(model)
	if model == "" || len(body) == 0 {
		return body
	}
	if rewritten, ok := rewriteTransparentBodyModel(body, model); ok {
		return rewritten
	}
	if strings.EqualFold(extractJSONModel(body), model) {
		return body
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body
	}
	raw["model"] = model
	encoded, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return encoded
}

func matchClientModelAcrossBackends(clientModel string) (*backend.BackendConfig, *backend.ModelMapping) {
	if ListEnabledBackendsForMatch == nil {
		return nil, nil
	}
	backends := ListEnabledBackendsForMatch()
	if len(backends) == 0 {
		return nil, nil
	}
	// Transparent routing: exact/loose name match only (mino2.5 ≈ mino2.5 free), no hybrid family conversion.
	matchCfg := backend.DefaultModelMatchingConfig()
	matchCfg.Strategy = backend.StrategyExact
	matchCfg.ConversionWeight = 0
	selector := backend.NewBackendSelector(matchCfg)
	selected, actualModel, err := selector.SelectBackendByModel(clientModel, backends)
	if err != nil || selected == nil {
		return nil, nil
	}
	mapping := backend.FindLooseModelMapping(clientModel, selected)
	if mapping == nil {
		am := strings.TrimSpace(actualModel)
		if am == "" {
			return nil, nil
		}
		mapping = &backend.ModelMapping{RequestedModel: clientModel, ActualModel: am}
	}
	return selected, mapping
}

func matchModelOnBackend(clientModel, backendID string) *backend.ModelMapping {
	if ListEnabledBackendsForMatch == nil || strings.TrimSpace(backendID) == "" {
		return nil
	}
	for _, cfg := range ListEnabledBackendsForMatch() {
		if cfg != nil && cfg.ID == backendID {
			return backend.FindLooseModelMapping(clientModel, cfg)
		}
	}
	return nil
}

// applyClientModelRewrite keeps the client model string when it already equals ActualModel
// or RequestedModel; otherwise rewrites to ActualModel. Never cross free/paid tier on rewrite
// when the client explicitly asked for one tier (e.g. keep deepseek-v4-flash-free).
func applyClientModelRewrite(body []byte, clientModel string, mapping *backend.ModelMapping) (resolved string, out []byte) {
	out = body
	client := strings.TrimSpace(clientModel)
	if mapping == nil {
		return client, out
	}
	actual := strings.TrimSpace(mapping.ActualModel)
	requested := strings.TrimSpace(mapping.RequestedModel)
	if actual == "" {
		actual = requested
	}
	if client == "" {
		return actual, out
	}
	// Exact usable name — do not rewrite
	if strings.EqualFold(client, actual) || strings.EqualFold(client, requested) {
		keep := client
		if strings.EqualFold(client, actual) {
			keep = actual
		} else if strings.EqualFold(client, requested) && requested != "" {
			// Prefer ActualModel only when same free-tier (alias), else keep client spelling
			if backend.ModelHasFreeTier(client) == backend.ModelHasFreeTier(actual) && actual != "" {
				keep = actual
				if keep != client {
					if rewritten, ok := rewriteTransparentBodyModel(out, keep); ok {
						out = rewritten
					}
				}
				return keep, out
			}
			return client, out
		}
		return keep, out
	}
	// Loose match rewrite:
	// - never rewrite free → paid (CreditsError on OpenCode Zen etc.)
	// - allow base → free alias (mino2.5 → mino2.5 free) when that is the mapped ActualModel
	if actual != "" && backend.ModelHasFreeTier(client) && !backend.ModelHasFreeTier(actual) {
		return client, out
	}
	if actual != "" && client != actual {
		if rewritten, ok := rewriteTransparentBodyModel(out, actual); ok {
			out = rewritten
		}
		return actual, out
	}
	return client, out
}

func extractJSONModel(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return ""
	}
	m, _ := raw["model"].(string)
	return strings.TrimSpace(m)
}

func isUnspecifiedClientModel(model string) bool {
	m := strings.TrimSpace(model)
	if m == "" {
		return true
	}
	lower := strings.ToLower(m)
	if lower == "auto" || lower == "default" {
		return true
	}
	// virtual pipeline models (centag/xxx、pipeline.xxx / pipeline.xxx.auto)
	if strings.HasPrefix(lower, "centag/") || strings.HasPrefix(lower, "pipeline.") || strings.HasPrefix(lower, "pipeline_") {
		return true
	}
	return false
}

// rewriteTransparentBodyModel sets JSON "model" to Centag's resolved model, keeping other fields.
func rewriteTransparentBodyModel(body []byte, model string) ([]byte, bool) {
	model = strings.TrimSpace(model)
	if model == "" || len(body) == 0 {
		return body, false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body, false
	}
	if cur, _ := raw["model"].(string); strings.TrimSpace(cur) == model {
		return body, false
	}
	raw["model"] = model
	out, err := json.Marshal(raw)
	if err != nil {
		return body, false
	}
	return out, true
}

func (n *TransparentForwardNode) getHTTPClient(ctx context.Context) (HTTPClient, error) {
	if n.capabilityBroker == nil {
		return nil, fmt.Errorf("capability broker not configured")
	}
	return n.capabilityBroker.GetHTTPClient(ctx, n.permissions)
}

// resolveTransparentUpstreamAuth 选择打向上游的 Authorization。
func resolveTransparentUpstreamAuth(backendID string, meta map[string]interface{}) string {
	if ResolveBackendEndpoint != nil && strings.TrimSpace(backendID) != "" {
		if ep, epErr := ResolveBackendEndpoint(backendID); epErr == nil && ep != nil {
			if key := strings.TrimSpace(ep.APIKey); key != "" {
				return "Bearer " + key
			}
		}
	}
	return strings.TrimSpace(stringMeta(meta, "forward_authorization"))
}

// isResponsesAPIPath 判断请求路径是否为 OpenAI Responses API（/v1/responses 等）。
func isResponsesAPIPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	path = strings.TrimRight(path, "/")
	return strings.HasSuffix(path, "/responses")
}

// injectSystemPromptIntoChatBody 用网关 system_prompt 替换客户端 messages 中的 system 角色。
// 不对 Responses 形态（input、无可用 messages）动手，避免污染后续转换判定。
func injectSystemPromptIntoChatBody(body []byte, systemPrompt string) ([]byte, bool) {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" || len(body) == 0 {
		return body, false
	}
	if looksLikeResponsesBody(body) {
		return body, false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return body, false
	}
	msgs, ok := raw["messages"].([]interface{})
	if !ok {
		return body, false
	}
	filtered := make([]interface{}, 0, len(msgs)+1)
	filtered = append(filtered, map[string]interface{}{
		"role":    "system",
		"content": systemPrompt,
	})
	for _, m := range msgs {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := mm["role"].(string); strings.EqualFold(strings.TrimSpace(role), "system") {
			continue
		}
		filtered = append(filtered, mm)
	}
	raw["messages"] = filtered
	out, err := json.Marshal(raw)
	if err != nil {
		return body, false
	}
	return out, true
}

// buildMinimalChatBody 在无 raw_request_body（WebUI 测试场景）时，
// 用 input.Content 构造最小合法 chat/completions JSON，保持与真实请求体一致的格式。
func buildMinimalChatBody(content, model string) []byte {
	if model == "" {
		model = "default"
	}
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model    string `json:"model"`
		Messages []msg  `json:"messages"`
	}{
		Model: model,
		Messages: []msg{
			{Role: "user", Content: content},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	return b
}

func truncateBody(b []byte, maxLen int) string {
	s := string(b)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// followRedirect 跟随重定向
func (n *TransparentForwardNode) followRedirect(
	ctx context.Context,
	origReq *http.Request,
	location string,
	client HTTPClient,
	method string,
	body []byte,
) (*http.Response, error) {
	var lastResp *http.Response
	redirectCount := 0

	for redirectCount < n.MaxRedirects {
		// 构建新的请求 URL
		newURL := location
		if !strings.HasPrefix(location, "http://") && !strings.HasPrefix(location, "https://") {
			// 相对路径
			baseURL := origReq.URL.String()
			if idx := strings.LastIndex(baseURL, "/"); idx >= 0 {
				newURL = baseURL[:idx+1] + location
			}
		}

		// 创建新的请求
		newReq, err := http.NewRequestWithContext(ctx, method, newURL, strings.NewReader(string(body)))
		if err != nil {
			return nil, fmt.Errorf("build redirect request: %w", err)
		}

		// 复制原始请求的头
		for key, values := range origReq.Header {
			for _, value := range values {
				newReq.Header.Add(key, value)
			}
		}

		// 发送请求
		resp, err := client.Do(newReq)
		if err != nil {
			return nil, fmt.Errorf("redirect request failed: %w", err)
		}

		// 保存上一个响应
		if lastResp != nil {
			lastResp.Body.Close()
		}
		lastResp = resp

		// 如果不是重定向，返回响应
		if resp.StatusCode != 301 && resp.StatusCode != 302 {
			return resp, nil
		}

		// 获取新的 Location
		location = resp.Header.Get("Location")
		if location == "" {
			return resp, nil
		}

		redirectCount++
	}

	// 达到最大重定向次数，返回最后一个响应
	return lastResp, nil
}

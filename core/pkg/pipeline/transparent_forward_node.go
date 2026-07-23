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
)

// TransparentForwardNode forwards the original HTTP request to an upstream API unchanged.
// Unlike GeneratorNode, it does NOT re-assemble the request body (no prompt_template/system_prompt).
// The client's raw JSON (including extended fields like thinking, tool_choice, etc.) is forwarded as-is
// to the backend selected by client model match or system defaults.
type TransparentForwardNode struct {
	BaseNode
	DefaultScheme  string
	RedirectPolicy string // never, always, smart
	MaxRedirects   int
	// FixedEgress 固定出站跳板：不做跨后端模型匹配，固定默认/钉死后端 + Key 改写
	FixedEgress bool
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
	// Rewrite before upstream call; otherwise providers (e.g. BigModel) return
	// "输入不能为空" while Centag logs still show a non-empty messages_preview.
	responsesToChat := false
	if strings.Contains(targetURL, "/chat/completions") {
		if rewritten, ok := convertResponsesBodyToChatCompletions(body); ok {
			body = rewritten
			responsesToChat = true
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

	// 对可重试的 HTTP 错误码返回 error，触发上层重试/降级逻辑。
	// 不可重试的错误码（如 401/403）仍透传给客户端。
	if config.IsRetryableStatusCode(resp.StatusCode) {
		return nil, fmt.Errorf("transparent_forward node %q: upstream returned %d: %s", n.id, resp.StatusCode, truncateBody(respBody, 512))
	}

	contentType := resp.Header.Get("Content-Type")
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
		"status_code":     resp.StatusCode,
		"content_type":    contentType,
		"forwarded":       true,
	}
	if n.FixedEgress {
		outMeta["fixed_egress"] = true
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

	content := string(respBody)
	var toolCalls []ToolCall
	finishReason := ""
	// After Responses→Chat rewrite, upstream returns chat.completion(.chunk) SSE/JSON.
	// OpenCode expects Responses SSE — flatten text/tool_calls and let the protocol
	// formatter rebuild the Responses envelope (disable raw passthrough).
	if responsesToChat && resp.StatusCode < 400 {
		extracted := extractChatCompletionResult(respBody)
		if extracted.Text != "" || len(extracted.ToolCalls) > 0 {
			content = extracted.Text
			toolCalls = extracted.ToolCalls
			finishReason = extracted.FinishReason
			outMeta["raw_passthrough"] = false
			outMeta["responses_to_chat"] = true
			contentType = "text/plain"
			outMeta["content_type"] = contentType
		}
	}

	return &NodeOutput{
		Content:      content,
		Metadata:     outMeta,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
	}, nil
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
		backendID = resolveFallbackBackendID(n.config.Backend, pinnedBackend)
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
		if c == "" || c == "{{system.default_backend}}" {
			resolved, _ := ResolveVirtualVars("{{system.default_backend}}", "")
			c = strings.TrimSpace(resolved)
		}
		if c != "" && c != "{{system.default_backend}}" {
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
		if rewritten, ok := rewriteTransparentBodyModel(out, resolved); ok {
			out = rewritten
		}
	}
	return resolved, out
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
	// virtual pipeline models (pipeline.xxx / pipeline.xxx.auto)
	if strings.HasPrefix(lower, "pipeline.") || strings.HasPrefix(lower, "pipeline_") {
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

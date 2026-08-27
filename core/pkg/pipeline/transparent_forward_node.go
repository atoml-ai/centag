package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline/promptstrategy"
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
	// SystemPromptStrategy system prompt 策略（passthrough/append/replace）
	SystemPromptStrategy promptstrategy.SystemMode
	// AppendPosition append 模式下的插入位置
	AppendPosition promptstrategy.AppendPosition
}

// accountSelectorStore 跨请求共享的账户池选择器。
// 引擎每次请求都会 CreateFromConfig 重建节点实例（executeNode），
// 因此选择器若挂在节点上会被反复重置，round_robin/least_usage/sticky_session
// 都会退化为恒选第一个健康账户。这里提升为按 backend 共享的全局状态。
var (
	accountSelectorStoreMu sync.Mutex
	accountSelectorStore   = map[string]accountSelectorEntry{}
)

type accountSelectorEntry struct {
	selector *backend.AccountPoolSelector
	shape    string
}

// accountPoolShape 计算账户池指纹；池形态变化（增删账户/权重/策略）时重建选择器。
func accountPoolShape(pool *backend.AccountPoolConfig) string {
	if pool == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(pool.Strategy))))
	for _, acc := range pool.Accounts {
		fmt.Fprintf(h, "|%s:%d:%v", acc.ID, acc.Weight, acc.Enabled)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// getAccountSelector 获取指定后端的共享选择器；未配置账户池时返回 nil。
func getAccountSelector(backendID string, pool *backend.AccountPoolConfig) *backend.AccountPoolSelector {
	if pool == nil || strings.TrimSpace(backendID) == "" {
		return nil
	}
	shape := accountPoolShape(pool)
	accountSelectorStoreMu.Lock()
	defer accountSelectorStoreMu.Unlock()
	if e, ok := accountSelectorStore[backendID]; ok && e.shape == shape {
		return e.selector
	}
	sel := backend.NewAccountPoolSelector()
	accountSelectorStore[backendID] = accountSelectorEntry{selector: sel, shape: shape}
	return sel
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
		// 新字段：system_prompt_strategy（优先于 inject_system_prompt）
		if s, ok := config.CustomConfig["system_prompt_strategy"].(string); ok {
			node.SystemPromptStrategy = promptstrategy.ResolveSystemMode(s, nil)
		} else {
			// 兼容旧字段
			node.SystemPromptStrategy = promptstrategy.ResolveSystemMode("", &node.InjectSystemPrompt)
		}
		if s, ok := config.CustomConfig["append_position"].(string); ok {
			node.AppendPosition = promptstrategy.AppendPosition(strings.TrimSpace(s))
		}
	} else {
		// 无 custom_config 时，使用默认映射
		node.SystemPromptStrategy = promptstrategy.ResolveSystemMode("", &node.InjectSystemPrompt)
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
	// 真实代理场景：raw_request_body 由 attachTransparentRequestMetadata 填充（完整 JSON）。
	// /execute 等结构化入口：无 raw_request_body 时优先用 input.Messages 组装完整 chat 体
	//（此前仅用 input.Content 构造单条消息，messages-only 请求在上游变成空输入，
	// 报 "Input must have at least 1 token."）；再退化为单条 Content（WebUI 快速测试）。
	if len(body) == 0 && input != nil {
		model := strings.TrimSpace(n.config.Model)
		if model == "" && meta != nil {
			model = strings.TrimSpace(stringMeta(meta, "model"))
		}
		if len(input.Messages) > 0 {
			body = buildChatBodyFromMessages(input.Messages, model)
		} else {
			body = buildMinimalChatBody(strings.TrimSpace(input.Content), model)
		}
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("transparent_forward node %q: empty request body", n.id)
	}

	clientModel := extractJSONModel(body)
	backendID, resolvedModel, body := n.resolveTransparentRoute(ctx, meta, body, clientModel)

	targetURL, err := n.resolveTargetURL(meta, backendID, requestPath)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: %w", n.id, err)
	}

	// Responses / Anthropic Messages 客户端 POST 到 /v1/responses 或 /v1/messages，
	// 但配置后端出站统一为 /chat/completions。
	// 必须先于 inject_system_prompt：否则 inject 会在异协议体上误写 messages，
	// 导致桥接失败，上游 chat SSE 会原样打给非 chat 客户端。
	bridgeToChat := false
	anthropicToChat := false
	if strings.Contains(targetURL, "/chat/completions") {
		body, bridgeToChat, anthropicToChat = applyChatCompletionsRequestBridges(body, requestPath)
		// 无论是否走了全文转换，都清洗 tools：
		// 已是 chat 形态但带 flat/hosted tools 时，智谱会报 tools[0].function 不能为空。
		if sanitized, ok := sanitizeChatCompletionsTools(body); ok {
			body = sanitized
		}
	}

	// 直连注入 gateway system：仅在 chat messages 形态上替换；须在 Responses→Chat 之后。
	// 使用新的 promptstrategy 算子，支持 passthrough/append/replace 三种模式
	if n.SystemPromptStrategy != promptstrategy.SystemModePassthrough {
		systemPrompt := strings.TrimSpace(n.config.SystemPrompt)
		if systemPrompt == "" {
			if execCtx, ok := ctx.Value(executionContextKey{}).(*ExecutionContext); ok && execCtx != nil && execCtx.pipeline != nil {
				systemPrompt = strings.TrimSpace(execCtx.pipeline.GlobalConfig.SystemPrompt)
			}
		}
		if systemPrompt != "" && !looksLikeResponsesBody(body) {
			// 解析 body 中的 messages
			var messages []promptstrategy.Message
			if rawMap, err := parseChatBody(body); err == nil {
				messages = rawMap
			}
			// 应用策略
			result, err := promptstrategy.ApplySystemStrategy(promptstrategy.SystemApplyInput{
				Mode:           n.SystemPromptStrategy,
				GatewayPrompt:  systemPrompt,
				AppendPosition: n.AppendPosition,
				Messages:       messages,
				RawBody:        body,
			})
			if err == nil && result.Applied {
				body = result.RawBody
			}
		}
	}
	// 旧 injectSystemPromptIntoChatBody 已收敛为 ApplySystemStrategy 薄封装；
	// NewTransparentForwardNode 总会 ResolveSystemMode，不再走平行分支。

	// DeepSeek thinking+tools：窄触发补回客户端丢掉的 reasoning_content（见 reasoning_roundtrip.go）。
	body, _ = applyReasoningRoundtripOnRequest(meta, body)

	client, err := n.getHTTPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: %w", n.id, err)
	}

	// 账户池优先于跨后端降级：限额/鉴权/5xx/Router.Unavailable 等先换同后端其它 Key；
	// 池内启用 Key 全部失败后，再走同后端换模型 / 备用后端（billingFallback + FallbackGroups）。
	// 未配置账户池时回退到单 Key（resolveTransparentUpstreamAuth）。
	pool := resolveTransparentAccountPool(backendID)

	maxAttempts := 1
	if pool != nil {
		if n := countEnabledPoolAccounts(pool); n > 1 {
			maxAttempts = n
		}
	}
	sessionKey := backend.ExtractSessionKey(ctx, body, stringMeta(meta, "session_id"))
	selector := getAccountSelector(backendID, pool)
	triedAccounts := map[string]bool{}

	var (
		statusCode    int
		contentType   string
		respBody      []byte
		poolExhausted bool
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		attemptReq, reqErr := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(string(body)))
		if reqErr != nil {
			return nil, fmt.Errorf("transparent_forward node %q: build request: %w", n.id, reqErr)
		}
		attemptReq.Header.Set("Content-Type", "application/json")

		// 已解析到配置后端时，优先用后端 API Key 鉴权上游。
		// 客户端 Authorization 是 Centag 网关鉴权（JWT / 网关 API Key），不能原样转发给上游，
		// 否则会出现直连正常、透明模式 AuthError: Invalid API key。
		// 无托管后端（高级旁路绝对 URL）时才透传 forward_authorization。
		auth := resolveTransparentUpstreamAuth(backendID, meta)
		accountID := ""
		if pool != nil && selector != nil {
			result, selErr := selectUnusedPoolAccount(ctx, selector, pool, sessionKey, triedAccounts, maxAttempts, backendID)
			if selErr == nil && result != nil {
				auth = "Bearer " + backend.NormalizeOpenAICompatibleAPIKey(result.Key)
				accountID = result.Account.ID
				triedAccounts[accountID] = true
			} else {
				// 池内已无未尝试的健康账户：结束 Key 轮换，用最后一次结果走后续降级。
				logger.Warn("transparent_forward account pool exhausted, stop key rotate",
					logger.GetField("node_id", n.id),
					logger.GetField("backend_id", backendID),
					logger.GetField("tried", len(triedAccounts)),
					logger.GetField("error", errString(selErr)),
				)
				if attempt == 0 {
					// 首次即无可用账户：退化为单 Key 出站一次，再交给跨模型/跨后端降级。
					accountID = ""
				} else {
					break
				}
			}
		}
		poolStrategy := ""
		if pool != nil {
			poolStrategy = pool.Strategy
		}
		logger.Info("transparent_forward outbound",
			logger.GetField("node_id", n.id),
			logger.GetField("backend_id", backendID),
			logger.GetField("model", resolvedModel),
			logger.GetField("fixed_egress", n.FixedEgress),
			logger.GetField("target_url", targetURL),
			logger.GetField("account_pool", pool != nil),
			logger.GetField("account_id", accountID),
			logger.GetField("pool_strategy", poolStrategy),
			logger.GetField("attempt", attempt+1),
			logger.GetField("max_attempts", maxAttempts),
		)
		if auth != "" {
			attemptReq.Header.Set("Authorization", auth)
		}

		currentResp, doErr := client.Do(attemptReq)
		if doErr != nil {
			// 网络错误也换下一把 Key（同后端优先），勿直接跳跨后端。
			if pool != nil && selector != nil && accountID != "" && attempt < maxAttempts-1 {
				logger.Info("transparent_forward rotate account",
					logger.GetField("node_id", n.id),
					logger.GetField("backend_id", backendID),
					logger.GetField("account_id", accountID),
					logger.GetField("error", doErr.Error()),
					logger.GetField("attempt", attempt+1),
					logger.GetField("max_attempts", maxAttempts),
				)
				selector.DisableAccountTemporarily(pool, accountID, backendID)
				continue
			}
			return nil, fmt.Errorf("transparent_forward node %q: upstream request failed: %w", n.id, doErr)
		}

		// [+] 处理 301/302 重定向
		if (currentResp.StatusCode == 301 || currentResp.StatusCode == 302) && n.RedirectPolicy != "never" {
			location := currentResp.Header.Get("Location")
			if location != "" {
				// smart 模式：仅 GET/HEAD 跟随
				if n.RedirectPolicy != "smart" || (method == http.MethodGet || method == http.MethodHead) {
					// 跟随重定向
					currentResp.Body.Close()
					currentResp, doErr = n.followRedirect(ctx, attemptReq, location, client, method, body)
					if doErr != nil {
						return nil, fmt.Errorf("transparent_forward node %q: redirect failed: %w", n.id, doErr)
					}
				}
			}
		}

		currentBody, readErr := io.ReadAll(currentResp.Body)
		currentResp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("transparent_forward node %q: read response: %w", n.id, readErr)
		}

		// 可轮换失败且池内还有未尝试 Key → 临时禁用当前账户并换下一个（先于跨后端降级）。
		if retryableAccountFailure(currentResp.StatusCode, string(currentBody)) && pool != nil && accountID != "" && attempt < maxAttempts-1 {
			logger.Info("transparent_forward rotate account",
				logger.GetField("node_id", n.id),
				logger.GetField("backend_id", backendID),
				logger.GetField("account_id", accountID),
				logger.GetField("status_code", currentResp.StatusCode),
				logger.GetField("attempt", attempt+1),
				logger.GetField("max_attempts", maxAttempts),
			)
			selector.DisableAccountTemporarily(pool, accountID, backendID)
			continue
		}

		statusCode = currentResp.StatusCode
		contentType = currentResp.Header.Get("Content-Type")
		respBody = currentBody
		break
	}

	// 账户池耗尽判定：循环自然结束（所有 attempt 用完）且池已配置、已尝试过账户、
	// 最后一次返回的是可轮换失败（429/5xx 等）。此时 executeWithRetry 不应再重试，
	// 否则 N 个 Key × M 次重试 = N×M 倍请求放大（免费额度按次计费场景下尤其致命）。
	if pool != nil && len(triedAccounts) > 0 && retryableAccountFailure(statusCode, string(respBody)) {
		poolExhausted = true
	}

	bodyStr := string(respBody)

	// 账户池已耗尽（或未配置）后：同后端换模型 → 备用后端；最后向上返回 error 供 FallbackGroups。
	// 纯鉴权 401（池已穷尽）仍透传给客户端。
	if config.IsBillingOrQuotaFailure(statusCode, bodyStr) {
		if out, ok := n.retryWithSystemBillingFallback(ctx, client, method, meta, body, backendID, resolvedModel, clientModel, requestPath, bridgeToChat, anthropicToChat, statusCode, bodyStr); ok {
			return out, nil
		}
		return nil, newTransparentUpstreamError(n.id, backendID, resolvedModel, targetURL, statusCode, bodyStr, poolExhausted)
	}

	// 模型不存在 / Router.Unavailable：池 Key 已轮换完毕后，再同后端换模型。
	if statusCode >= 400 && (isUpstreamModelOrPlaceholderError(bodyStr) || isUpstreamRouterUnavailable(bodyStr)) {
		if out, ok := n.retryWithSystemBillingFallback(ctx, client, method, meta, body, backendID, resolvedModel, clientModel, requestPath, bridgeToChat, anthropicToChat, statusCode, bodyStr); ok {
			return out, nil
		}
		return nil, newTransparentUpstreamError(n.id, backendID, resolvedModel, targetURL, statusCode, bodyStr, poolExhausted)
	}

	// 对可重试的 HTTP 错误码返回 error，触发上层重试/降级逻辑（此时同后端 Key 已尝试过）。
	if config.IsRetryableStatusCode(statusCode) {
		return nil, newTransparentUpstreamError(n.id, backendID, resolvedModel, targetURL, statusCode, bodyStr, poolExhausted)
	}

	out := n.buildTransparentOutput(targetURL, statusCode, contentType, respBody, backendID, resolvedModel, clientModel, requestPath, bridgeToChat, anthropicToChat, nil)
	applyReasoningRoundtripOnResponse(meta, body, respBody, statusCode)

	// 上游返回空正文（HTTP 成功但 0 token/无正文/无工具调用）视为节点失败：
	// 交给降级/错误处理，否则主备都会被当成「成功但无输出」，客户端拿不到真实错误。
	if statusCode >= 200 && statusCode < 400 && transparentOutputIsEmpty(out) {
		return nil, newTransparentUpstreamError(n.id, backendID, resolvedModel, targetURL, http.StatusBadGateway,
			"upstream returned an empty response (0 tokens)")
	}

	return out, nil
}

// transparentOutputIsEmpty 判断透明转发节点输出是否为空（无正文、无工具调用、无推理）。
func transparentOutputIsEmpty(out *NodeOutput) bool {
	if out == nil {
		return true
	}
	return strings.TrimSpace(out.Content) == "" &&
		len(out.ToolCalls) == 0 &&
		strings.TrimSpace(out.ReasoningContent) == ""
}

// retryableAccountFailure 同后端账户池内是否应换下一把 Key（优先于跨后端）：
// 429 限流、401/402/403、5xx、Router.Unavailable、计费/额度类失败。
// 明确的客户端坏请求（400）不轮换 Key。
func retryableAccountFailure(statusCode int, body string) bool {
	if statusCode == http.StatusBadRequest {
		return config.IsBillingOrQuotaFailure(statusCode, body)
	}
	switch statusCode {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	}
	if statusCode >= 500 {
		return true
	}
	if isUpstreamRouterUnavailable(body) {
		return true
	}
	return config.IsBillingOrQuotaFailure(statusCode, body)
}

func countEnabledPoolAccounts(pool *backend.AccountPoolConfig) int {
	if pool == nil {
		return 0
	}
	n := 0
	for _, acc := range pool.Accounts {
		if acc.Enabled {
			n++
		}
	}
	return n
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// selectUnusedPoolAccount 选择尚未在本请求尝试过的健康账户；若选到已尝试账户则临时禁用后重选。
func selectUnusedPoolAccount(
	ctx context.Context,
	selector *backend.AccountPoolSelector,
	pool *backend.AccountPoolConfig,
	sessionKey string,
	tried map[string]bool,
	maxTries int,
	backendID string,
) (*backend.AccountPoolResult, error) {
	var lastErr error
	for i := 0; i < maxTries; i++ {
		result, err := selector.SelectAccountForRequest(ctx, pool, sessionKey, backendID)
		if err != nil {
			return nil, err
		}
		if result == nil || result.Account.ID == "" {
			return nil, fmt.Errorf("empty account selection")
		}
		if !tried[result.Account.ID] {
			return result, nil
		}
		selector.DisableAccountTemporarily(pool, result.Account.ID, backendID)
		lastErr = fmt.Errorf("account %s already tried", result.Account.ID)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no unused healthy accounts")
	}
	return nil, lastErr
}

// resolveTransparentAccountPool 解析后端账户池；无托管后端或未配置账户池时返回 nil。
func resolveTransparentAccountPool(backendID string) *backend.AccountPoolConfig {
	if ResolveBackendEndpoint == nil || strings.TrimSpace(backendID) == "" {
		return nil
	}
	ep, err := ResolveBackendEndpoint(backendID)
	if err != nil || ep == nil {
		return nil
	}
	return ep.AccountPool
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

// isUpstreamRouterUnavailable OpenCode Zen 等网关对暂不可达模型返回 Router.Unavailable。
func isUpstreamRouterUnavailable(body string) bool {
	return strings.Contains(body, "Router.Unavailable") ||
		strings.Contains(strings.ToLower(body), "router.unavailable")
}

// retryWithSystemBillingFallback 在余额/额度失败时按候选链再打：配置的 fallback_* → 同后端免费档模型。
func (n *TransparentForwardNode) retryWithSystemBillingFallback(
	ctx context.Context,
	client HTTPClient,
	method string,
	meta map[string]interface{},
	body []byte,
	primaryBackend, primaryModel, clientModel, requestPath string,
	bridgeToChat, anthropicToChat bool,
	primaryStatusCode int, primaryRespBody string,
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
	cands := billingFallbackCandidatesContext(ctx, primaryBackend, failedModels)
	if len(cands) == 0 {
		return nil, false
	}
	for _, cand := range cands {
		out, ok := n.doBillingFallbackAttempt(ctx, client, method, meta, body, primaryBackend, primaryModel, clientModel, requestPath, bridgeToChat, anthropicToChat, cand.backendID, cand.model, primaryStatusCode, primaryRespBody)
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
	return billingFallbackCandidatesContext(context.Background(), primaryBackend, failedModels)
}

func billingFallbackCandidatesContext(ctx context.Context, primaryBackend string, failedModels map[string]bool) []billingFallbackCandidate {
	primaryBackend = strings.TrimSpace(primaryBackend)
	fbBackend, fbModel := ResolveVirtualVarsContext(ctx, "{{system.fallback_backend}}", "{{system.fallback_model}}")
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

	// 同后端换模型优先；其它后端最后（账户池 Key 轮换已在外层完成）。
	// 注意：不自动替换为「同后端免费档模型」——用户显式配置的主/备模型不可用时
	// 应如实失败（由流水线降级组继续尝试用户配置的备用节点），而不是静默改用未指定的免费模型。
	sameBackendFallbackModel := fbModel
	if !strings.EqualFold(fbBackend, primaryBackend) {
		sameBackendFallbackModel = ""
	}
	add(primaryBackend, sameBackendFallbackModel)
	if fbBackend != "" && !strings.EqualFold(fbBackend, primaryBackend) && !strings.Contains(fbBackend, "{{") {
		add(fbBackend, fbModel)
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
	bridgeToChat, anthropicToChat bool,
	fbBackend, fbModel string,
	primaryStatusCode int, primaryRespBody string,
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
	attemptBridge := bridgeToChat
	attemptAnthropic := anthropicToChat
	if strings.Contains(targetURL, "/chat/completions") {
		var convertedAnthropic bool
		retryBody, attemptBridge, convertedAnthropic = applyChatCompletionsRequestBridges(retryBody, requestPath)
		if convertedAnthropic {
			attemptAnthropic = true
		}
		if sanitized, ok := sanitizeChatCompletionsTools(retryBody); ok {
			retryBody = sanitized
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
		"fallback_reason":               MaskSensitiveData(fmt.Sprintf("HTTP %d: %s", primaryStatusCode, truncateRespBody(primaryRespBody, 200))),
	}
	out := n.buildTransparentOutput(targetURL, resp.StatusCode, resp.Header.Get("Content-Type"), respBody, fbBackend, fbModel, clientModel, requestPath, attemptBridge, attemptAnthropic, extra)
	applyReasoningRoundtripOnResponse(meta, body, respBody, resp.StatusCode)
	AnnotateFallbackNotice(out)
	return out, true
}

func (n *TransparentForwardNode) buildTransparentOutput(
	targetURL string,
	statusCode int,
	contentType string,
	respBody []byte,
	backendID, resolvedModel, clientModel, requestPath string,
	bridgeToChat, anthropicToChat bool,
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
	// 记录 system prompt 策略信息
	if n.SystemPromptStrategy != promptstrategy.SystemModePassthrough {
		outMeta["system_prompt_strategy"] = string(n.SystemPromptStrategy)
		outMeta["inject_system_prompt"] = true
	} else if n.InjectSystemPrompt {
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
	// P1-T3：上游以 2xx 返回错误对象时，映射为规范 HTTP 状态写回 metadata，
	// chat 路径（transparentPassthroughStatusAndType）与 /execute 均据此返回真实状态。
	// 真实 4xx/5xx 保持原有透传语义（见 Plain401AuthStillPassthrough 回归）。
	if statusCode >= 200 && statusCode < 400 {
		if mapped, bad := DetectUpstreamErrorPayload(contentType, respBody); bad {
			outMeta["upstream_error"] = true
			outMeta["status_code"] = mapped
		}
	}
	var toolCalls []ToolCall
	finishReason := ""
	reasoning := ""
	if bridgeToChat && statusCode < 400 {
		// 无论是否抽到正文，都必须关闭透传：否则 chat.completion SSE 会原样打给
		// /v1/responses 或 /v1/messages 客户端，格式对不上会一直转圈或报错。
		extracted := extractChatCompletionResult(respBody)
		content = extracted.Text
		reasoning = extracted.Reasoning
		toolCalls = extracted.ToolCalls
		finishReason = extracted.FinishReason
		outMeta["raw_passthrough"] = false
		outMeta["responses_to_chat"] = true // 兼容既有 FormatChunk 判定
		if anthropicToChat || isMessagesAPIPath(requestPath) {
			outMeta["anthropic_to_chat"] = true
		}
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
			if bid == "" {
				// 兼容键名：/execute 与 OpenAI 客户端习惯用 backend
				bid = strings.TrimSpace(stringMeta(meta, "backend"))
			}
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
	ctx context.Context,
	meta map[string]interface{},
	body []byte,
	clientModel string,
) (backendID, resolvedModel string, outBody []byte) {
	outBody = body

	pinnedBackend := strings.TrimSpace(stringMeta(meta, "backend_id"))
	if pinnedBackend == "" {
		// 兼容键名：/execute 与 OpenAI 客户端习惯用 backend
		pinnedBackend = strings.TrimSpace(stringMeta(meta, "backend"))
	}
	preferredBackend, preferredModel := "", ""
	if ov, ok := config.ProxyDefaultsFromContext(ctx); ok {
		preferredBackend = strings.TrimSpace(ov.DefaultBackendID)
		preferredModel = strings.TrimSpace(ov.DefaultModel)
	}

	if n.FixedEgress {
		// 直连固定出站：只用节点配置的后端与模型，忽略请求头或 Agent 注入的 X-Backend-ID。
		// 具体 ID（非 {{system.*}}）绝不回落到「第一个可用后端 / 系统默认」，避免钉死 zen 却打到 bigmodel。
		rawBackend := strings.TrimSpace(n.config.Backend)
		if rawBackend != "" && !strings.Contains(rawBackend, "{{") {
			backendID = rawBackend
		} else {
			// {{system.fallback_backend}} 必须解析为系统降级后端，不能被「我的默认后端」抢占，
			// 否则 primary=quota 失败后 forward_fallback 仍打回同一个不可用后端。
			pin := preferredBackend
			if strings.Contains(rawBackend, "fallback_backend") {
				pin = ""
			}
			backendID = resolveFallbackBackendID(ctx, rawBackend, pin)
		}
		if backendID != "" {
			modelHint := n.config.Model
			// fallback 节点用系统/节点模型；仅 default_backend 类节点才回落「我的默认模型」
			if preferredModel != "" && (modelHint == "" || strings.Contains(modelHint, "{{system.default_model}}")) {
				modelHint = preferredModel
			}
			if strings.Contains(strings.TrimSpace(n.config.Model), "fallback_model") {
				modelHint = n.config.Model
			}
			resolvedModel, outBody = applyFallbackModel(ctx, outBody, backendID, modelHint)
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
			resolvedModel, outBody = applyFallbackModel(ctx, outBody, backendID, n.config.Model)
			return backendID, resolvedModel, outBody
		}

		// Team 用户「我的默认后端」优先：仅当客户端模型能在该后端命中时钉死，
		// 避免 deepseek-*-free 被跨后端匹配到 opencode-zen；若模型明确属于其它后端（如 glm-4-flash），
		// 则继续跨后端匹配，不能强行改写到默认后端的默认模型。
		if preferredBackend != "" {
			if mapping := matchModelOnBackend(clientModel, preferredBackend); mapping != nil {
				backendID = preferredBackend
				resolvedModel, outBody = applyClientModelRewrite(outBody, clientModel, mapping)
				return backendID, resolvedModel, outBody
			}
		}

		if matchedBackend, mapping := matchClientModelAcrossBackends(clientModel); matchedBackend != nil && mapping != nil {
			if isBackendAllowed(ctx, matchedBackend.ID) {
				backendID = matchedBackend.ID
				resolvedModel, outBody = applyClientModelRewrite(outBody, clientModel, mapping)
				return backendID, resolvedModel, outBody
			}
			// Backend not in allowlist: fall through to find an allowed backend
		}

		// 客户端模型无处可匹配：回落到我的默认后端 + 默认模型
		if preferredBackend != "" && isBackendAllowed(ctx, preferredBackend) {
			backendID = preferredBackend
			modelHint := preferredModel
			if modelHint == "" {
				modelHint = n.config.Model
			}
			resolvedModel, outBody = applyFallbackModel(ctx, outBody, backendID, modelHint)
			return backendID, resolvedModel, outBody
		}
	}

	// Fallback: 用户默认 → 节点/系统默认 → first usable enabled backend
	backendID = resolveFallbackBackendID(ctx, n.config.Backend, firstNonEmpty(pinnedBackend, preferredBackend))
	if backendID != "" && !isBackendAllowed(ctx, backendID) {
		// Fallback backend not in allowlist: clear to signal no valid route
		backendID = ""
	}
	if backendID != "" {
		modelHint := n.config.Model
		if preferredModel != "" && (modelHint == "" || strings.Contains(modelHint, "{{")) {
			modelHint = preferredModel
		}
		resolvedModel, outBody = applyFallbackModel(ctx, outBody, backendID, modelHint)
	}
	return backendID, resolvedModel, outBody
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// resolveFallbackBackendID resolves {{system.default_backend}} / empty to a concrete backend.
// When DefaultBackendID is unset, falls back to the first usable enabled backend.
func resolveFallbackBackendID(ctx context.Context, nodeBackend, pinnedBackend string) string {
	candidates := []string{
		strings.TrimSpace(pinnedBackend),
		strings.TrimSpace(nodeBackend),
	}
	for _, c := range candidates {
		// 空候选必须跳过：ResolveVirtualVarsContext("") 会当成 default_backend，
		// 从而在解析 {{system.fallback_backend}} 之前错误返回用户/系统默认后端。
		if c == "" {
			continue
		}
		if strings.Contains(c, "{{system.") {
			resolved, _ := ResolveVirtualVarsContext(ctx, c, "")
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

func applyFallbackModel(ctx context.Context, body []byte, backendID, nodeModel string) (resolved string, out []byte) {
	out = body
	_, resolved = ResolveVirtualVarsContext(ctx, backendID, nodeModel)
	if resolved == "" {
		_, resolved = ResolveVirtualVarsContext(ctx, "{{system.default_backend}}", "{{system.default_model}}")
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

// injectSystemPromptIntoChatBody 兼容旧调用点：等价于 system_prompt_strategy=replace。
// 不对 Responses 形态动手；非 system 消息字段（tool_calls / 多模态 content）由 ApplySystemStrategy 保真。
func injectSystemPromptIntoChatBody(body []byte, systemPrompt string) ([]byte, bool) {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" || len(body) == 0 {
		return body, false
	}
	if looksLikeResponsesBody(body) || looksLikeAnthropicMessagesBody(body) {
		return body, false
	}
	result, err := promptstrategy.ApplySystemStrategy(promptstrategy.SystemApplyInput{
		Mode:          promptstrategy.SystemModeReplace,
		GatewayPrompt: systemPrompt,
		RawBody:       body,
	})
	if err != nil || !result.Applied || len(result.RawBody) == 0 {
		return body, false
	}
	return result.RawBody, true
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

// buildChatBodyFromMessages 用结构化 Messages 组装完整 chat-completions 请求体。
// 保留 role/content/tool_calls/tool_call_id/reasoning_content 全量字段，
// 供 /execute 等仅传 Messages 的入口回退使用（对齐 chat 入口的原始 body 形态）。
func buildChatBodyFromMessages(messages []Message, model string) []byte {
	if model == "" {
		model = "default"
	}
	body := struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}{Model: model, Messages: messages}
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

// parseChatBody 从 chat JSON body 中解析 messages 为 promptstrategy.Message 列表
func parseChatBody(body []byte) ([]promptstrategy.Message, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	msgsRaw, ok := raw["messages"]
	if !ok {
		return nil, nil
	}

	msgsArr, ok := msgsRaw.([]interface{})
	if !ok {
		return nil, nil
	}

	messages := make([]promptstrategy.Message, 0, len(msgsArr))
	for _, m := range msgsArr {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := mm["role"].(string)
		content, _ := mm["content"].(string)
		messages = append(messages, promptstrategy.Message{
			Role:    role,
			Content: content,
		})
	}

	return messages, nil
}

func truncateRespBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "..."
}

// isBackendAllowed 检查后端是否在用户计费组白名单内。
// FilterAllowedBackend 为 nil 时放行所有后端（无白名单约束）。
func isBackendAllowed(ctx context.Context, backendID string) bool {
	return IsBackendAllowed(ctx, backendID)
}

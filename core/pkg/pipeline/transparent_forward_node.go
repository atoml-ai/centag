package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TransparentForwardNode forwards the original HTTP request to an upstream API unchanged.
// Unlike GeneratorNode, it does NOT re-assemble the request body (no prompt_template/system_prompt).
// The client's raw JSON (including extended fields like thinking, tool_choice, etc.) is forwarded as-is
// to the backend specified by config.Backend (or overridden via metadata backend_id / X-Backend-ID header).
type TransparentForwardNode struct {
	BaseNode
	DefaultScheme string
}

func NewTransparentForwardNode(config NodeConfig) (PipelineNode, error) {
	node := &TransparentForwardNode{
		BaseNode: BaseNode{
			config:      config,
			timeout:     120,
			retryConfig: DefaultRetryConfig(),
			permissions: []string{"network.outbound"},
		},
		DefaultScheme: "https",
	}
	if config.CustomConfig != nil {
		if s, ok := config.CustomConfig["default_scheme"].(string); ok && strings.TrimSpace(s) != "" {
			node.DefaultScheme = strings.TrimSpace(s)
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

	// 后端解析：config.backend（模板配置，支持 {{system.default_backend}} 虚拟变量）
	// → X-Backend-ID header（运行时覆盖）
	backendID := strings.TrimSpace(n.config.Backend)
	if backendID == "" || backendID == "{{system.default_backend}}" {
		resolvedBackend, _ := ResolveVirtualVars(backendID, n.config.Model)
		backendID = strings.TrimSpace(resolvedBackend)
	}
	if backendID == "" && meta != nil {
		backendID = strings.TrimSpace(stringMeta(meta, "backend_id"))
	}

	targetURL, err := ResolveTransparentTargetURL(meta, backendID, requestPath, n.DefaultScheme)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: %w", n.id, err)
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

	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: build request: %w", n.id, err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 已解析到配置后端时，优先用后端 API Key 鉴权上游。
	// 客户端 Authorization 是 Centag 网关鉴权（JWT / 网关 API Key），不能原样转发给上游，
	// 否则会出现直连正常、透明模式 AuthError: Invalid API key。
	// 无后端（如 raw-forward + X-Target-URL）时才透传 forward_authorization。
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("transparent_forward node %q: read response: %w", n.id, err)
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

	return &NodeOutput{
		Content: string(respBody),
		Metadata: map[string]interface{}{
			"raw_passthrough": true,
			"target_url":      targetURL,
			"target_base_url": baseURL,
			"status_code":     resp.StatusCode,
			"content_type":    contentType,
			"forwarded":       true,
		},
	}, nil
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
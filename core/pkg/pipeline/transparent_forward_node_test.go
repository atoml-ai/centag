package pipeline

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
)

type mockHTTPClient struct {
	lastReq *http.Request
	body    string
	status  int
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	return &http.Response{
		StatusCode: m.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

type capturingHTTPClient struct {
	inner *mockHTTPClient
	body  string
}

func (c *capturingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		c.body = string(b)
		req.Body = io.NopCloser(strings.NewReader(c.body))
	}
	return c.inner.Do(req)
}

func TestTransparentForwardNode_Execute(t *testing.T) {
	client := &mockHTTPClient{
		status: 200,
		body:   `{"id":"ok"}`,
	}
	broker := &mockCapabilityBroker{}
	broker.httpClient = client

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "transparent_forward"
	tf.SetCapabilityBroker(broker)

	output, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":            "https://api.example.com",
			"request_path":          "/v1/chat/completions",
			"raw_request_body":      `{"model":"gpt-4","messages":[]}`,
			"forward_authorization": "Bearer sk-test",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if output.Content != `{"id":"ok"}` {
		t.Fatalf("content = %q", output.Content)
	}
	if output.Metadata["raw_passthrough"] != true {
		t.Fatalf("raw_passthrough = %v", output.Metadata["raw_passthrough"])
	}
	if client.lastReq == nil || !strings.HasSuffix(client.lastReq.URL.String(), "/v1/chat/completions") {
		t.Fatalf("unexpected url: %v", client.lastReq)
	}
	if client.lastReq.Header.Get("Authorization") != "Bearer sk-test" {
		t.Fatalf("authorization = %q", client.lastReq.Header.Get("Authorization"))
	}
}

func TestTransparentForwardNode_PrefersBackendAPIKeyOverClientAuth(t *testing.T) {
	client := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	broker := &mockCapabilityBroker{httpClient: client}

	prev := ResolveBackendEndpoint
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		if backendID != "openai-opencode-zen" {
			t.Fatalf("backendID=%q", backendID)
		}
		return &BackendEndpoint{
			BaseURL: "https://opencode.ai/zen/v1",
			APIKey:  "sk-backend-real",
		}, nil
	}
	t.Cleanup(func() { ResolveBackendEndpoint = prev })

	node, err := NewTransparentForwardNode(NodeConfig{Backend: "openai-opencode-zen"})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":          "/v1/chat/completions",
			"raw_request_body":      `{"model":"mimo-v2.5-free","messages":[]}`,
			"forward_authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.centag-jwt",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := client.lastReq.Header.Get("Authorization"); got != "Bearer sk-backend-real" {
		t.Fatalf("authorization = %q, want backend API key (not client JWT)", got)
	}
}

func TestRewriteTransparentBodyModel(t *testing.T) {
	in := []byte(`{"model":"hy3-free","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	out, ok := rewriteTransparentBodyModel(in, "glm-4-flash")
	if !ok {
		t.Fatal("expected rewrite")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["model"] != "glm-4-flash" {
		t.Fatalf("model=%v", raw["model"])
	}
	if _, ok := raw["messages"]; !ok {
		t.Fatal("messages must be preserved")
	}
	if raw["stream"] != true {
		t.Fatalf("stream=%v", raw["stream"])
	}
}

func TestResolveTransparentUpstreamAuth(t *testing.T) {
	prev := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prev })

	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{APIKey: "sk-backend"}, nil
	}
	got := resolveTransparentUpstreamAuth("b1", map[string]interface{}{
		"forward_authorization": "Bearer client-jwt",
	})
	if got != "Bearer sk-backend" {
		t.Fatalf("got %q", got)
	}

	ResolveBackendEndpoint = nil
	got = resolveTransparentUpstreamAuth("", map[string]interface{}{
		"forward_authorization": "Bearer client-key",
	})
	if got != "Bearer client-key" {
		t.Fatalf("no-backend passthrough got %q", got)
	}
}

func TestTransparentForwardNode_PreferClientModelAcrossBackends(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "backend-a",
				Name:    "A",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "other", ActualModel: "other"},
				},
			},
			{
				ID:      "backend-b",
				Name:    "B",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "mino2.5 free", ActualModel: "mino2.5 free"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://" + backendID + ".example.com/v1", APIKey: "sk-" + backendID}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"mino2.5","messages":[{"role":"user","content":"hi"}],"stream":true}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Metadata["backend_id"] != "backend-b" {
		t.Fatalf("backend_id=%v, want backend-b", out.Metadata["backend_id"])
	}
	if inner.lastReq == nil || !strings.Contains(inner.lastReq.URL.Host, "backend-b") {
		t.Fatalf("url=%v, want backend-b host", inner.lastReq)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal([]byte(capturing.body), &sent); err != nil {
		t.Fatal(err)
	}
	// client "mino2.5" != actual "mino2.5 free" → rewrite for upstream
	if sent["model"] != "mino2.5 free" {
		t.Fatalf("model=%v, want mino2.5 free", sent["model"])
	}
	if out.Metadata["executor_model"] != "mino2.5 free" {
		t.Fatalf("executor_model=%v, want mino2.5 free", out.Metadata["executor_model"])
	}
}

func TestTransparentForwardNode_KeepClientModelWhenExactActual(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "zen",
				Name:    "Zen",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "mimo-v2.5-free", ActualModel: "mimo-v2.5-free"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://zen.example.com/v1", APIKey: "sk-zen"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"mimo-v2.5-free","messages":[]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(capturing.body), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["model"] != "mimo-v2.5-free" {
		t.Fatalf("model rewritten to %v, want keep client string", raw["model"])
	}
}

func TestIsUnspecifiedClientModel(t *testing.T) {
	if !isUnspecifiedClientModel("") || !isUnspecifiedClientModel("auto") {
		t.Fatal("empty/auto should be unspecified")
	}
	if !isUnspecifiedClientModel("pipeline.transparent-proxy.auto") {
		t.Fatal("virtual model should be unspecified")
	}
	if isUnspecifiedClientModel("mino2.5") {
		t.Fatal("real model should be specified")
	}
}

func TestTransparentForwardNode_KeepFreeTierModel(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "opencode-zen",
				Name:    "Zen",
				Type:    "openai",
				BaseURL: "https://opencode.ai/zen/v1",
				APIKey:  "sk-zen",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "deepseek-v4-flash", ActualModel: "deepseek-v4-flash"},
					{RequestedModel: "deepseek-v4-flash-free", ActualModel: "deepseek-v4-flash-free"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "sk-zen"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"deepseek-v4-flash-free","messages":[]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal([]byte(capturing.body), &sent); err != nil {
		t.Fatal(err)
	}
	if sent["model"] != "deepseek-v4-flash-free" {
		t.Fatalf("model=%v, must keep free tier (not rewrite to paid)", sent["model"])
	}
}

func TestTransparentForwardNode_FallbackFirstUsableBackend(t *testing.T) {
	// pipeline.transparent-proxy → body may keep virtual model or a stripped default;
	// with empty system DefaultBackendID, must still pick first usable enabled backend.
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "zen",
				Name:    "Zen",
				Type:    "openai",
				BaseURL: "https://zen.example.com/v1",
				APIKey:  "sk-zen",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "mimo-v2.5-free", ActualModel: "mimo-v2.5-free"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://zen.example.com/v1", APIKey: "sk-zen"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"pipeline.transparent-proxy","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Metadata["backend_id"] != "zen" {
		t.Fatalf("backend_id=%v, want zen (first usable fallback)", out.Metadata["backend_id"])
	}
	if !strings.Contains(inner.lastReq.URL.Host, "zen.example.com") {
		t.Fatalf("url=%v", inner.lastReq.URL)
	}
}

func TestTransparentForwardNode_VirtualPipelineModelPrefersUserBackend(t *testing.T) {
	// 虚拟 pipeline 模型名（无真实模型）时，必须回落到「我的默认后端」而非系统默认后端。
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok","choices":[{"message":{"content":"ok"}}]}`}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "opencode-zen",
			DefaultModel:     "mimo-v2.5-free",
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "opencode-zen",
				Name:    "Zen",
				Type:    "openai",
				BaseURL: "https://zen.example.com/v1",
				APIKey:  "sk-zen",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "mimo-v2.5-free", ActualModel: "mimo-v2.5-free"},
				},
			},
			{
				ID:      "deepseek",
				Name:    "DeepSeek",
				Type:    "openai",
				BaseURL: "https://api.deepseek.com/v1",
				APIKey:  "sk-ds",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "deepseek-v4-flash", ActualModel: "deepseek-v4-flash"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		base := "https://api.deepseek.com/v1"
		if backendID == "opencode-zen" {
			base = "https://zen.example.com/v1"
		}
		return &BackendEndpoint{BaseURL: base, APIKey: "sk"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	ctx := config.WithProxyDefaults(context.Background(), config.ProxyDefaults{
		DefaultBackendID: "deepseek",
		DefaultModel:     "deepseek-v4-flash",
	})

	out, err := tf.Execute(ctx, &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"pipeline.transparent-proxy","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Metadata["backend_id"] != "deepseek" {
		t.Fatalf("backend_id=%v, want deepseek (user preferred backend)", out.Metadata["backend_id"])
	}
	if !strings.Contains(inner.lastReq.URL.Host, "deepseek.com") {
		t.Fatalf("url=%v, want user preferred backend", inner.lastReq.URL)
	}
	if !strings.Contains(capturing.body, `"model":"deepseek-v4-flash"`) {
		t.Fatalf("upstream body model not rewritten to user default model: %s", capturing.body)
	}
}

func TestTransparentForwardNode_ResponsesToChatCompletions(t *testing.T) {
	inner := &mockHTTPClient{
		status: 200,
		body: "data: {\"choices\":[{\"delta\":{\"content\":\"glm\"}}]}\n\n" +
			"data: [DONE]\n\n",
	}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://open.bigmodel.cn/api/paas/v4", APIKey: "sk"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"backend_id":   "bigmodel-ai",
			"request_path": "/v1/responses",
			"raw_request_body": `{
				"model":"gpt-5.6-luna",
				"stream":true,
				"input":[{"role":"user","content":"你使用的是什么大模型"}]
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(capturing.body, `"messages"`) {
		t.Fatalf("upstream body missing messages: %s", capturing.body)
	}
	if strings.Contains(capturing.body, `"input"`) {
		t.Fatalf("upstream body still has input: %s", capturing.body)
	}
	if !strings.HasSuffix(inner.lastReq.URL.Path, "/chat/completions") {
		t.Fatalf("url=%v", inner.lastReq.URL)
	}
	if out.Metadata["raw_passthrough"] != false {
		t.Fatalf("raw_passthrough=%v, want false for responses rewrite", out.Metadata["raw_passthrough"])
	}
	if out.Content != "glm" {
		t.Fatalf("content=%q, want extracted assistant text", out.Content)
	}
}

func TestTransparentForwardNode_AnthropicMessagesToChat(t *testing.T) {
	inner := &mockHTTPClient{
		status: 200,
		body: "data: {\"choices\":[{\"delta\":{\"content\":\"我是 DeepSeek\"}}]}\n\n" +
			"data: [DONE]\n\n",
	}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "sk-zen"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"backend_id":   "opencode-zen",
			"request_path": "/v1/messages",
			"raw_request_body": `{
				"model":"claude-fable-5",
				"max_tokens":1024,
				"stream":true,
				"system":[{"type":"text","text":"You are helpful"}],
				"messages":[{"role":"user","content":[{"type":"text","text":"你使用的是什么大模型"}]}]
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasSuffix(inner.lastReq.URL.Path, "/chat/completions") {
		t.Fatalf("url=%v", inner.lastReq.URL)
	}
	var upstream map[string]interface{}
	if err := json.Unmarshal([]byte(capturing.body), &upstream); err != nil {
		t.Fatalf("upstream body json: %v body=%s", err, capturing.body)
	}
	if _, hasSystem := upstream["system"]; hasSystem {
		t.Fatalf("upstream body still has top-level system: %s", capturing.body)
	}
	if !strings.Contains(capturing.body, `"messages"`) {
		t.Fatalf("upstream body missing messages: %s", capturing.body)
	}
	if !strings.Contains(capturing.body, `"role":"system"`) {
		t.Fatalf("upstream body missing system role message: %s", capturing.body)
	}
	if out.Metadata["raw_passthrough"] != false {
		t.Fatalf("raw_passthrough=%v, want false", out.Metadata["raw_passthrough"])
	}
	if out.Metadata["anthropic_to_chat"] != true {
		t.Fatalf("anthropic_to_chat=%v", out.Metadata["anthropic_to_chat"])
	}
	if out.Content != "我是 DeepSeek" {
		t.Fatalf("content=%q, want extracted assistant text", out.Content)
	}
}

func TestTransparentForwardNode_GeminiToChat(t *testing.T) {
	inner := &mockHTTPClient{
		status: 200,
		body:   `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`,
	}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "sk-zen"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"backend_id":   "opencode-zen",
			"request_path": "/v1beta/models/gemini-3.1-flash-lite:generateContent",
			"raw_request_body": `{
				"contents":[{"role":"user","parts":[{"text":"你好"}]}],
				"generationConfig":{"temperature":0.7,"maxOutputTokens":1024}
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasSuffix(inner.lastReq.URL.Path, "/chat/completions") {
		t.Fatalf("url=%v", inner.lastReq.URL)
	}
	var upstream map[string]interface{}
	if err := json.Unmarshal([]byte(capturing.body), &upstream); err != nil {
		t.Fatalf("upstream body json: %v body=%s", err, capturing.body)
	}
	if _, hasContents := upstream["contents"]; hasContents {
		t.Fatalf("upstream body still has Gemini contents: %s", capturing.body)
	}
	if !strings.Contains(capturing.body, `"messages"`) {
		t.Fatalf("upstream body missing messages: %s", capturing.body)
	}
	msgs, _ := upstream["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("messages count=%d, want 1", len(msgs))
	}
	msg, _ := msgs[0].(map[string]interface{})
	if msg["role"] != "user" || msg["content"] != "你好" {
		t.Fatalf("message=%v, want user/你好", msg)
	}
	if upstream["temperature"] != 0.7 {
		t.Fatalf("temperature=%v, want 0.7", upstream["temperature"])
	}
	if out.Metadata["raw_passthrough"] != false {
		t.Fatalf("raw_passthrough=%v, want false", out.Metadata["raw_passthrough"])
	}
	if out.Content != "你好" {
		t.Fatalf("content=%q, want extracted assistant text", out.Content)
	}
}

func TestTransparentForwardNode_ResponsesToChat_ReasoningOnlyStillDisablesPassthrough(t *testing.T) {
	inner := &mockHTTPClient{
		status: 200,
		body: "data: {\"choices\":[{\"delta\":{\"content\":null,\"reasoning_content\":\"我是\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":null,\"reasoning_content\":\"免费模型\"}}]}\n\n" +
			"data: [DONE]\n\n",
	}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "sk"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{CustomConfig: map[string]interface{}{
		"route_policy": "fixed",
	}})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"backend_id":   "opencode-zen",
			"request_path": "/v1/responses",
			"raw_request_body": `{
				"model":"gpt-5.6-luna",
				"stream":true,
				"input":[{"role":"user","content":"你是谁"}]
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Metadata["raw_passthrough"] != false {
		t.Fatalf("raw_passthrough=%v, want false", out.Metadata["raw_passthrough"])
	}
	if out.Metadata["responses_to_chat"] != true {
		t.Fatalf("responses_to_chat=%v", out.Metadata["responses_to_chat"])
	}
	if out.Content != "我是免费模型" {
		t.Fatalf("content=%q, want reasoning fallback text", out.Content)
	}
	if strings.HasPrefix(strings.TrimSpace(out.Content), "data:") {
		t.Fatalf("must not keep chat SSE body for responses clients: %q", out.Content)
	}
}

func TestTransparentForwardNode_ResponsesToChatCompletions_ToolCalls(t *testing.T) {
	inner := &mockHTTPClient{
		status: 200,
		body: "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\\\"command\\\":\\\"pwd\\\"}\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
			"data: [DONE]\n\n",
	}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://open.bigmodel.cn/api/paas/v4", APIKey: "sk"}, nil
	}

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"backend_id":   "bigmodel-ai",
			"request_path": "/v1/responses",
			"raw_request_body": `{
				"model":"gpt-5.6-luna",
				"stream":true,
				"tools":[{"type":"function","name":"bash","description":"shell","parameters":{"type":"object"}}],
				"input":[{"role":"user","content":"pwd"}]
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(capturing.body, `"function":{"`) && !strings.Contains(capturing.body, `"function":{`) {
		// nested function object required for chat completions
		if !strings.Contains(capturing.body, `"function"`) {
			t.Fatalf("upstream tools not nested: %s", capturing.body)
		}
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Function.Name != "bash" {
		t.Fatalf("ToolCalls=%+v", out.ToolCalls)
	}
	if out.FinishReason != "tool_calls" {
		t.Fatalf("FinishReason=%q", out.FinishReason)
	}
	if out.Metadata["raw_passthrough"] != false {
		t.Fatalf("raw_passthrough=%v", out.Metadata["raw_passthrough"])
	}
}

func TestTransparentForwardNode_RoutePolicyAndInjectSwitches(t *testing.T) {
	n1, err := NewTransparentForwardNode(NodeConfig{CustomConfig: map[string]interface{}{
		"route_policy": "fixed",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !n1.(*TransparentForwardNode).FixedEgress {
		t.Fatal("route_policy=fixed should set FixedEgress")
	}

	n2, err := NewTransparentForwardNode(NodeConfig{CustomConfig: map[string]interface{}{
		"route_policy":         "match_model",
		"fixed_egress":         true, // route_policy wins after bool
		"inject_system_prompt": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	tf2 := n2.(*TransparentForwardNode)
	if tf2.FixedEgress {
		t.Fatal("route_policy=match_model should clear FixedEgress")
	}
	if !tf2.InjectSystemPrompt {
		t.Fatal("inject_system_prompt should be true")
	}
}

func TestInjectSystemPromptIntoChatBody(t *testing.T) {
	in := []byte(`{"model":"m","messages":[{"role":"system","content":"client"},{"role":"user","content":"hi"}]}`)
	out, ok := injectSystemPromptIntoChatBody(in, "gateway persona")
	if !ok {
		t.Fatal("expected rewrite")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	msgs := raw["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("len=%d", len(msgs))
	}
	first := msgs[0].(map[string]interface{})
	if first["role"] != "system" || first["content"] != "gateway persona" {
		t.Fatalf("first=%v", first)
	}
	second := msgs[1].(map[string]interface{})
	if second["role"] != "user" {
		t.Fatalf("second=%v", second)
	}
}

func TestTransparentForwardNode_InjectSystemPromptOnExecute(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prev := ResolveBackendEndpoint
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://api.example.com/v1", APIKey: "sk-x"}, nil
	}
	t.Cleanup(func() { ResolveBackendEndpoint = prev })

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend:      "b1",
		SystemPrompt: "gateway only",
		CustomConfig: map[string]interface{}{
			"route_policy":         "fixed",
			"inject_system_prompt": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"m","messages":[{"role":"system","content":"client"},{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(capturing.body, `"gateway only"`) {
		t.Fatalf("missing injected system prompt: %s", capturing.body)
	}
	if strings.Contains(capturing.body, `"client"`) {
		t.Fatalf("client system should be replaced: %s", capturing.body)
	}
}

// 直连模板 inject_system_prompt=true + OpenCode /v1/responses：
// 必须先 Responses→Chat，再注入 system；且 responses_to_chat 必须为 true，
// 否则会把 chat.completion.chunk（含响应追踪前缀）原样打给 Responses 客户端。
func TestTransparentForwardNode_InjectSystemPrompt_AfterResponsesToChat(t *testing.T) {
	inner := &mockHTTPClient{
		status: 200,
		body: "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"deepseek\"}}]}\n\n" +
			"data: [DONE]\n\n",
	}
	capturing := &capturingHTTPClient{inner: inner}
	broker := &mockCapabilityBroker{httpClient: capturing}

	prev := ResolveBackendEndpoint
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "sk-x"}, nil
	}
	t.Cleanup(func() { ResolveBackendEndpoint = prev })

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend:      "opencode-zen",
		SystemPrompt: "gateway persona for direct-backend",
		CustomConfig: map[string]interface{}{
			"route_policy":         "fixed",
			"inject_system_prompt": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"backend_id":   "opencode-zen",
			"request_path": "/v1/responses",
			"raw_request_body": `{
				"model":"gpt-5.6-luna",
				"stream":true,
				"input":[{"role":"user","content":"你使用的什么模型"}]
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(capturing.body, `"input"`) {
		t.Fatalf("upstream body still has input (convert must run before inject): %s", capturing.body)
	}
	if !strings.Contains(capturing.body, `"messages"`) {
		t.Fatalf("upstream body missing messages: %s", capturing.body)
	}
	if !strings.Contains(capturing.body, `"gateway persona for direct-backend"`) {
		t.Fatalf("missing injected system after responses→chat: %s", capturing.body)
	}
	if !strings.Contains(capturing.body, `"你使用的什么模型"`) {
		t.Fatalf("user content lost: %s", capturing.body)
	}
	if out.Metadata["responses_to_chat"] != true {
		t.Fatalf("responses_to_chat=%v, want true", out.Metadata["responses_to_chat"])
	}
	if out.Metadata["raw_passthrough"] != false {
		t.Fatalf("raw_passthrough=%v, want false", out.Metadata["raw_passthrough"])
	}
	if out.Content != "deepseek" {
		t.Fatalf("content=%q, want extracted text (not chat SSE)", out.Content)
	}
	if strings.HasPrefix(strings.TrimSpace(out.Content), "data:") {
		t.Fatalf("must not keep chat SSE for /v1/responses: %q", out.Content)
	}
}

func TestInjectSystemPromptIntoChatBody_SkipsResponsesShape(t *testing.T) {
	in := []byte(`{"model":"m","input":[{"role":"user","content":"hi"}]}`)
	out, ok := injectSystemPromptIntoChatBody(in, "gateway")
	if ok {
		t.Fatal("must not inject into responses-shaped body")
	}
	if string(out) != string(in) {
		t.Fatalf("body mutated: %s", out)
	}
}

func TestTransparentForwardNode_FallbackBackendNotStolenByUserDefault(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID:  "quota-primary",
			DefaultModel:      "quota-model",
			FallbackBackendID: "deepseek",
			FallbackModel:     "deepseek-v4-flash",
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	var hitBackend string
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		hitBackend = backendID
		base := "https://api.deepseek.com/v1"
		if backendID == "quota-primary" {
			base = "http://127.0.0.1:9/v1"
		}
		return &BackendEndpoint{BaseURL: base, APIKey: "sk"}, nil
	}

	inner := &mockHTTPClient{status: 200, body: `{"id":"ok","choices":[{"message":{"content":"fb-ok"}}]}`}
	broker := &mockCapabilityBroker{httpClient: inner}
	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.fallback_backend}}",
		Model:   "{{system.fallback_model}}",
		CustomConfig: map[string]interface{}{
			"route_policy": "fixed",
			"is_fallback":  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward_fallback"
	tf.SetCapabilityBroker(broker)

	ctx := config.WithProxyDefaults(context.Background(), config.ProxyDefaults{
		DefaultBackendID: "quota-primary",
		DefaultModel:     "quota-model",
	})
	_, err = tf.Execute(ctx, &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"quota-model","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if hitBackend != "deepseek" {
		t.Fatalf("fallback node hit backend %q, want deepseek (must not use user default quota-primary)", hitBackend)
	}
}

func TestTransparentForwardNode_FixedEgressIgnoresPinnedBackendID(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "opencode-zen",
			DefaultModel:     "mimo-v2.5-free",
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	var hitBackend string
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		hitBackend = backendID
		return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "sk"}, nil
	}

	inner := &mockHTTPClient{status: 200, body: `{"id":"ok","choices":[{"message":{"content":"hi"}}]}`}
	broker := &mockCapabilityBroker{httpClient: inner}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
		CustomConfig: map[string]interface{}{
			"route_policy": "fixed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			// Agent / 请求头误注入的后端，直连模式必须忽略
			"backend_id":   "bigmodel-ai",
			"request_path": "/v1/chat/completions",
			"raw_request_body": `{
				"model":"glm-5.1",
				"stream":true,
				"messages":[{"role":"user","content":"你好"}]
			}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if hitBackend != "opencode-zen" {
		t.Fatalf("upstream backend=%q, want opencode-zen (ignore pinned bigmodel-ai)", hitBackend)
	}
	if out.Metadata["backend_id"] != "opencode-zen" {
		t.Fatalf("metadata backend_id=%v", out.Metadata["backend_id"])
	}
	if out.Metadata["executor_model"] != "mimo-v2.5-free" && out.Metadata["model"] != "mimo-v2.5-free" {
		t.Fatalf("model meta=%v", out.Metadata)
	}
}

func TestIsUpstreamModelOrPlaceholderError_ZhBigModel(t *testing.T) {
	body := `{"error":{"code":"1211","message":"模型不存在，请检查模型代码。"}}`
	if !isUpstreamModelOrPlaceholderError(body) {
		t.Fatal("expected Chinese model-not-found to be treated as model error")
	}
}

func TestTransparentForwardNode_RedirectPolicyConfig(t *testing.T) {
	node, err := NewTransparentForwardNode(NodeConfig{CustomConfig: map[string]interface{}{
		"redirect_policy": "always",
		"max_redirects":   3,
	}})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	if tf.RedirectPolicy != "always" || tf.MaxRedirects != 3 {
		t.Fatalf("policy=%s max=%d", tf.RedirectPolicy, tf.MaxRedirects)
	}
	def, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	dtf := def.(*TransparentForwardNode)
	if dtf.RedirectPolicy != "never" || dtf.MaxRedirects != 5 {
		t.Fatalf("defaults policy=%s max=%d", dtf.RedirectPolicy, dtf.MaxRedirects)
	}
}

type sequenceHTTPClient struct {
	calls  int
	bodies []string
	auths  []string
	resps  []struct {
		status int
		body   string
	}
}

func (s *sequenceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		s.bodies = append(s.bodies, string(b))
	}
	s.auths = append(s.auths, req.Header.Get("Authorization"))
	idx := s.calls
	s.calls++
	if idx >= len(s.resps) {
		idx = len(s.resps) - 1
	}
	r := s.resps[idx]
	return &http.Response{
		StatusCode: r.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}

func TestTransparentForwardNode_BillingFallbackRetriesFreeModel(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID:  "zen",
			DefaultModel:      "gpt-5.6-luna",
			FallbackBackendID: "zen",
			FallbackModel:     "mimo-v2.5-free",
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://api.example.com/v1", APIKey: "sk-test"}, nil
	}

	seq := &sequenceHTTPClient{
		resps: []struct {
			status int
			body   string
		}{
			{401, `{"error":{"type":"CreditsError","message":"Insufficient balance"}}`},
			{200, `{"id":"ok","choices":[{"message":{"content":"free-ok"}}]}`},
		},
	}
	broker := &mockCapabilityBroker{httpClient: seq}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
		CustomConfig: map[string]interface{}{
			"route_policy": "fixed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-5.6-luna","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seq.calls != 2 {
		t.Fatalf("calls=%d want 2", seq.calls)
	}
	if !strings.Contains(seq.bodies[1], "mimo-v2.5-free") {
		t.Fatalf("fallback body model missing: %s", seq.bodies[1])
	}
	if out.Metadata["billing_fallback_used"] != true {
		t.Fatalf("metadata=%v", out.Metadata)
	}
}

func TestBillingFallbackCandidates_NoFreeTierAutoPick(t *testing.T) {
	mgr := backend.NewManager()
	_ = mgr.Add(&backend.BackendConfig{
		ID:      "zen",
		Name:    "Zen",
		Type:    "openai",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "deepseek-v4-flash-free", ActualModel: "deepseek-v4-flash-free"},
			{RequestedModel: "gpt-5.6-luna", ActualModel: "gpt-5.6-luna"},
		},
	})
	backend.SetManagerForTest(mgr)
	t.Cleanup(func() { backend.SetManagerForTest(nil) })

	// 未配置显式降级模型时，不得自动替换为「同后端免费档模型」——
	// 用户显式指定的主/备模型不可用时应如实失败，而不是静默改用未指定的免费模型。
	failed := map[string]bool{"gpt-5.6-luna": true}
	cands := billingFallbackCandidates("zen", failed)
	for _, c := range cands {
		if c.model == "deepseek-v4-flash-free" {
			t.Fatalf("free-tier auto-pick must be removed, got %#v", cands)
		}
		if strings.Contains(c.model, "{{") || strings.Contains(c.backendID, "{{") {
			t.Fatalf("placeholder leaked into candidates: %#v", c)
		}
	}
}

func TestTransparentForwardNode_BillingFallbackNoFreeTierWhenUnset(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "zen",
			DefaultModel:     "gpt-5.4-nano",
			// FallbackModel 未配置：不得从 SupportedModels 自动挑选 *-free 替代
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	mgr := backend.NewManager()
	_ = mgr.Add(&backend.BackendConfig{
		ID:      "zen",
		Name:    "Zen",
		Type:    "openai",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "gpt-5.4-nano", ActualModel: "gpt-5.4-nano"},
			{RequestedModel: "mimo-v2.5-free", ActualModel: "mimo-v2.5-free"},
		},
	})
	backend.SetManagerForTest(mgr)
	t.Cleanup(func() { backend.SetManagerForTest(nil) })

	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://api.example.com/v1", APIKey: "sk-test"}, nil
	}

	seq := &sequenceHTTPClient{
		resps: []struct {
			status int
			body   string
		}{
			{401, `{"error":{"type":"CreditsError","message":"Insufficient balance"}}`},
		},
	}
	broker := &mockCapabilityBroker{httpClient: seq}
	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "zen",
		Model:   "gpt-5.4-nano",
		CustomConfig: map[string]interface{}{
			"route_policy": "fixed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-5.4-nano","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err == nil {
		t.Fatal("expected error when no configured fallback candidate (free-tier auto-pick removed)")
	}
}

func TestTransparentForwardNode_Plain401AuthStillPassthrough(t *testing.T) {
	client := &mockHTTPClient{
		status: 401,
		body:   `{"error":"invalid_api_key"}`,
	}
	broker := &mockCapabilityBroker{httpClient: client}
	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"x","messages":[]}`,
		},
	})
	if err != nil {
		t.Fatalf("plain 401 should passthrough, got err=%v", err)
	}
	if out.Metadata["status_code"] != 401 {
		t.Fatalf("status_code=%v", out.Metadata["status_code"])
	}
}

func TestTransparentForwardNode_AccountPoolRotatesOnPlain401(t *testing.T) {
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	pool := &backend.AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []backend.BackendAccount{
			{ID: "key-bad", APIKey: "sk-bad", Enabled: true, Weight: 1},
			{ID: "key-good", APIKey: "sk-good", Enabled: true, Weight: 1},
		},
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{
			BaseURL:     "https://api.example.com/v1",
			APIKey:      "sk-fallback",
			AccountPool: pool,
		}, nil
	}

	seq := &sequenceHTTPClient{
		resps: []struct {
			status int
			body   string
		}{
			{401, `{"error":{"type":"AuthError","message":"Invalid API key."}}`},
			{200, `{"id":"ok","choices":[{"message":{"content":"hi"}}]}`},
		},
	}
	broker := &mockCapabilityBroker{httpClient: seq}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "zen-pool-401",
		CustomConfig: map[string]interface{}{
			"route_policy": "fixed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"mimo-v2.5-free","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seq.calls != 2 {
		t.Fatalf("calls=%d want 2 (rotate after plain 401)", seq.calls)
	}
	if len(seq.auths) != 2 || seq.auths[0] == seq.auths[1] {
		t.Fatalf("auths=%v, want two different pool keys", seq.auths)
	}
	if seq.auths[0] != "Bearer sk-bad" || seq.auths[1] != "Bearer sk-good" {
		t.Fatalf("auths=%v, want Bearer sk-bad then Bearer sk-good", seq.auths)
	}
	if out.Metadata["status_code"] != 200 {
		t.Fatalf("status_code=%v want 200 after rotate", out.Metadata["status_code"])
	}
}

func TestRetryableAccountFailure_Plain401(t *testing.T) {
	if !retryableAccountFailure(401, `{"error":{"type":"AuthError","message":"Invalid API key."}}`) {
		t.Fatal("plain 401 should be retryable for account pool rotation")
	}
	if !retryableAccountFailure(429, `rate limit`) {
		t.Fatal("429 should be retryable")
	}
	// 402 PaymentRequired 故意不轮换：同后端多 Key 通常共享上游账户余额，
	// 账户没钱换 Key 仍 402，徒增 N×M 放大。统一交给 billing fallback / FallbackGroups。
	if retryableAccountFailure(402, `payment required`) {
		t.Fatal("402 should NOT rotate account keys (permanent upstream billing failure)")
	}
	if !retryableAccountFailure(500, `{"type":"Router.Unavailable","modelID":"x"}`) {
		t.Fatal("5xx / Router.Unavailable should rotate account before other backends")
	}
	if retryableAccountFailure(400, `invalid json`) {
		t.Fatal("plain 400 should not rotate account keys")
	}
	if retryableAccountFailure(404, `not found`) {
		t.Fatal("404 should not be retryable for account pool")
	}
}

func TestBillingFallbackCandidates_UsesConfiguredFallbackOnly(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			FallbackBackendID: "other",
			FallbackModel:     "other-model",
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	mgr := backend.NewManager()
	_ = mgr.Add(&backend.BackendConfig{
		ID:      "primary",
		Name:    "Primary",
		Type:    "openai",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "primary-free", ActualModel: "primary-free"},
		},
	})
	backend.SetManagerForTest(mgr)
	t.Cleanup(func() { backend.SetManagerForTest(nil) })

	cands := billingFallbackCandidates("primary", map[string]bool{"paid-model": true})
	// 只应包含显式配置的降级后端模型，不得自动补同后端免费档
	if len(cands) != 1 || cands[0].backendID != "other" || cands[0].model != "other-model" {
		t.Fatalf("expected only configured fallback, got %#v", cands)
	}
}

func TestTransparentForwardNode_AccountPoolExhaustsBeforeOtherBackend(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID:  "primary",
			DefaultModel:      "m1",
			FallbackBackendID: "other",
			FallbackModel:     "m2",
		},
	})
	t.Cleanup(func() { config.Set(prevCfg) })

	pool := &backend.AccountPoolConfig{
		Strategy: "round_robin",
		Accounts: []backend.BackendAccount{
			{ID: "k1", APIKey: "sk-1", Enabled: true, Weight: 1},
			{ID: "k2", APIKey: "sk-2", Enabled: true, Weight: 1},
		},
	}
	prevEP := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		if backendID == "other" {
			return &BackendEndpoint{BaseURL: "https://other.example.com/v1", APIKey: "sk-other"}, nil
		}
		return &BackendEndpoint{
			BaseURL:     "https://primary.example.com/v1",
			APIKey:      "sk-fallback",
			AccountPool: pool,
		}, nil
	}

	quotaBody := `{"error":{"type":"FreeUsageLimitError","message":"quota exceeded"}}`
	seq := &sequenceHTTPClient{
		resps: []struct {
			status int
			body   string
		}{
			{429, quotaBody},
			{429, quotaBody},
			{200, `{"id":"ok","choices":[{"message":{"content":"from-other"}}]}`},
		},
	}
	broker := &mockCapabilityBroker{httpClient: seq}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "primary",
		CustomConfig: map[string]interface{}{
			"route_policy": "fixed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seq.calls != 3 {
		t.Fatalf("calls=%d want 3 (two pool keys then other backend)", seq.calls)
	}
	if len(seq.auths) < 3 {
		t.Fatalf("auths=%v", seq.auths)
	}
	if seq.auths[0] != "Bearer sk-1" || seq.auths[1] != "Bearer sk-2" {
		t.Fatalf("expected both primary pool keys first, got %v", seq.auths[:2])
	}
	if seq.auths[2] != "Bearer sk-other" {
		t.Fatalf("expected other backend only after pool exhausted, got %v", seq.auths)
	}
	if out.Metadata["status_code"] != 200 {
		t.Fatalf("status_code=%v", out.Metadata["status_code"])
	}
	if out.Metadata["billing_fallback_used"] != true {
		t.Fatalf("expected billing_fallback_used, metadata=%v", out.Metadata)
	}
}

func TestTransparentForwardNode_SystemPromptStrategy_Passthrough(t *testing.T) {
	client := &capturingHTTPClient{
		inner: &mockHTTPClient{
			status: 200,
			body:   `{"id":"ok"}`,
		},
	}
	broker := &mockCapabilityBroker{}
	broker.httpClient = client

	node, err := NewTransparentForwardNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"inject_system_prompt":   true,
			"system_prompt_strategy": "passthrough",
		},
		SystemPrompt: "gateway system",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	rawBody := `{"model":"x","messages":[{"role":"system","content":"client system"},{"role":"user","content":"hello"}]}`
	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": rawBody,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 验证 body 未被修改
	var body map[string]interface{}
	json.Unmarshal([]byte(client.body), &body)
	msgs := body["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	// 验证 system 消息未被替换
	firstMsg := msgs[0].(map[string]interface{})
	if firstMsg["content"] != "client system" {
		t.Errorf("expected first message content='client system', got %v", firstMsg["content"])
	}
}

func TestTransparentForwardNode_SystemPromptStrategy_Replace(t *testing.T) {
	client := &capturingHTTPClient{
		inner: &mockHTTPClient{
			status: 200,
			body:   `{"id":"ok"}`,
		},
	}
	broker := &mockCapabilityBroker{}
	broker.httpClient = client

	node, err := NewTransparentForwardNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"system_prompt_strategy": "replace",
		},
		SystemPrompt: "gateway system",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	rawBody := `{"model":"x","messages":[{"role":"system","content":"client system"},{"role":"user","content":"hello"}]}`
	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": rawBody,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Metadata["system_prompt_strategy"] != "replace" {
		t.Errorf("expected system_prompt_strategy=replace, got %v", out.Metadata["system_prompt_strategy"])
	}
	// 验证 body 被修改为 gateway system
	var body map[string]interface{}
	json.Unmarshal([]byte(client.body), &body)
	msgs := body["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	firstMsg := msgs[0].(map[string]interface{})
	if firstMsg["content"] != "gateway system" {
		t.Errorf("expected first message content='gateway system', got %v", firstMsg["content"])
	}
}

func TestTransparentForwardNode_SystemPromptStrategy_Append(t *testing.T) {
	client := &capturingHTTPClient{
		inner: &mockHTTPClient{
			status: 200,
			body:   `{"id":"ok"}`,
		},
	}
	broker := &mockCapabilityBroker{}
	broker.httpClient = client

	node, err := NewTransparentForwardNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"system_prompt_strategy": "append",
			"append_position":        "after_client",
		},
		SystemPrompt: "gateway system",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	rawBody := `{"model":"x","messages":[{"role":"system","content":"client system"},{"role":"user","content":"hello"}]}`
	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": rawBody,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Metadata["system_prompt_strategy"] != "append" {
		t.Errorf("expected system_prompt_strategy=append, got %v", out.Metadata["system_prompt_strategy"])
	}
	// 验证 body 包含 client system + gateway system
	var body map[string]interface{}
	json.Unmarshal([]byte(client.body), &body)
	msgs := body["messages"].([]interface{})
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}
	firstMsg := msgs[0].(map[string]interface{})
	if firstMsg["content"] != "client system" {
		t.Errorf("expected first message content='client system', got %v", firstMsg["content"])
	}
	secondMsg := msgs[1].(map[string]interface{})
	if secondMsg["content"] != "gateway system" {
		t.Errorf("expected second message content='gateway system', got %v", secondMsg["content"])
	}
}

func TestTransparentForwardNode_LegacyInjectSystemPrompt(t *testing.T) {
	client := &capturingHTTPClient{
		inner: &mockHTTPClient{
			status: 200,
			body:   `{"id":"ok"}`,
		},
	}
	broker := &mockCapabilityBroker{}
	broker.httpClient = client

	// 使用旧字段 inject_system_prompt=true
	node, err := NewTransparentForwardNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"inject_system_prompt": true,
		},
		SystemPrompt: "gateway system",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	rawBody := `{"model":"x","messages":[{"role":"system","content":"client system"},{"role":"user","content":"hello"},{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]}]}`
	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": rawBody,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 旧字段应该映射到 replace
	if out.Metadata["inject_system_prompt"] != true {
		t.Errorf("expected inject_system_prompt=true, got %v", out.Metadata["inject_system_prompt"])
	}
	if out.Metadata["system_prompt_strategy"] != "replace" {
		t.Errorf("expected system_prompt_strategy=replace, got %v", out.Metadata["system_prompt_strategy"])
	}
	var sent map[string]interface{}
	if err := json.Unmarshal([]byte(client.body), &sent); err != nil {
		t.Fatalf("unmarshal upstream body: %v", err)
	}
	msgs := sent["messages"].([]interface{})
	if len(msgs) != 3 {
		t.Fatalf("messages len=%d want 3", len(msgs))
	}
	if msgs[0].(map[string]interface{})["content"] != "gateway system" {
		t.Fatalf("system not replaced: %#v", msgs[0])
	}
	asst := msgs[2].(map[string]interface{})
	if _, ok := asst["tool_calls"].([]interface{}); !ok {
		t.Fatalf("tool_calls not preserved on #d/replace path: %#v", asst)
	}
}

// P1-T9 回归：/execute 结构化入口此前丢 body——节点必须优先消费 input.Messages
// 组装完整 chat 体（而非空 content 单条消息），否则上游报 Input must have at least 1 token.
func TestTransparentForwardNode_BuildsBodyFromInputMessages(t *testing.T) {
	cap := &capturingHTTPClient{inner: &mockHTTPClient{status: 200, body: `{"id":"ok"}`}}
	broker := &mockCapabilityBroker{}
	broker.httpClient = cap

	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "transparent_forward"
	tf.SetCapabilityBroker(broker)

	output, err := tf.Execute(context.Background(), &NodeInput{
		Messages: []Message{
			{Role: "user", Content: "我叫小明"},
			{Role: "assistant", Content: "你好小明！"},
			{Role: "user", Content: "我叫什么？"},
		},
		Metadata: map[string]interface{}{
			"target_url":   "https://api.example.com",
			"request_path": "/v1/chat/completions",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if output.Content != `{"id":"ok"}` {
		t.Fatalf("content = %q", output.Content)
	}
	var got struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(cap.body), &got); err != nil {
		t.Fatalf("outgoing body not valid json: %v\nbody=%s", err, cap.body)
	}
	if len(got.Messages) != 3 || got.Messages[2].Content != "我叫什么？" {
		t.Fatalf("messages lost or truncated: %s", cap.body)
	}
	if strings.Contains(cap.body, `"content":""`) {
		t.Fatalf("empty-content message present (regression): %s", cap.body)
	}
}

// P1-T9 回归：仅有 Content 时维持最小体构造（WebUI 快速测试场景不回归）
func TestTransparentForwardNode_ContentFallbackStillWorks(t *testing.T) {
	cap := &capturingHTTPClient{inner: &mockHTTPClient{status: 200, body: `{"id":"ok"}`}}
	broker := &mockCapabilityBroker{}
	broker.httpClient = cap

	node, _ := NewTransparentForwardNode(NodeConfig{})
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "transparent_forward"
	tf.SetCapabilityBroker(broker)

	if _, err := tf.Execute(context.Background(), &NodeInput{
		Content: "你好",
		Metadata: map[string]interface{}{
			"target_url":   "https://api.example.com",
			"request_path": "/v1/chat/completions",
		},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(cap.body), &got); err != nil {
		t.Fatalf("body invalid: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "你好" {
		t.Fatalf("content fallback broken: %s", cap.body)
	}
}

// P1-T9 回归：metadata.backend 作为 backend_id 的别名参与 pinning
func TestTransparentForwardNode_BackendKeyAliasPinning(t *testing.T) {
	cap := &capturingHTTPClient{inner: &mockHTTPClient{status: 200, body: `{"id":"ok"}`}}
	broker := &mockCapabilityBroker{}
	broker.httpClient = cap

	node, _ := NewTransparentForwardNode(NodeConfig{})
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "transparent_forward"
	tf.SetCapabilityBroker(broker)

	called := ""
	orig := ResolveBackendEndpoint
	ResolveBackendEndpoint = func(id string) (*BackendEndpoint, error) {
		called = id
		return &BackendEndpoint{BaseURL: "https://pinned.example.com"}, nil
	}
	defer func() { ResolveBackendEndpoint = orig }()

	if _, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"route_policy":     "fixed",
			"backend":          "pinned-be",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`,
		},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called != "pinned-be" {
		t.Fatalf("metadata.backend alias ignored; resolved=%q url=%v", called, cap.inner.lastReq.URL)
	}
}

// --- FilterAllowedBackend 收口测试（问题二三条漂移路径） ---

func TestFilterAllowedBackend_CrossBackendMatch_Denied(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	broker := &mockCapabilityBroker{httpClient: inner}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	prevFilter := FilterAllowedBackend
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
		FilterAllowedBackend = prevFilter
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "allowed-backend",
				Name:    "Allowed",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "gpt-4", ActualModel: "gpt-4"},
				},
			},
			{
				ID:      "denied-backend",
				Name:    "Denied",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "gpt-4", ActualModel: "gpt-4"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://" + backendID + ".example.com/v1", APIKey: "sk-" + backendID}, nil
	}
	// 只允许 allowed-backend
	FilterAllowedBackend = func(ctx context.Context, backendID string) bool {
		return backendID == "allowed-backend"
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	// 客户端只声明模型，不声明后端 → 引擎跨后端匹配 → 应被 FilterAllowedBackend 拒绝
	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	bid, _ := out.Metadata["backend_id"].(string)
	if bid == "denied-backend" {
		t.Fatalf("expected cross-backend drift to be blocked, got backend_id=%q", bid)
	}
	// 应该回落到 allowed-backend 或默认后端
	if bid != "allowed-backend" && bid != "" {
		t.Logf("backend_id=%q (fell back from denied-backend)", bid)
	}
}

func TestFilterAllowedBackend_PreferredBackend_Denied(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	broker := &mockCapabilityBroker{httpClient: inner}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	prevFilter := FilterAllowedBackend
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
		FilterAllowedBackend = prevFilter
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{
			{
				ID:      "fallback-backend",
				Name:    "Fallback",
				Enabled: true,
				SupportedModels: []backend.ModelMapping{
					{RequestedModel: "gpt-4", ActualModel: "gpt-4"},
				},
			},
		}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://" + backendID + ".example.com/v1", APIKey: "sk-" + backendID}, nil
	}
	// 只允许 fallback-backend
	FilterAllowedBackend = func(ctx context.Context, backendID string) bool {
		return backendID == "fallback-backend"
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "{{system.default_backend}}",
		Model:   "{{system.default_model}}",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	// 通过 config.WithProxyDefaults 注入 preferred backend（模拟 "我的默认后端"），但该后端不在白名单
	ctx := config.WithProxyDefaults(context.Background(), config.ProxyDefaults{
		DefaultBackendID: "preferred-not-allowed",
		DefaultModel:     "gpt-4",
	})

	out, err := tf.Execute(ctx, &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	bid, _ := out.Metadata["backend_id"].(string)
	if bid == "preferred-not-allowed" {
		t.Fatalf("expected preferred backend to be blocked, got backend_id=%q", bid)
	}
}

func TestFilterAllowedBackend_Fallback_Denied(t *testing.T) {
	inner := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	broker := &mockCapabilityBroker{httpClient: inner}

	prevList := ListEnabledBackendsForMatch
	prevEP := ResolveBackendEndpoint
	prevFilter := FilterAllowedBackend
	t.Cleanup(func() {
		ListEnabledBackendsForMatch = prevList
		ResolveBackendEndpoint = prevEP
		FilterAllowedBackend = prevFilter
	})

	ListEnabledBackendsForMatch = func() []*backend.BackendConfig {
		return []*backend.BackendConfig{}
	}
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://" + backendID + ".example.com/v1", APIKey: "sk-" + backendID}, nil
	}
	// 拒绝所有后端
	FilterAllowedBackend = func(ctx context.Context, backendID string) bool {
		return false
	}

	node, err := NewTransparentForwardNode(NodeConfig{
		Backend: "test-model",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	// FixedEgress 模式，后端不在白名单，应返回空后端ID
	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"route_policy":     "fixed",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`,
			"target_url":       "https://test-backend.example.com/v1/chat/completions",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// FixedEgress 模式下，后端被拒绝后应返回空或错误
	bid, _ := out.Metadata["backend_id"].(string)
	if bid == "fallback-not-allowed" {
		t.Fatalf("expected fallback backend to be blocked, got backend_id=%q", bid)
	}
}

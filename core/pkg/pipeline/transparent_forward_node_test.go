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

func TestBillingFallbackCandidates_IgnoresResolvedFreeWhenBodyStillPaid(t *testing.T) {
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

	// 模拟：resolvedModel 已是 free，但 body 仍是付费 —— free 不得进入 failed 集合
	failed := map[string]bool{"gpt-5.6-luna": true}
	cands := billingFallbackCandidates("zen", failed)
	found := false
	for _, c := range cands {
		if c.model == "deepseek-v4-flash-free" && c.backendID == "zen" {
			found = true
		}
		if strings.Contains(c.model, "{{") || strings.Contains(c.backendID, "{{") {
			t.Fatalf("placeholder leaked into candidates: %#v", c)
		}
	}
	if !found {
		t.Fatalf("expected free-tier candidate, got %#v", cands)
	}
}

func TestTransparentForwardNode_BillingFallbackPicksFreeTierWhenUnset(t *testing.T) {
	prevCfg := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "zen",
			DefaultModel:     "gpt-5.4-nano",
			// FallbackModel 未配置：应从 SupportedModels 挑选 *-free
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
			{200, `{"id":"ok"}`},
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

	out, err := tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"gpt-5.4-nano","messages":[{"role":"user","content":"hi"}]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(seq.bodies[1], "mimo-v2.5-free") {
		t.Fatalf("expected free-tier rewrite, body=%s", seq.bodies[1])
	}
	if out.Metadata["billing_fallback_used"] != true {
		t.Fatal("expected billing_fallback_used")
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

package pipeline

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"centag/core/pkg/backend"
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

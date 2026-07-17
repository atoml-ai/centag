package pipeline

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
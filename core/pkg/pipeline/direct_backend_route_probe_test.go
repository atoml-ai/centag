package pipeline

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// 回归：直连节点钉死 opencode-zen 时，出站 URL 必须是 Zen，不能落到系统默认 bigmodel。
func TestFixedEgress_UsesNodeBackendURLNotSystemDefault(t *testing.T) {
	prev := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prev })

	var gotBackendID string
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		gotBackendID = backendID
		switch backendID {
		case "opencode-zen":
			return &BackendEndpoint{BaseURL: "https://opencode.ai/zen/v1", APIKey: "zen-key"}, nil
		case "bigmodel-ai":
			return &BackendEndpoint{BaseURL: "https://open.bigmodel.cn/api/paas/v4", APIKey: "bm-key"}, nil
		default:
			t.Fatalf("unexpected backend id %q", backendID)
			return nil, nil
		}
	}

	raw := []byte(`{
		"id":"forward","type":"transparent_forward",
		"backend":"opencode-zen","model":"deepseek-v4-flash-free",
		"config":{
			"backend":"opencode-zen","model":"deepseek-v4-flash-free",
			"custom_config":{"route_policy":"fixed","fixed_egress":true,"inject_system_prompt":true}
		}
	}`)
	var pnc PipelineNodeConfig
	if err := json.Unmarshal(raw, &pnc); err != nil {
		t.Fatal(err)
	}
	pnc.Normalize()

	client := &mockHTTPClient{status: 200, body: `{"choices":[{"message":{"content":"ok"}}]}`}
	capturing := &capturingHTTPClient{inner: client}
	broker := &mockCapabilityBroker{httpClient: capturing}

	node, err := NewTransparentForwardNode(pnc.Config)
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	if !tf.FixedEgress {
		t.Fatal("expected FixedEgress")
	}
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)

	if _, err := tf.Execute(context.Background(), &NodeInput{Content: "ping"}); err != nil {
		t.Fatal(err)
	}
	if gotBackendID != "opencode-zen" {
		t.Fatalf("ResolveBackendEndpoint id=%q want opencode-zen", gotBackendID)
	}
	if client.lastReq == nil {
		t.Fatal("no request")
	}
	gotURL := client.lastReq.URL.String()
	if gotURL != "https://opencode.ai/zen/v1/chat/completions" {
		t.Fatalf("target URL=%q", gotURL)
	}
	if auth := client.lastReq.Header.Get("Authorization"); auth != "Bearer zen-key" {
		t.Fatalf("auth=%q", auth)
	}
	body := capturing.body
	if body == "" && client.lastReq.Body != nil {
		b, _ := io.ReadAll(client.lastReq.Body)
		body = string(b)
	}
	if !strings.Contains(body, "deepseek-v4-flash-free") {
		t.Fatalf("body model missing: %s", body)
	}
}

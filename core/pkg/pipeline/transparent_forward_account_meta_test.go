package pipeline

import (
	"context"
	"testing"

	"centag/core/pkg/backend"
)

// TestTransparentForwardNode_AccountIDInOutputMeta 验证 047：
// 账户池轮换成功后，output.Metadata["account_id"] 携带最终成功出站的账户 ID，
// 供计量层（token_usage.account_id）按 Key 分开记账。
func TestTransparentForwardNode_AccountIDInOutputMeta(t *testing.T) {
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

	node, err := NewTransparentForwardNode(NodeConfig{Backend: "meta-account-047"})
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
		t.Fatalf("calls=%d want 2", seq.calls)
	}
	if got, _ := out.Metadata["account_id"].(string); got != "key-good" {
		t.Fatalf("account_id=%q want key-good (last successful outbound)", got)
	}

	// 一次性成功（无池）时 account_id 为空串（不落表）
	solo := &mockHTTPClient{status: 200, body: `{"id":"ok"}`}
	broker2 := &mockCapabilityBroker{httpClient: solo}
	node2, _ := NewTransparentForwardNode(NodeConfig{Backend: "no-pool-047"})
	tf2 := node2.(*TransparentForwardNode)
	tf2.BaseNode.id = "forward2"
	tf2.SetCapabilityBroker(broker2)
	prevEP2 := ResolveBackendEndpoint
	t.Cleanup(func() { ResolveBackendEndpoint = prevEP2 })
	ResolveBackendEndpoint = func(backendID string) (*BackendEndpoint, error) {
		return &BackendEndpoint{BaseURL: "https://api.example.com/v1", APIKey: "sk-alone"}, nil
	}
	out2, err := tf2.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"m","messages":[]}`,
		},
	})
	if err != nil {
		t.Fatalf("Execute(2): %v", err)
	}
	if got, _ := out2.Metadata["account_id"].(string); got != "" {
		t.Fatalf("account_id=%q want empty without pool", got)
	}
}

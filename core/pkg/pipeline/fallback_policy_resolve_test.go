package pipeline

import (
	"context"
	"testing"

	"centag/core/pkg/config"
)

func TestResolveFallbackRuleTarget_RequestedModelPlaceholder(t *testing.T) {
	input := &NodeInput{
		Metadata: map[string]interface{}{
			"raw_request_body": `{"model":"gpt-5.4-nano","messages":[]}`,
		},
	}
	backend, model := resolveFallbackRuleTarget(config.FallbackRule{
		BackendID: "opencode-zen",
		Model:     "{{requested_model}}",
	}, input, "opencode-zen", "gpt-5.6-luna")
	if backend != "opencode-zen" {
		t.Fatalf("backend=%q", backend)
	}
	if model != "gpt-5.4-nano" {
		t.Fatalf("model=%q want gpt-5.4-nano (must not keep placeholder)", model)
	}
}

func TestIsUsableFallbackNodeOutput_RejectsModelError(t *testing.T) {
	out := &NodeOutput{
		Content:  `{"type":"error","error":{"type":"ModelError","message":"Model {{requested_model}} is not supported"}}`,
		Metadata: map[string]interface{}{"status_code": 401},
	}
	if isUsableFallbackNodeOutput(out) {
		t.Fatal("ModelError output must be rejected")
	}
}

func TestTransparentForwardNode_ModelErrorReturnsError(t *testing.T) {
	client := &mockHTTPClient{
		status: 401,
		body:   `{"type":"error","error":{"type":"ModelError","message":"Model x is not supported"}}`,
	}
	broker := &mockCapabilityBroker{httpClient: client}
	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	tf := node.(*TransparentForwardNode)
	tf.SetCapabilityBroker(broker)

	_, err = tf.Execute(context.Background(), &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"x","messages":[]}`,
		},
	})
	if err == nil {
		t.Fatal("ModelError should return error for fallback chaining")
	}
}

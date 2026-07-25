package pipeline

import (
	"context"
	"testing"

	"centag/core/pkg/pipeline/promptstrategy"
)

// capturingLLMClient 记录发给后端的 messages，便于断言 system 策略。
type capturingLLMClient struct {
	lastReq *LLMRequest
	resp    string
}

func (c *capturingLLMClient) Chat(_ context.Context, req *LLMRequest) (*LLMResponse, error) {
	c.lastReq = req
	content := c.resp
	if content == "" {
		content = "ok"
	}
	return &LLMResponse{Model: req.Model, Content: content, TokenUsage: 1}, nil
}

func TestGeneratorNode_SystemPromptStrategy_ExplicitPassthrough(t *testing.T) {
	node, err := NewGeneratorNode(NodeConfig{
		Backend:      "b1",
		Model:        "m1",
		SystemPrompt: "gateway persona",
		CustomConfig: map[string]interface{}{
			"system_prompt_strategy": "passthrough",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	gen := node.(*GeneratorNode)
	if gen.SystemPromptStrategy != promptstrategy.SystemModePassthrough {
		t.Fatalf("strategy=%q, want passthrough", gen.SystemPromptStrategy)
	}

	cap := &capturingLLMClient{}
	gen.SetCapabilityBroker(&mockCapabilityBroker{llmClient: cap})

	_, err = gen.Execute(context.Background(), &NodeInput{
		Messages: []Message{
			{Role: "system", Content: "client system"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cap.lastReq == nil {
		t.Fatal("expected LLM request")
	}
	if len(cap.lastReq.Messages) < 2 {
		t.Fatalf("messages=%v", cap.lastReq.Messages)
	}
	if cap.lastReq.Messages[0].Role != "system" || cap.lastReq.Messages[0].Content != "client system" {
		t.Fatalf("explicit passthrough must keep client system, got %#v", cap.lastReq.Messages[0])
	}
	for _, m := range cap.lastReq.Messages {
		if m.Content == "gateway persona" {
			t.Fatal("gateway persona must not be injected under explicit passthrough")
		}
	}
}

func TestGeneratorNode_SystemPromptStrategy_DefaultReplaceWhenUnset(t *testing.T) {
	node, err := NewGeneratorNode(NodeConfig{
		Backend:      "b1",
		Model:        "m1",
		SystemPrompt: "gateway persona",
		// 无 system_prompt_strategy：兼容旧行为 → replace
	})
	if err != nil {
		t.Fatal(err)
	}
	gen := node.(*GeneratorNode)
	cap := &capturingLLMClient{}
	gen.SetCapabilityBroker(&mockCapabilityBroker{llmClient: cap})

	_, err = gen.Execute(context.Background(), &NodeInput{
		Messages: []Message{
			{Role: "system", Content: "client system"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cap.lastReq == nil || len(cap.lastReq.Messages) < 2 {
		t.Fatalf("bad request: %#v", cap.lastReq)
	}
	if cap.lastReq.Messages[0].Content != "gateway persona" {
		t.Fatalf("default replace want gateway persona, got %#v", cap.lastReq.Messages[0])
	}
	for _, m := range cap.lastReq.Messages {
		if m.Content == "client system" {
			t.Fatal("client system should be replaced")
		}
	}
}

func TestGeneratorNode_SystemPromptStrategy_Append(t *testing.T) {
	node, err := NewGeneratorNode(NodeConfig{
		Backend:      "b1",
		Model:        "m1",
		SystemPrompt: "answer in Russian",
		CustomConfig: map[string]interface{}{
			"system_prompt_strategy": "append",
			"append_position":        "after_client",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	gen := node.(*GeneratorNode)
	cap := &capturingLLMClient{}
	gen.SetCapabilityBroker(&mockCapabilityBroker{llmClient: cap})

	_, err = gen.Execute(context.Background(), &NodeInput{
		Messages: []Message{
			{Role: "system", Content: "you are a coder"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs := cap.lastReq.Messages
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages (2 system + user), got %d: %#v", len(msgs), msgs)
	}
	if msgs[0].Content != "you are a coder" || msgs[1].Content != "answer in Russian" {
		t.Fatalf("append order wrong: %#v", msgs)
	}
}

func TestPromptOpsNodes_Registered(t *testing.T) {
	registry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(registry); err != nil {
		t.Fatal(err)
	}
	for _, nt := range []NodeType{NodeTypeUserPromptOps, NodeTypeOutputPostOps} {
		if !registry.IsRegistered(nt) {
			t.Errorf("%s not registered", nt)
		}
		n, err := registry.Create(nt, NodeConfig{})
		if err != nil {
			t.Errorf("Create(%s): %v", nt, err)
			continue
		}
		if n.Type() != nt {
			t.Errorf("Type()=%v, want %v", n.Type(), nt)
		}
	}
	if got := KindForBuiltinType(NodeTypeUserPromptOps); got != "prompt.ops" {
		t.Errorf("kind=%q", got)
	}
	if got := KindForBuiltinType(NodeTypeOutputPostOps); got != "prompt.postprocess" {
		t.Errorf("kind=%q", got)
	}
}

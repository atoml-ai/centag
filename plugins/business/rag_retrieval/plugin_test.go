package rag_retrieval

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"centag/core/pkg/pipeline"
)

func TestPlugin_Execute_DefaultNoSilentMock(t *testing.T) {
	t.Setenv("CENTAG_RAG_ALLOW_MOCK", "")
	_ = os.Unsetenv("CENTAG_RAG_ALLOW_MOCK")

	p := &Plugin{}
	resp, err := p.Execute(context.Background(), &pipeline.NodeExecutionRequest{
		Input:  &pipeline.NodeInput{Content: "Go concurrency"},
		Config: pipeline.NodeConfig{CustomConfig: map[string]interface{}{"top_k": float64(3), "threshold": 0.7}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Output.Content), &body); err != nil {
		t.Fatal(err)
	}
	if body["mock"] == true {
		t.Fatal("production default must not set mock=true")
	}
	if n, _ := body["count"].(float64); n != 0 {
		t.Fatalf("empty retrieval expected, count=%v docs=%v", body["count"], body["documents"])
	}
}

func TestPlugin_Execute_AllowMockOptIn(t *testing.T) {
	p := &Plugin{}
	resp, err := p.Execute(context.Background(), &pipeline.NodeExecutionRequest{
		Input: &pipeline.NodeInput{Content: "goroutines"},
		Config: pipeline.NodeConfig{CustomConfig: map[string]interface{}{
			"allow_mock": true,
			"top_k":      float64(5),
			"threshold":  0.5,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Output.Content), &body); err != nil {
		t.Fatal(err)
	}
	if n, _ := body["count"].(float64); n < 1 {
		t.Fatalf("mock should return docs, got %v", body)
	}
	if body["mock"] != true {
		t.Fatalf("mock flag want true, got %v", body["mock"])
	}
}

func TestPlugin_Execute_InvalidInput(t *testing.T) {
	p := &Plugin{}
	if _, err := p.Execute(context.Background(), nil); err == nil {
		t.Fatal("nil request should error")
	}
	if _, err := p.Execute(context.Background(), &pipeline.NodeExecutionRequest{}); err == nil {
		t.Fatal("nil input should error")
	}
}

func TestPlugin_Descriptor(t *testing.T) {
	p := &Plugin{}
	d := p.Descriptor()
	if d.Implementation != "business.rag_retrieval" {
		t.Fatalf("Implementation=%q", d.Implementation)
	}
	if p.GetBusinessType() != "rag_retrieval" {
		t.Fatal(p.GetBusinessType())
	}
	if err := p.ValidateConfig(pipeline.NodeConfig{}); err != nil {
		t.Fatal(err)
	}
}

package pipeline

import (
	"context"
	"testing"
)

func TestBoolFromInterface(t *testing.T) {
	tests := []struct {
		in   interface{}
		want bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"disable", false},
		{"unknown", true},
	}
	for _, tt := range tests {
		if got := BoolFromInterface(tt.in, true); got != tt.want {
			t.Errorf("BoolFromInterface(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestInjectCacheControlFromMetadata(t *testing.T) {
	execCtx := NewExecutionContext(nil)
	InjectCacheControlFromMetadata(execCtx, map[string]interface{}{
		"cache_read":  false,
		"cache_write": true,
	})
	if BoolFromExecCtx(execCtx, "cache_read", true) {
		t.Fatal("expected cache_read=false")
	}
	if !BoolFromExecCtx(execCtx, "cache_write", false) {
		t.Fatal("expected cache_write=true")
	}
}

func TestCacheNodeSkipsReadWhenDisabled(t *testing.T) {
	node, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation": "read",
			"strategy":  "exact",
		},
	})
	if err != nil {
		t.Fatalf("NewCacheNode: %v", err)
	}

	execCtx := NewExecutionContext(nil)
	execCtx.SetVariable("cache_read", false)
	ctx := context.WithValue(context.Background(), executionContextKey{}, execCtx)

	out, err := node.Execute(ctx, &NodeInput{Content: "question"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Metadata["cache_hit"] != false {
		t.Fatalf("cache_hit = %v, want false", out.Metadata["cache_hit"])
	}
	if out.Metadata["cache_read_skipped"] != true {
		t.Fatalf("expected cache_read_skipped metadata")
	}
}

func TestCacheNodeSkipsWriteWhenDisabled(t *testing.T) {
	node, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation": "write",
			"strategy":  "exact",
		},
	})
	if err != nil {
		t.Fatalf("NewCacheNode: %v", err)
	}

	execCtx := NewExecutionContext(nil)
	execCtx.SetVariable("cache_write", false)
	ctx := context.WithValue(context.Background(), executionContextKey{}, execCtx)

	out, err := node.Execute(ctx, &NodeInput{Content: "answer"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Metadata["write_success"] != false {
		t.Fatalf("write_success = %v, want false", out.Metadata["write_success"])
	}
	if out.Metadata["cache_write_skipped"] != true {
		t.Fatalf("expected cache_write_skipped metadata")
	}
}
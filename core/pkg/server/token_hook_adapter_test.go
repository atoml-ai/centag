package server

import (
	"context"
	"sync/atomic"
	"testing"

	"centag/core/pkg/hooks"
)

type countingRecordSvc struct {
	n atomic.Int32
}

// We only need to verify TriggerTokenUsedHooks invokes a single TokenHook once.
func TestWireTokenUsage_SingleHookInvocation(t *testing.T) {
	hm := hooks.NewManager()
	var n atomic.Int32
	hm.RegisterTokenHook(&countingTokenHook{n: &n})

	usage := &hooks.TokenUsage{
		UserID:       1,
		Model:        "m",
		Backend:      "b",
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
		Success:      true,
	}
	if err := hm.TriggerTokenUsedHooks(context.Background(), usage); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if n.Load() != 1 {
		t.Fatalf("expected exactly 1 token hook call, got %d", n.Load())
	}

	// Second path must not double-register: simulate pipeline persist calling trigger only.
	if err := hm.TriggerTokenUsedHooks(context.Background(), usage); err != nil {
		t.Fatalf("trigger2: %v", err)
	}
	if n.Load() != 2 {
		t.Fatalf("expected 2 calls across 2 requests, got %d", n.Load())
	}
}

type countingTokenHook struct {
	n *atomic.Int32
}

func (h *countingTokenHook) OnTokenUsed(ctx context.Context, usage *hooks.TokenUsage) error {
	h.n.Add(1)
	return nil
}

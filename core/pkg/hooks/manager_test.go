package hooks

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"centag/core/pkg/types"
)

type orderedStorageHook struct {
	id    int
	order *[]int
	fail  bool
}

func (h *orderedStorageHook) OnRequest(ctx context.Context, req *types.UnifiedRequest) error {
	*h.order = append(*h.order, h.id)
	if h.fail {
		return errors.New("request hook failed")
	}
	return nil
}

func (h *orderedStorageHook) OnResponse(ctx context.Context, resp *types.UnifiedResponse) error {
	return nil
}

func (h *orderedStorageHook) OnCacheHit(ctx context.Context, key string, data []byte) error {
	return nil
}

type countingTokenHook struct {
	n    *atomic.Int32
	fail bool
}

func (h *countingTokenHook) OnTokenUsed(ctx context.Context, usage *TokenUsage) error {
	h.n.Add(1)
	if h.fail {
		return errors.New("token hook failed")
	}
	return nil
}

type countingBillingHook struct {
	usageN  *atomic.Int32
	quotaN  *atomic.Int32
	fail    bool
}

func (h *countingBillingHook) OnUsage(ctx context.Context, usage *TokenUsage) error {
	h.usageN.Add(1)
	if h.fail {
		return errors.New("billing hook failed")
	}
	return nil
}

func (h *countingBillingHook) OnQuotaExceeded(ctx context.Context, userID int64) error {
	h.quotaN.Add(1)
	return nil
}

func TestDefaultHookManager_RegisterOrder(t *testing.T) {
	m := NewManager()
	var order []int
	m.RegisterStorageHook(&orderedStorageHook{id: 1, order: &order})
	m.RegisterStorageHook(&orderedStorageHook{id: 2, order: &order})
	m.RegisterStorageHook(&orderedStorageHook{id: 3, order: &order})

	if err := m.TriggerRequestHooks(context.Background(), &types.UnifiedRequest{Model: "m"}); err != nil {
		t.Fatalf("TriggerRequestHooks returned error: %v", err)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected order [1 2 3], got %v", order)
	}
}

func TestDefaultHookManager_FailOpen(t *testing.T) {
	m := NewManager()
	var order []int
	m.RegisterStorageHook(&orderedStorageHook{id: 1, order: &order, fail: true})
	m.RegisterStorageHook(&orderedStorageHook{id: 2, order: &order})

	err := m.TriggerRequestHooks(context.Background(), &types.UnifiedRequest{})
	if err != nil {
		t.Fatalf("fail-open must return nil, got %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected second hook to still run, order=%v", order)
	}

	var tokenN, usageN, quotaN atomic.Int32
	m.RegisterTokenHook(&countingTokenHook{n: &tokenN, fail: true})
	m.RegisterBillingHook(&countingBillingHook{usageN: &usageN, quotaN: &quotaN, fail: true})

	if err := m.TriggerTokenUsedHooks(context.Background(), &TokenUsage{TotalTokens: 1}); err != nil {
		t.Fatalf("TriggerTokenUsedHooks fail-open: %v", err)
	}
	if tokenN.Load() != 1 || usageN.Load() != 1 {
		t.Fatalf("token/billing hooks should run once, got token=%d usage=%d", tokenN.Load(), usageN.Load())
	}

	if err := m.TriggerQuotaExceededHooks(context.Background(), 42); err != nil {
		t.Fatalf("TriggerQuotaExceededHooks: %v", err)
	}
	if quotaN.Load() != 1 {
		t.Fatalf("expected quota hook once, got %d", quotaN.Load())
	}
}

func TestDefault_SetAndGet(t *testing.T) {
	m := NewManager()
	SetDefault(m)
	defer SetDefault(nil)
	if Default() != m {
		t.Fatal("Default() should return SetDefault manager")
	}
}

func TestDefaultHookManager_Counts(t *testing.T) {
	m := NewManager()
	m.RegisterStorageHook(&orderedStorageHook{order: &[]int{}})
	m.RegisterTokenHook(&countingTokenHook{n: &atomic.Int32{}})
	m.RegisterBillingHook(&countingBillingHook{usageN: &atomic.Int32{}, quotaN: &atomic.Int32{}})
	s, b, tok, l := m.Counts()
	if s != 1 || b != 1 || tok != 1 || l != 0 {
		t.Fatalf("unexpected counts storage=%d billing=%d token=%d logging=%d", s, b, tok, l)
	}
}

type countingLoggingHook struct {
	n *atomic.Int32
}

func (h *countingLoggingHook) OnRequestLog(ctx context.Context, entry *RequestLog) error {
	h.n.Add(1)
	return nil
}

func TestDefaultHookManager_ResponseCacheAndLogging(t *testing.T) {
	m := NewManager()
	var order []int
	m.RegisterStorageHook(&orderedStorageHook{id: 1, order: &order})
	if err := m.TriggerResponseHooks(context.Background(), &types.UnifiedResponse{}); err != nil {
		t.Fatal(err)
	}
	if err := m.TriggerCacheHitHooks(context.Background(), "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	// OnResponse / OnCacheHit do not append to order (only OnRequest does).
	if len(order) != 0 {
		t.Fatalf("expected no OnRequest side effects, order=%v", order)
	}
	if err := m.TriggerRequestHooks(context.Background(), &types.UnifiedRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 {
		t.Fatalf("expected OnRequest once, order=%v", order)
	}

	var logN atomic.Int32
	m.RegisterLoggingHook(&countingLoggingHook{n: &logN})
	if err := m.TriggerLoggingHooks(context.Background(), &RequestLog{Model: "m", StatusCode: 200}); err != nil {
		t.Fatal(err)
	}
	if logN.Load() != 1 {
		t.Fatalf("logging hook once, got %d", logN.Load())
	}
	_, _, _, l := m.Counts()
	if l != 1 {
		t.Fatalf("logging count=%d", l)
	}
}

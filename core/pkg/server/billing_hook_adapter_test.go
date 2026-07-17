package server

import (
	"context"
	"testing"
	"time"

	"centag/core/internal/billing"
	"centag/core/pkg/hooks"
)

func TestBillingHookAdapter_OnUsageRecordsEvent(t *testing.T) {
	svc := billing.NewService()
	defer svc.Close()
	mock := billing.NewMockHandler()
	svc.RegisterHandler(mock)

	hm := hooks.NewManager()
	hm.RegisterBillingHook(newBillingHookAdapter(svc))

	err := hm.TriggerTokenUsedHooks(context.Background(), &hooks.TokenUsage{
		UserID:      7,
		TenantID:    "t1",
		Backend:     "b1",
		Model:       "m1",
		TotalTokens: 100,
		Success:     true,
	})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(mock.GetEvents()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	events := mock.GetEvents()
	if len(events) < 1 {
		t.Fatal("expected at least one billing event")
	}
	if events[0].UserID != 7 || events[0].Tokens != 100 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestBillingHookAdapter_OnQuotaExceeded(t *testing.T) {
	hm := hooks.NewManager()
	var got int64
	hm.RegisterBillingHook(&quotaCaptureHook{onQuota: func(uid int64) { got = uid }})
	if err := hm.TriggerQuotaExceededHooks(context.Background(), 99); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if got != 99 {
		t.Fatalf("got userID %d, want 99", got)
	}
}

type quotaCaptureHook struct {
	onQuota func(int64)
}

func (h *quotaCaptureHook) OnUsage(ctx context.Context, usage *hooks.TokenUsage) error {
	return nil
}

func (h *quotaCaptureHook) OnQuotaExceeded(ctx context.Context, userID int64) error {
	h.onQuota(userID)
	return nil
}

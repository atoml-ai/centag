package billingasync

import "testing"

func TestNewService_AndRequestEvent(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService nil")
	}
	ev := NewRequestEvent(1, "t1", "openai", "gpt-4", 100)
	if ev == nil {
		t.Fatal("NewRequestEvent nil")
	}
	if ev.UserID != 1 || ev.Tokens != 100 {
		t.Fatalf("unexpected event: %+v", ev)
	}
	mock := NewMockHandler()
	if mock == nil {
		t.Fatal("NewMockHandler nil")
	}
	_ = DefaultPricingCurrency
}

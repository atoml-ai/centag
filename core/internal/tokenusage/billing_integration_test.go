package tokenusage

import (
	"context"
	"testing"

	"centag/core/internal/billing"
)

func TestRecordUsage_WithPricingService(t *testing.T) {
	ctx := context.Background()
	store := billing.NewMemoryRuleStore()
	rule := &billing.PricingRule{
		Name: "t", BackendID: "b1", Model: "m1",
		InputPricePerM: 1, OutputPricePerM: 1, Priority: 1, Enabled: true,
	}
	if err := store.CreateRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	SetPricingService(billing.NewPricingService(store))
	t.Cleanup(func() { SetPricingService(nil) })

	svc, err := NewEphemeralService()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordUsage(ctx, &UsageRecord{
		UserID: 1, BackendID: "b1", Model: "m1",
		PromptTokens: 1_000_000, CompletionTokens: 0, TotalTokens: 1_000_000, Success: true,
	}); err != nil {
		t.Fatal(err)
	}

	var cost, inCost float64
	var ruleID *int64
	err = svc.db.QueryRow(`SELECT cost_usd, input_cost, pricing_rule_id FROM token_usage WHERE user_id = 1`).
		Scan(&cost, &inCost, &ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if cost != 1 || inCost != 1 {
		t.Fatalf("cost=%v input=%v", cost, inCost)
	}
	if ruleID == nil || *ruleID != rule.ID {
		t.Fatalf("ruleID=%v want %d", ruleID, rule.ID)
	}
}

func TestEstimateCost_FallbackWhenPricingFails(t *testing.T) {
	SetPricingService(nil)
	cost := EstimateCost("ollama-local", "llama", 1000, 0)
	if cost != 0 {
		t.Fatalf("ollama should be free, got %v", cost)
	}
}

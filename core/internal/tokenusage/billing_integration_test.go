package tokenusage

import (
	"context"
	"testing"

	"centag/core/internal/billing"
)

func TestRecordUsage_WithPricingService(t *testing.T) {
	ctx := context.Background()
	store := billing.NewMemoryRuleStore()
	costRule := &billing.PricingRule{
		Name: "t-cost", BackendID: "b1", Model: "m1",
		PriceType: billing.PriceTypeCost,
		InputPricePerM: 1, OutputPricePerM: 1, Priority: 1, Enabled: true,
	}
	revRule := &billing.PricingRule{
		Name: "t-rev", BackendID: "b1", Model: "m1",
		PriceType: billing.PriceTypeRevenue,
		InputPricePerM: 3, OutputPricePerM: 3, Priority: 1, Enabled: true,
	}
	if err := store.CreateRule(ctx, costRule); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRule(ctx, revRule); err != nil {
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

	var cost, inCost, revenue float64
	var ruleID *int64
	err = svc.db.QueryRow(`SELECT cost_usd, input_cost, revenue_usd, pricing_rule_id FROM token_usage WHERE user_id = 1`).
		Scan(&cost, &inCost, &revenue, &ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if cost != 1 || inCost != 1 {
		t.Fatalf("cost=%v input=%v", cost, inCost)
	}
	if revenue != 3 {
		t.Fatalf("revenue=%v want 3", revenue)
	}
	if ruleID == nil || *ruleID != costRule.ID {
		t.Fatalf("ruleID=%v want %d", ruleID, costRule.ID)
	}
}

func TestEstimateCost_FallbackWhenPricingFails(t *testing.T) {
	SetPricingService(nil)
	cost := EstimateCost("ollama-local", "llama", 1000, 0)
	if cost != 0 {
		t.Fatalf("ollama should be free, got %v", cost)
	}
}

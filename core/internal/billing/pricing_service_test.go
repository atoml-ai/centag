package billing

import (
	"context"
	"testing"
	"time"
)

func seedStore(t *testing.T) *MemoryRuleStore {
	t.Helper()
	ctx := context.Background()
	s := NewMemoryRuleStore()
	rules := []*PricingRule{
		{Name: "wild", BackendID: "ppinfra", Model: "*", InputPricePerM: 9, OutputPricePerM: 9, Priority: 10, Enabled: true},
		{Name: "exact", BackendID: "ppinfra", Model: "deepseek-v3.2", InputPricePerM: 1, OutputPricePerM: 1, Priority: 10, Enabled: true},
		{Name: "high", BackendID: "ppinfra", Model: "deepseek-v3.2", InputPricePerM: 2, OutputPricePerM: 2, Priority: 100, Enabled: true},
		{Name: "off", BackendID: "solo-backend", Model: "solo-model", InputPricePerM: 1.5, OutputPricePerM: 1.5, Priority: 100, Enabled: false},
	}
	for _, r := range rules {
		if err := s.CreateRule(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestPricing_ExactAndWildcardAndPriority(t *testing.T) {
	ctx := context.Background()
	svc := NewPricingService(seedStore(t), WithCacheTTL(time.Minute))

	info, err := svc.ResolvePrice(ctx, "ppinfra", "deepseek-v3.2")
	if err != nil {
		t.Fatal(err)
	}
	if info.Source != PriceSourceRule || info.InputPricePerM != 2 {
		t.Fatalf("want high-priority exact rule price=2, got %+v", info)
	}

	info2, err := svc.ResolvePrice(ctx, "ppinfra", "unknown-model")
	if err != nil {
		t.Fatal(err)
	}
	if info2.InputPricePerM != 9 || info2.Source != PriceSourceRule {
		t.Fatalf("want wildcard 9, got %+v", info2)
	}

	// disabled-only backend → fall through to default (no legacy entry)
	info3, err := svc.ResolvePrice(ctx, "solo-backend", "solo-model")
	if err != nil {
		t.Fatal(err)
	}
	if info3.Source != PriceSourceDefault {
		t.Fatalf("want default for disabled rule miss, got %+v", info3)
	}
}

func TestPricing_CostFormula(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryRuleStore()
	_ = s.CreateRule(ctx, &PricingRule{
		Name: "c", BackendID: "b", Model: "m",
		InputPricePerM: 1, OutputPricePerM: 2, Priority: 1, Enabled: true, Currency: "USD",
	})
	svc := NewPricingService(s)
	bd, err := svc.EstimateCost(ctx, "b", "m", 1_000_000, 500_000)
	if err != nil {
		t.Fatal(err)
	}
	if bd.InputCost != 1 || bd.OutputCost != 1 || bd.TotalCost != 2 {
		t.Fatalf("breakdown %+v", bd)
	}
	if bd.Currency != "USD" {
		t.Fatalf("currency %s", bd.Currency)
	}
}

func TestPricingCacheInvalidation(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryRuleStore()
	rule := &PricingRule{
		Name: "c", BackendID: "b", Model: "m",
		InputPricePerM: 1, OutputPricePerM: 1, Priority: 1, Enabled: true,
	}
	_ = s.CreateRule(ctx, rule)
	svc := NewPricingService(s, WithCacheTTL(time.Hour))

	info, _ := svc.ResolvePrice(ctx, "b", "m")
	if info.InputPricePerM != 1 {
		t.Fatalf("got %+v", info)
	}
	rule.InputPricePerM = 8
	_ = s.UpdateRule(ctx, rule.ID, rule)
	// cached stale
	stale, _ := svc.ResolvePrice(ctx, "b", "m")
	if stale.InputPricePerM != 1 {
		t.Fatalf("expected stale cache 1, got %+v", stale)
	}
	_ = svc.InvalidateCache(ctx)
	fresh, _ := svc.ResolvePrice(ctx, "b", "m")
	if fresh.InputPricePerM != 8 {
		t.Fatalf("expected 8 after invalidate, got %+v", fresh)
	}
}

func TestPriceResolutionFallback(t *testing.T) {
	ctx := context.Background()
	svc := NewPricingService(NewMemoryRuleStore())
	info, err := svc.ResolvePrice(ctx, "totally-unknown", "x")
	if err != nil {
		t.Fatal(err)
	}
	if info.Source != PriceSourceDefault || info.InputPricePerM != 0.7 {
		t.Fatalf("want default 0.7 USD, got %+v", info)
	}
	info2, err := svc.ResolvePrice(ctx, "ollama-local", "any")
	if err != nil {
		t.Fatal(err)
	}
	if info2.Source != PriceSourceLegacyTable || info2.InputPricePerM != 0 {
		t.Fatalf("want legacy ollama 0, got %+v", info2)
	}
}

func TestBillingService(t *testing.T) {
	TestPricing_ExactAndWildcardAndPriority(t)
	TestPricing_CostFormula(t)
	TestPricingCacheInvalidation(t)
	TestPriceResolutionFallback(t)
}

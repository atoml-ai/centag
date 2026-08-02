package billing

import (
	"context"
	"testing"
	"time"

	"centag/core/pkg/billing"
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

func seedStoreWithType(t *testing.T) *MemoryRuleStore {
	t.Helper()
	ctx := context.Background()
	s := NewMemoryRuleStore()
	rules := []*PricingRule{
		{Name: "cost-wild", BackendID: "ppinfra", Model: "*", PriceType: billing.PriceTypeCost, InputPricePerM: 1, OutputPricePerM: 2, Priority: 10, Enabled: true},
		{Name: "cost-exact", BackendID: "ppinfra", Model: "deepseek-v3.2", PriceType: billing.PriceTypeCost, InputPricePerM: 3, OutputPricePerM: 4, Priority: 50, Enabled: true},
		{Name: "revenue-wild", BackendID: "ppinfra", Model: "*", PriceType: billing.PriceTypeRevenue, InputPricePerM: 5, OutputPricePerM: 6, Priority: 10, Enabled: true},
		{Name: "revenue-exact", BackendID: "ppinfra", Model: "deepseek-v3.2", PriceType: billing.PriceTypeRevenue, InputPricePerM: 7, OutputPricePerM: 8, Priority: 50, Enabled: true},
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

func TestPricing_ResolvePriceByType(t *testing.T) {
	ctx := context.Background()
	svc := NewPricingService(seedStoreWithType(t), WithCacheTTL(time.Minute))

	// Cost type - exact match
	info, err := svc.ResolvePriceByType(ctx, "ppinfra", "deepseek-v3.2", billing.PriceTypeCost)
	if err != nil {
		t.Fatal(err)
	}
	if info.Source != PriceSourceRule || info.InputPricePerM != 3 || info.OutputPricePerM != 4 {
		t.Fatalf("want cost exact 3/4, got %+v", info)
	}
	if info.PriceType != billing.PriceTypeCost {
		t.Fatalf("want PriceType=cost, got %s", info.PriceType)
	}

	// Revenue type - exact match
	info2, err := svc.ResolvePriceByType(ctx, "ppinfra", "deepseek-v3.2", billing.PriceTypeRevenue)
	if err != nil {
		t.Fatal(err)
	}
	if info2.InputPricePerM != 7 || info2.OutputPricePerM != 8 {
		t.Fatalf("want revenue exact 7/8, got %+v", info2)
	}

	// Cost type - wildcard match
	info3, err := svc.ResolvePriceByType(ctx, "ppinfra", "unknown-model", billing.PriceTypeCost)
	if err != nil {
		t.Fatal(err)
	}
	if info3.InputPricePerM != 1 || info3.OutputPricePerM != 2 {
		t.Fatalf("want cost wildcard 1/2, got %+v", info3)
	}

	// Unknown backend - fallback to default
	info4, err := svc.ResolvePriceByType(ctx, "unknown", "model", billing.PriceTypeCost)
	if err != nil {
		t.Fatal(err)
	}
	if info4.Source != PriceSourceDefault {
		t.Fatalf("want default, got %+v", info4)
	}
}

func TestPricing_EstimateCostByType(t *testing.T) {
	ctx := context.Background()
	svc := NewPricingService(seedStoreWithType(t))

	// Cost type
	bd, err := svc.EstimateCostByType(ctx, "ppinfra", "deepseek-v3.2", 1_000_000, 500_000, billing.PriceTypeCost)
	if err != nil {
		t.Fatal(err)
	}
	// cost-exact: input=3, output=4; tokens: 1M input = 3, 0.5M output = 2
	if bd.InputCost != 3 || bd.OutputCost != 2 || bd.TotalCost != 5 {
		t.Fatalf("want cost 3/2/5, got %+v", bd)
	}
	if bd.PriceType != billing.PriceTypeCost {
		t.Fatalf("want PriceType=cost, got %s", bd.PriceType)
	}

	// Revenue type
	bd2, err := svc.EstimateCostByType(ctx, "ppinfra", "deepseek-v3.2", 1_000_000, 500_000, billing.PriceTypeRevenue)
	if err != nil {
		t.Fatal(err)
	}
	// revenue-exact: input=7, output=8; 1M input = 7, 0.5M output = 4
	if bd2.InputCost != 7 || bd2.OutputCost != 4 || bd2.TotalCost != 11 {
		t.Fatalf("want revenue 7/4/11, got %+v", bd2)
	}

	// Negative tokens should be clamped to 0
	bd3, err := svc.EstimateCostByType(ctx, "ppinfra", "deepseek-v3.2", -100, -200, billing.PriceTypeCost)
	if err != nil {
		t.Fatal(err)
	}
	if bd3.TotalCost != 0 {
		t.Fatalf("want 0 for negative tokens, got %+v", bd3)
	}
}

func TestPricing_EstimateDualPricing(t *testing.T) {
	ctx := context.Background()
	svc := NewPricingService(seedStoreWithType(t))

	costBD, revenueBD, err := svc.EstimateDualPricing(ctx, "ppinfra", "deepseek-v3.2", 1_000_000, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	// cost: input=3, output=4 → 3+4=7
	if costBD.TotalCost != 7 {
		t.Fatalf("want cost total=7, got %+v", costBD)
	}
	if costBD.PriceType != billing.PriceTypeCost {
		t.Fatalf("want cost PriceType, got %s", costBD.PriceType)
	}
	// revenue: input=7, output=8 → 7+8=15
	if revenueBD.TotalCost != 15 {
		t.Fatalf("want revenue total=15, got %+v", revenueBD)
	}
	if revenueBD.PriceType != billing.PriceTypeRevenue {
		t.Fatalf("want revenue PriceType, got %s", revenueBD.PriceType)
	}

	// Unknown backend - revenue should fallback to cost
	costBD2, revenueBD2, err := svc.EstimateDualPricing(ctx, "unknown", "model", 1_000_000, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	// Both should be default (0.7 * 2M = 1.4)
	if costBD2.TotalCost != 1.4 {
		t.Fatalf("want fallback cost total=1.4, got %+v", costBD2)
	}
	if revenueBD2.TotalCost != 1.4 {
		t.Fatalf("want fallback revenue total=1.4, got %+v", revenueBD2)
	}
}

func TestPricing_TypeCacheIsolation(t *testing.T) {
	ctx := context.Background()
	svc := NewPricingService(seedStoreWithType(t), WithCacheTTL(time.Hour))

	// Resolve cost
	costInfo, _ := svc.ResolvePriceByType(ctx, "ppinfra", "deepseek-v3.2", billing.PriceTypeCost)
	// Resolve revenue
	revInfo, _ := svc.ResolvePriceByType(ctx, "ppinfra", "deepseek-v3.2", billing.PriceTypeRevenue)

	// They should have different prices
	if costInfo.InputPricePerM == revInfo.InputPricePerM {
		t.Fatal("cost and revenue should have different prices")
	}

	// Re-resolve cost from cache
	costInfo2, _ := svc.ResolvePriceByType(ctx, "ppinfra", "deepseek-v3.2", billing.PriceTypeCost)
	if costInfo2.InputPricePerM != costInfo.InputPricePerM {
		t.Fatal("cached cost should match")
	}
}

func TestBillingService(t *testing.T) {
	TestPricing_ExactAndWildcardAndPriority(t)
	TestPricing_CostFormula(t)
	TestPricingCacheInvalidation(t)
	TestPriceResolutionFallback(t)
}

package billing

import (
	"context"
	"sync"
	"time"

	"centag/core/pkg/billing"
	"centag/core/pkg/scheduler"
)

// PricingService resolves prices and estimates costs. Distinct from Event Service.
type PricingService interface {
	ResolvePrice(ctx context.Context, backendID, model string) (*PriceInfo, error)
	EstimateCost(ctx context.Context, backendID, model string, inputTokens, outputTokens int) (*CostBreakdown, error)
	InvalidateCache(ctx context.Context) error

	// 扩展方法（v0.3.2）- 按价格类型解析
	ResolvePriceByType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*PriceInfo, error)
	EstimateCostByType(ctx context.Context, backendID, model string, inputTokens, outputTokens int, priceType billing.PriceType) (*CostBreakdown, error)
	EstimateDualPricing(ctx context.Context, backendID, model string, inputTokens, outputTokens int) (*billing.CostBreakdown, *billing.CostBreakdown, error)
}

type cachedPrice struct {
	info      *PriceInfo
	expiresAt time.Time
}

// DefaultPricingService implements PricingService with TTL cache + legacy fallback.
type DefaultPricingService struct {
	store      RuleStore
	legacy     *scheduler.ModelPriceTable
	ttl        time.Duration
	mu         sync.RWMutex
	cache      map[string]*cachedPrice
	cacheEpoch int64
}

// PricingServiceOption configures DefaultPricingService.
type PricingServiceOption func(*DefaultPricingService)

// WithCacheTTL sets price cache TTL (default 5 minutes).
func WithCacheTTL(d time.Duration) PricingServiceOption {
	return func(s *DefaultPricingService) {
		if d > 0 {
			s.ttl = d
		}
	}
}

// WithLegacyPriceTable overrides the deprecated hardcoded fallback table.
func WithLegacyPriceTable(pt *scheduler.ModelPriceTable) PricingServiceOption {
	return func(s *DefaultPricingService) {
		if pt != nil {
			s.legacy = pt
		}
	}
}

// NewPricingService creates a PricingService backed by RuleStore.
func NewPricingService(store RuleStore, opts ...PricingServiceOption) *DefaultPricingService {
	s := &DefaultPricingService{
		store:  store,
		legacy: scheduler.NewModelPriceTable(),
		ttl:    5 * time.Minute,
		cache:  make(map[string]*cachedPrice),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func cacheKey(backendID, model string) string {
	return backendID + "\x00" + model
}

func cacheKeyWithType(backendID, model string, priceType billing.PriceType) string {
	return backendID + "\x00" + model + "\x00" + string(priceType)
}

func (s *DefaultPricingService) InvalidateCache(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*cachedPrice)
	s.cacheEpoch++
	return nil
}

func (s *DefaultPricingService) ResolvePrice(ctx context.Context, backendID, model string) (*PriceInfo, error) {
	key := cacheKey(backendID, model)
	now := time.Now()

	s.mu.RLock()
	if ent, ok := s.cache[key]; ok && now.Before(ent.expiresAt) {
		info := *ent.info
		s.mu.RUnlock()
		return &info, nil
	}
	s.mu.RUnlock()

	info, err := s.resolveUncached(ctx, backendID, model)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[key] = &cachedPrice{info: info, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()

	cp := *info
	return &cp, nil
}

func (s *DefaultPricingService) resolveUncached(ctx context.Context, backendID, model string) (*PriceInfo, error) {
	if s.store != nil {
		rules, err := s.store.ListRules(ctx)
		if err != nil {
			return nil, err
		}
		if hit := matchPricingRule(rules, backendID, model); hit != nil {
			return &PriceInfo{
				InputPricePerM:  hit.InputPricePerM,
				OutputPricePerM: hit.OutputPricePerM,
				Currency:        coalesceCurrency(hit.Currency),
				PricingRuleID:   hit.ID,
				Source:          PriceSourceRule,
			}, nil
		}
	}

	if s.legacy != nil {
		return resolveFromLegacyTable(s.legacy, backendID, model), nil
	}

	return &PriceInfo{
		InputPricePerM:  0.7,
		OutputPricePerM: 0.7,
		Currency:        DefaultPricingCurrency,
		Source:          PriceSourceDefault,
	}, nil
}

func resolveFromLegacyTable(pt *scheduler.ModelPriceTable, backendID, model string) *PriceInfo {
	lp := pt.GetPrice(backendID, model)
	source := PriceSourceDefault
	if backendPrices, ok := pt.GetAllPrices()[backendID]; ok {
		if _, ok := backendPrices[model]; ok {
			source = PriceSourceLegacyTable
		} else if _, ok := backendPrices["*"]; ok {
			source = PriceSourceLegacyTable
		}
	}
	return &PriceInfo{
		InputPricePerM:  lp.InputPrice,
		OutputPricePerM: lp.OutputPrice,
		Currency:        coalesceCurrency(lp.Currency),
		Source:          source,
	}
}

func (s *DefaultPricingService) EstimateCost(ctx context.Context, backendID, model string, inputTokens, outputTokens int) (*CostBreakdown, error) {
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	info, err := s.ResolvePrice(ctx, backendID, model)
	if err != nil {
		return nil, err
	}
	in := float64(inputTokens) / 1_000_000 * info.InputPricePerM
	out := float64(outputTokens) / 1_000_000 * info.OutputPricePerM
	return &CostBreakdown{
		InputCost:       in,
		OutputCost:      out,
		TotalCost:       in + out,
		Currency:        info.Currency,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		PricingRuleID:   info.PricingRuleID,
		Source:          info.Source,
		InputPricePerM:  info.InputPricePerM,
		OutputPricePerM: info.OutputPricePerM,
	}, nil
}

func coalesceCurrency(c string) string {
	if c == "" {
		return DefaultPricingCurrency
	}
	return c
}

// matchPricingRule selects the best enabled rule.
// Order: higher priority first; on tie, more specific match wins
// (exact model > model *; exact backend > backend *).
func matchPricingRule(rules []*PricingRule, backendID, model string) *PricingRule {
	var best *PricingRule
	bestScore := -1
	bestPriority := -1 << 30

	for _, r := range rules {
		if r == nil || !r.Enabled {
			continue
		}
		if !wildcardMatch(r.BackendID, backendID) || !wildcardMatch(r.Model, model) {
			continue
		}
		score := specificityScore(r.BackendID, r.Model)
		if r.Priority > bestPriority || (r.Priority == bestPriority && score > bestScore) {
			best = r
			bestPriority = r.Priority
			bestScore = score
		}
	}
	return best
}

func wildcardMatch(pattern, value string) bool {
	return pattern == "*" || pattern == value
}

func specificityScore(backendID, model string) int {
	score := 0
	if backendID != "*" {
		score += 2
	}
	if model != "*" {
		score += 1
	}
	return score
}

// ResolvePriceByType 按价格类型解析价格
func (s *DefaultPricingService) ResolvePriceByType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*PriceInfo, error) {
	key := cacheKeyWithType(backendID, model, priceType)
	now := time.Now()

	s.mu.RLock()
	if ent, ok := s.cache[key]; ok && now.Before(ent.expiresAt) {
		info := *ent.info
		s.mu.RUnlock()
		return &info, nil
	}
	s.mu.RUnlock()

	info, err := s.resolveUncachedByType(ctx, backendID, model, priceType)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[key] = &cachedPrice{info: info, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()

	cp := *info
	return &cp, nil
}

func (s *DefaultPricingService) resolveUncachedByType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*PriceInfo, error) {
	if s.store != nil {
		rules, err := s.store.ListRulesByType(ctx, priceType)
		if err != nil {
			return nil, err
		}
		if hit := matchPricingRule(rules, backendID, model); hit != nil {
			return &PriceInfo{
				InputPricePerM:  hit.InputPricePerM,
				OutputPricePerM: hit.OutputPricePerM,
				Currency:        coalesceCurrency(hit.Currency),
				PricingRuleID:   hit.ID,
				Source:          PriceSourceRule,
				PriceType:       priceType,
			}, nil
		}
	}

	// 没有找到指定类型的规则，降级到 cost 类型
	if priceType != billing.PriceTypeCost {
		return s.resolveUncachedByType(ctx, backendID, model, billing.PriceTypeCost)
	}

	// cost 类型也没有找到，使用 legacy 或默认值
	if s.legacy != nil {
		return resolveFromLegacyTable(s.legacy, backendID, model), nil
	}

	return &PriceInfo{
		InputPricePerM:  0.7,
		OutputPricePerM: 0.7,
		Currency:        DefaultPricingCurrency,
		Source:          PriceSourceDefault,
		PriceType:       billing.PriceTypeCost,
	}, nil
}

// EstimateCostByType 按价格类型估算成本
func (s *DefaultPricingService) EstimateCostByType(ctx context.Context, backendID, model string, inputTokens, outputTokens int, priceType billing.PriceType) (*CostBreakdown, error) {
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	info, err := s.ResolvePriceByType(ctx, backendID, model, priceType)
	if err != nil {
		return nil, err
	}
	in := float64(inputTokens) / 1_000_000 * info.InputPricePerM
	out := float64(outputTokens) / 1_000_000 * info.OutputPricePerM
	return &CostBreakdown{
		InputCost:       in,
		OutputCost:      out,
		TotalCost:       in + out,
		Currency:        info.Currency,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		PricingRuleID:   info.PricingRuleID,
		Source:          info.Source,
		PriceType:       priceType,
		InputPricePerM:  info.InputPricePerM,
		OutputPricePerM: info.OutputPricePerM,
	}, nil
}

// EstimateDualPricing 同时估算成本和营收价格
func (s *DefaultPricingService) EstimateDualPricing(ctx context.Context, backendID, model string, inputTokens, outputTokens int) (*billing.CostBreakdown, *billing.CostBreakdown, error) {
	costBreakdown, err := s.EstimateCostByType(ctx, backendID, model, inputTokens, outputTokens, billing.PriceTypeCost)
	if err != nil {
		return nil, nil, err
	}

	revenueBreakdown, err := s.EstimateCostByType(ctx, backendID, model, inputTokens, outputTokens, billing.PriceTypeRevenue)
	if err != nil {
		// revenue 降级到 cost 价格
		revenueBreakdown = costBreakdown
	}

	// 转换为 pkg/billing 的 CostBreakdown
	toPkgCost := func(cb *CostBreakdown) *billing.CostBreakdown {
		return &billing.CostBreakdown{
			InputCost:     cb.InputCost,
			OutputCost:    cb.OutputCost,
			TotalCost:     cb.TotalCost,
			Currency:      cb.Currency,
			InputTokens:   cb.InputTokens,
			OutputTokens:  cb.OutputTokens,
			PricingRuleID: cb.PricingRuleID,
			Source:        cb.Source,
			PriceType:     cb.PriceType,
		}
	}

	return toPkgCost(costBreakdown), toPkgCost(revenueBreakdown), nil
}

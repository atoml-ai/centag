package billing

import (
	"context"
	"sync"
	"time"

	"centag/core/pkg/scheduler"
)

// PriceInfo is a resolved unit price for a backend/model pair.
type PriceInfo struct {
	InputPricePerM  float64 `json:"input_price_per_m"`
	OutputPricePerM float64 `json:"output_price_per_m"`
	Currency        string  `json:"currency"`
	PricingRuleID   int64   `json:"pricing_rule_id"`
	Source          string  `json:"source"` // rule | legacy_table | default
}

// CostBreakdown is an estimated cost for a token usage event.
type CostBreakdown struct {
	InputCost     float64 `json:"input_cost"`
	OutputCost    float64 `json:"output_cost"`
	TotalCost     float64 `json:"total_cost"`
	Currency      string  `json:"currency"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	PricingRuleID int64   `json:"pricing_rule_id"`
	Source        string  `json:"source"`
}

// PricingService resolves prices and estimates costs. Distinct from Event Service.
type PricingService interface {
	ResolvePrice(ctx context.Context, backendID, model string) (*PriceInfo, error)
	EstimateCost(ctx context.Context, backendID, model string, inputTokens, outputTokens int) (*CostBreakdown, error)
	InvalidateCache(ctx context.Context) error
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
		InputCost:     in,
		OutputCost:    out,
		TotalCost:     in + out,
		Currency:      info.Currency,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		PricingRuleID: info.PricingRuleID,
		Source:        info.Source,
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

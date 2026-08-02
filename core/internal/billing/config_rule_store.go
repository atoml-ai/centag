package billing

import (
	"context"
	"fmt"
	"sync"

	"centag/core/pkg/billing"
)

// ConfigRuleStore is a read-only rule store that reads from pricing YAML config.
// Used by centag for Team mode and Personal mode to fetch pricing rules.
type ConfigRuleStore struct {
	mu        sync.RWMutex
	rules     map[int64]*billing.PricingRule
	nextID    int64
	loaded    bool
	yamlPath  string
}

// NewConfigRuleStore creates a ConfigRuleStore from a YAML file path.
func NewConfigRuleStore(yamlPath string) *ConfigRuleStore {
	return &ConfigRuleStore{
		nextID:   1,
		rules:    make(map[int64]*billing.PricingRule),
		yamlPath: yamlPath,
	}
}

// loadOnce loads rules from YAML if not already loaded.
func (s *ConfigRuleStore) loadOnce(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return nil
	}
	file, err := LoadPricingYAMLFile(s.yamlPath)
	if err != nil {
		return fmt.Errorf("failed to load pricing YAML: %w", err)
	}
	NormalizePricingFileToUSD(file)
	for i := range file.Rules {
		r := file.Rules[i]
		r.ID = s.nextID
		s.nextID++
		r.Currency = DefaultPricingCurrency
		cp := r
		s.rules[cp.ID] = &cp
	}
	s.loaded = true
	return nil
}

func (s *ConfigRuleStore) ListRules(ctx context.Context) ([]*billing.PricingRule, error) {
	if err := s.loadOnce(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*billing.PricingRule, 0, len(s.rules))
	for _, r := range s.rules {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func (s *ConfigRuleStore) GetRule(ctx context.Context, id int64) (*billing.PricingRule, error) {
	if err := s.loadOnce(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	if !ok {
		return nil, fmt.Errorf("pricing rule %d not found", id)
	}
	cp := *r
	return &cp, nil
}

func (s *ConfigRuleStore) ListRulesByType(ctx context.Context, priceType billing.PriceType) ([]*billing.PricingRule, error) {
	if err := s.loadOnce(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*billing.PricingRule, 0)
	for _, r := range s.rules {
		if r.PriceType == priceType {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *ConfigRuleStore) GetRuleByModelAndType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*billing.PricingRule, error) {
	if err := s.loadOnce(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *billing.PricingRule
	bestPriority := -1 << 30

	for _, r := range s.rules {
		if r == nil || !r.Enabled {
			continue
		}
		if r.PriceType != priceType {
			continue
		}
		if !wildcardMatch(r.BackendID, backendID) || !wildcardMatch(r.Model, model) {
			continue
		}
		if r.Priority > bestPriority {
			best = r
			bestPriority = r.Priority
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no pricing rule found for %s/%s type=%s", backendID, model, priceType)
	}

	cp := *best
	return &cp, nil
}

func (s *ConfigRuleStore) CreateRule(ctx context.Context, rule *billing.PricingRule) error {
	return fmt.Errorf("ConfigRuleStore is read-only")
}

func (s *ConfigRuleStore) UpdateRule(ctx context.Context, id int64, rule *billing.PricingRule) error {
	return fmt.Errorf("ConfigRuleStore is read-only")
}

func (s *ConfigRuleStore) DeleteRule(ctx context.Context, id int64) error {
	return fmt.Errorf("ConfigRuleStore is read-only")
}

func (s *ConfigRuleStore) ImportFromYAML(ctx context.Context, data []byte) error {
	return fmt.Errorf("ConfigRuleStore is read-only")
}

func (s *ConfigRuleStore) ExportToYAML(ctx context.Context) ([]byte, error) {
	return nil, fmt.Errorf("ConfigRuleStore is read-only")
}

func (s *ConfigRuleStore) CountRules(ctx context.Context) (int, error) {
	if err := s.loadOnce(ctx); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rules), nil
}

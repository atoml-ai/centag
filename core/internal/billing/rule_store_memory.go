package billing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryRuleStore holds pricing rules in process memory (minimal edition).
type MemoryRuleStore struct {
	mu     sync.RWMutex
	nextID int64
	rules  map[int64]*PricingRule
}

// NewMemoryRuleStore creates an empty memory rule store.
func NewMemoryRuleStore() *MemoryRuleStore {
	return &MemoryRuleStore{
		nextID: 1,
		rules:  make(map[int64]*PricingRule),
	}
}

// NewMemoryRuleStoreFromYAML loads rules from YAML bytes into memory.
func NewMemoryRuleStoreFromYAML(data []byte) (*MemoryRuleStore, error) {
	s := NewMemoryRuleStore()
	if err := s.ImportFromYAML(context.Background(), data); err != nil {
		return nil, err
	}
	return s, nil
}

// NewMemoryRuleStoreFromFile loads rules from a YAML file path.
func NewMemoryRuleStoreFromFile(path string) (*MemoryRuleStore, error) {
	file, err := LoadPricingYAMLFile(path)
	if err != nil {
		return nil, err
	}
	data, err := MarshalPricingYAML(file)
	if err != nil {
		return nil, err
	}
	return NewMemoryRuleStoreFromYAML(data)
}

func (s *MemoryRuleStore) ListRules(ctx context.Context) ([]*PricingRule, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*PricingRule, 0, len(s.rules))
	for _, r := range s.rules {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func (s *MemoryRuleStore) GetRule(ctx context.Context, id int64) (*PricingRule, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	if !ok {
		return nil, fmt.Errorf("pricing rule %d not found", id)
	}
	cp := *r
	return &cp, nil
}

func (s *MemoryRuleStore) CreateRule(ctx context.Context, rule *PricingRule) error {
	_ = ctx
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}
	if rule.BackendID == "" || rule.Model == "" {
		return fmt.Errorf("backend_id and model are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	cp := *rule
	cp.ID = s.nextID
	s.nextID++
	cp.Currency = DefaultPricingCurrency
	cp.CreatedAt = now
	cp.UpdatedAt = now
	s.rules[cp.ID] = &cp
	rule.ID = cp.ID
	rule.Currency = DefaultPricingCurrency
	rule.CreatedAt = cp.CreatedAt
	rule.UpdatedAt = cp.UpdatedAt
	return nil
}

func (s *MemoryRuleStore) UpdateRule(ctx context.Context, id int64, rule *PricingRule) error {
	_ = ctx
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.rules[id]
	if !ok {
		return fmt.Errorf("pricing rule %d not found", id)
	}
	now := time.Now().UTC()
	cp := *rule
	cp.ID = id
	cp.CreatedAt = existing.CreatedAt
	cp.UpdatedAt = now
	cp.Currency = DefaultPricingCurrency
	s.rules[id] = &cp
	rule.Currency = DefaultPricingCurrency
	return nil
}

func (s *MemoryRuleStore) DeleteRule(ctx context.Context, id int64) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rules[id]; !ok {
		return fmt.Errorf("pricing rule %d not found", id)
	}
	delete(s.rules, id)
	return nil
}

func (s *MemoryRuleStore) ImportFromYAML(ctx context.Context, data []byte) error {
	file, err := ParsePricingYAML(data)
	if err != nil {
		return err
	}
	NormalizePricingFileToUSD(file)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = make(map[int64]*PricingRule)
	s.nextID = 1
	now := time.Now().UTC()
	for i := range file.Rules {
		r := file.Rules[i]
		r.ID = s.nextID
		s.nextID++
		r.Currency = DefaultPricingCurrency
		r.CreatedAt = now
		r.UpdatedAt = now
		cp := r
		s.rules[cp.ID] = &cp
	}
	return nil
}

func (s *MemoryRuleStore) ExportToYAML(ctx context.Context) ([]byte, error) {
	rules, err := s.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	file := &PricingRulesFile{
		Version:  "1.0",
		Currency: DefaultPricingCurrency,
		USDToCNY: USDToCNY(),
		Rules:    make([]PricingRule, 0, len(rules)),
	}
	for _, r := range rules {
		cp := *r
		cp.Currency = DefaultPricingCurrency
		file.Rules = append(file.Rules, cp)
	}
	return MarshalPricingYAML(file)
}

func (s *MemoryRuleStore) CountRules(ctx context.Context) (int, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rules), nil
}

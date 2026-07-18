package billing

import "context"

// RuleStore persists pricing rules. It does not record usage.
type RuleStore interface {
	ListRules(ctx context.Context) ([]*PricingRule, error)
	GetRule(ctx context.Context, id int64) (*PricingRule, error)
	CreateRule(ctx context.Context, rule *PricingRule) error
	UpdateRule(ctx context.Context, id int64, rule *PricingRule) error
	DeleteRule(ctx context.Context, id int64) error
	ImportFromYAML(ctx context.Context, data []byte) error
	ExportToYAML(ctx context.Context) ([]byte, error)
	CountRules(ctx context.Context) (int, error)
}

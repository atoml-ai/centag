package billing

import (
	"context"

	"centag/core/pkg/billing"
)

// RuleStore persists pricing rules. It does not record usage.
type RuleStore interface {
	ListRules(ctx context.Context) ([]*billing.PricingRule, error)
	GetRule(ctx context.Context, id int64) (*billing.PricingRule, error)
	CreateRule(ctx context.Context, rule *billing.PricingRule) error
	UpdateRule(ctx context.Context, id int64, rule *billing.PricingRule) error
	DeleteRule(ctx context.Context, id int64) error
	ImportFromYAML(ctx context.Context, data []byte) error
	ExportToYAML(ctx context.Context) ([]byte, error)
	CountRules(ctx context.Context) (int, error)

	// 扩展方法（v0.3.2）
	ListRulesByType(ctx context.Context, priceType billing.PriceType) ([]*billing.PricingRule, error)
	GetRuleByModelAndType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*billing.PricingRule, error)
}

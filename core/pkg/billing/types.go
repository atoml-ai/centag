package billing

import (
	"fmt"
	"time"
)

// ErrRuleNotFound is returned when a pricing rule is not found.
var ErrRuleNotFound = fmt.Errorf("pricing rule not found")

// PriceType 定价类型
type PriceType string

const (
	PriceTypeCost    PriceType = "cost"    // 成本价
	PriceTypeRevenue PriceType = "revenue" // 收入价
)

// PricingRule is a configurable price row keyed by backend_id + model + price_type.
type PricingRule struct {
	ID              int64      `json:"id" yaml:"id,omitempty"`
	Name            string     `json:"name" yaml:"name"`
	BackendID       string     `json:"backend_id" yaml:"backend_id"`
	Model           string     `json:"model" yaml:"model"`
	PriceType       PriceType  `json:"price_type" yaml:"price_type"`
	InputPricePerM  float64    `json:"input_price_per_m" yaml:"input_price_per_m"`
	OutputPricePerM float64    `json:"output_price_per_m" yaml:"output_price_per_m"`
	Currency        string     `json:"currency" yaml:"currency,omitempty"`
	Priority        int        `json:"priority" yaml:"priority"`
	Enabled         bool       `json:"enabled" yaml:"enabled"`
	EffectiveAt     *time.Time `json:"effective_at,omitempty" yaml:"effective_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Source          string     `json:"source,omitempty" yaml:"source,omitempty"` // "config" 或 "manual"
	CreatedAt       time.Time  `json:"created_at,omitempty" yaml:"-"`
	UpdatedAt       time.Time  `json:"updated_at,omitempty" yaml:"-"`
}

// PricingRulesFile is the YAML document for import/export.
// Prices and currency are always USD after normalize; usd_to_cny is display-only FX.
type PricingRulesFile struct {
	Version   string        `yaml:"version"`
	Currency  string        `yaml:"currency"`
	USDToCNY  float64       `yaml:"usd_to_cny,omitempty"`
	UpdatedAt string        `yaml:"updated_at,omitempty"`
	Rules     []PricingRule `yaml:"rules"`
}

// PriceInfo is a resolved unit price for a backend/model pair.
type PriceInfo struct {
	BackendID       string    `json:"backend_id,omitempty"`
	Model           string    `json:"model,omitempty"`
	PriceType       PriceType `json:"price_type,omitempty"`
	InputPricePerM  float64   `json:"input_price_per_m"`
	OutputPricePerM float64   `json:"output_price_per_m"`
	Currency        string    `json:"currency"`
	PricingRuleID   int64     `json:"pricing_rule_id"`
	Source          string    `json:"source"` // rule | legacy_table | default | user_override
}

// CostBreakdown is an estimated cost for a token usage event.
type CostBreakdown struct {
	InputCost       float64   `json:"input_cost"`
	OutputCost      float64   `json:"output_cost"`
	TotalCost       float64   `json:"total_cost"`
	Currency        string    `json:"currency"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	PricingRuleID   int64     `json:"pricing_rule_id"`
	Source          string    `json:"source"`
	PriceType       PriceType `json:"price_type,omitempty"`
	InputPricePerM  float64   `json:"input_price_per_m,omitempty"`  // 成本侧 input 单价（USD per 1M tokens）
	OutputPricePerM float64   `json:"output_price_per_m,omitempty"` // 成本侧 output 单价（USD per 1M tokens）
}

// Price source constants for ResolvePrice / EstimateCost.
const (
	PriceSourceRule    = "rule"
	PriceSourceDefault = "default"
	PriceSourceConfig  = "config" // 从配置文件加载
)

// DefaultPricingCurrency is the storage / calculation currency (cost_usd column stores USD).
const DefaultPricingCurrency = "USD"

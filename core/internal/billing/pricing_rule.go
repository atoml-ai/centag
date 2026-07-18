package billing

import "time"

// PricingRule is a configurable price row keyed by backend_id + model.
type PricingRule struct {
	ID              int64     `json:"id" yaml:"id,omitempty"`
	Name            string    `json:"name" yaml:"name"`
	BackendID       string    `json:"backend_id" yaml:"backend_id"`
	Model           string    `json:"model" yaml:"model"`
	InputPricePerM  float64   `json:"input_price_per_m" yaml:"input_price_per_m"`
	OutputPricePerM float64   `json:"output_price_per_m" yaml:"output_price_per_m"`
	Currency        string    `json:"currency" yaml:"currency,omitempty"`
	Priority        int       `json:"priority" yaml:"priority"`
	Enabled         bool      `json:"enabled" yaml:"enabled"`
	CreatedAt       time.Time `json:"created_at,omitempty" yaml:"-"`
	UpdatedAt       time.Time `json:"updated_at,omitempty" yaml:"-"`
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

// Price source constants for ResolvePrice / EstimateCost.
const (
	PriceSourceRule        = "rule"
	PriceSourceLegacyTable = "legacy_table"
	PriceSourceDefault     = "default"
)

// DefaultPricingCurrency is the storage / calculation currency (cost_usd column stores USD).
const DefaultPricingCurrency = "USD"

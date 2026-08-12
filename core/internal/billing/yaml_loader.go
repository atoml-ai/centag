package billing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	billingpkg "centag/core/pkg/billing"
	"centag/core/pkg/bootstrap"
	"gopkg.in/yaml.v3"
)

// DefaultPricingFileCandidates are searched when no explicit path is given.
var DefaultPricingFileCandidates = []string{
	"config/pricing/default.yaml",
	filepath.Join("..", "config", "pricing", "default.yaml"),
	filepath.Join("..", "..", "config", "pricing", "default.yaml"),
}

// projectRootPricingCandidate anchors the default pricing file at ProjectRoot().
// It covers install layouts (~/.centag/lib/<edition>/config/pricing) and the
// release bundle (APP_DIR/config/pricing) where the working directory is not
// guaranteed to be the project root.
func projectRootPricingCandidate() string {
	return filepath.Join(bootstrap.ProjectRoot(), "config", "pricing", "default.yaml")
}

// ResolveDefaultPricingPath returns an existing default pricing YAML path.
// CENTAG_PRICING_FILE overrides the search when set.
func ResolveDefaultPricingPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("CENTAG_PRICING_FILE")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("CENTAG_PRICING_FILE %q: %w", p, err)
		}
		return p, nil
	}
	for _, c := range DefaultPricingFileCandidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	if c := projectRootPricingCandidate(); c != "" {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("pricing YAML not found (tried %v, plus %q)", DefaultPricingFileCandidates, projectRootPricingCandidate())
}

type pricingRuleYAML struct {
	ID              int64   `yaml:"id,omitempty"`
	Name            string  `yaml:"name"`
	BackendID       string  `yaml:"backend_id"`
	Model           string  `yaml:"model"`
	PriceType       string  `yaml:"price_type,omitempty"`
	InputPricePerM  float64 `yaml:"input_price_per_m"`
	OutputPricePerM float64 `yaml:"output_price_per_m"`
	Currency        string  `yaml:"currency,omitempty"`
	Priority        int     `yaml:"priority"`
	Enabled         *bool   `yaml:"enabled"`
}

type pricingRulesFileYAML struct {
	Version   string            `yaml:"version"`
	Currency  string            `yaml:"currency"`
	USDToCNY  float64           `yaml:"usd_to_cny"`
	UpdatedAt string            `yaml:"updated_at,omitempty"`
	Rules     []pricingRuleYAML `yaml:"rules"`
}

// ParsePricingYAML parses pricing rules from YAML bytes.
// Missing `enabled` defaults to true; explicit false is preserved.
// Currency defaults to USD; CNY files are accepted and normalized by callers via NormalizePricingFileToUSD.
func ParsePricingYAML(data []byte) (*PricingRulesFile, error) {
	var raw pricingRulesFileYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse pricing yaml: %w", err)
	}
	file := &PricingRulesFile{
		Version:   raw.Version,
		Currency:  raw.Currency,
		USDToCNY:  raw.USDToCNY,
		UpdatedAt: raw.UpdatedAt,
	}
	if file.Currency == "" {
		file.Currency = DefaultPricingCurrency
	}
	if file.USDToCNY <= 0 {
		file.USDToCNY = DefaultUSDToCNY
	}
	for i, r := range raw.Rules {
		if r.BackendID == "" || r.Model == "" {
			return nil, fmt.Errorf("rule[%d]: backend_id and model are required", i)
		}
		enabled := true
		if r.Enabled != nil {
			enabled = *r.Enabled
		}
		currency := r.Currency
		if currency == "" {
			currency = file.Currency
		}
		file.Rules = append(file.Rules, PricingRule{
			ID:              r.ID,
			Name:            r.Name,
			BackendID:       r.BackendID,
			Model:           r.Model,
			PriceType:       billingpkg.PriceType(r.PriceType),
			InputPricePerM:  r.InputPricePerM,
			OutputPricePerM: r.OutputPricePerM,
			Currency:        currency,
			Priority:        r.Priority,
			Enabled:         enabled,
		})
	}
	return file, nil
}

// LoadPricingYAMLFile reads and parses a pricing YAML file.
func LoadPricingYAMLFile(path string) (*PricingRulesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePricingYAML(data)
}

// MarshalPricingYAML serializes rules to YAML bytes (always USD + usd_to_cny).
func MarshalPricingYAML(file *PricingRulesFile) ([]byte, error) {
	if file == nil {
		file = &PricingRulesFile{Version: "1.0", Currency: DefaultPricingCurrency, USDToCNY: USDToCNY()}
	}
	if file.Version == "" {
		file.Version = "1.0"
	}
	file.Currency = DefaultPricingCurrency
	if file.USDToCNY <= 0 {
		file.USDToCNY = USDToCNY()
	}
	if file.UpdatedAt == "" {
		file.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	for i := range file.Rules {
		file.Rules[i].Currency = DefaultPricingCurrency
	}
	return yaml.Marshal(file)
}

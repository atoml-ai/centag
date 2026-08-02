package billing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"centag/core/pkg/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "pricing.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	return p
}

func TestConfigRuleStore_ListRules(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "Test Cost"
    backend_id: "test"
    model: "gpt-4o"
    price_type: "cost"
    input_price_per_m: 2.5
    output_price_per_m: 10.0
    priority: 100
    enabled: true
  - name: "Test Revenue"
    backend_id: "test"
    model: "gpt-4o"
    price_type: "revenue"
    input_price_per_m: 5.0
    output_price_per_m: 20.0
    priority: 50
    enabled: true
  - name: "Test Disabled"
    backend_id: "test"
    model: "gpt-3.5"
    price_type: "cost"
    input_price_per_m: 1.0
    output_price_per_m: 3.0
    priority: 10
    enabled: false
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	rules, err := store.ListRules(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 3) // all rules loaded, including disabled

	// IDs should be assigned sequentially
	ids := make(map[int64]bool)
	for _, r := range rules {
		assert.True(t, r.ID > 0, "rule ID should be positive")
		assert.False(t, ids[r.ID], "duplicate rule ID: %d", r.ID)
		ids[r.ID] = true
		assert.Equal(t, billing.DefaultPricingCurrency, r.Currency, "currency should be normalized to USD")
	}
}

func TestConfigRuleStore_GetRule(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "Ollama"
    backend_id: "ollama"
    model: "*"
    price_type: "cost"
    input_price_per_m: 0
    output_price_per_m: 0
    priority: 0
    enabled: true
  - name: "GPT-4o"
    backend_id: "openai"
    model: "gpt-4o"
    price_type: "cost"
    input_price_per_m: 2.5
    output_price_per_m: 10.0
    priority: 100
    enabled: true
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	// Load once to assign IDs
	_, err := store.ListRules(ctx)
	require.NoError(t, err)

	rules, _ := store.ListRules(ctx)
	var targetID int64
	for _, r := range rules {
		if r.Name == "GPT-4o" {
			targetID = r.ID
			break
		}
	}
	require.True(t, targetID > 0)

	rule, err := store.GetRule(ctx, targetID)
	require.NoError(t, err)
	assert.Equal(t, "GPT-4o", rule.Name)
	assert.Equal(t, "openai", rule.BackendID)
	assert.Equal(t, "gpt-4o", rule.Model)
	assert.Equal(t, billing.PriceTypeCost, rule.PriceType)
}

func TestConfigRuleStore_GetRule_NotFound(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "Ollama"
    backend_id: "ollama"
    model: "*"
    price_type: "cost"
    input_price_per_m: 0
    output_price_per_m: 0
    priority: 0
    enabled: true
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	_, err := store.GetRule(ctx, 9999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestConfigRuleStore_ListRulesByType(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "Cost Rule 1"
    backend_id: "a"
    model: "*"
    price_type: "cost"
    input_price_per_m: 1.0
    output_price_per_m: 2.0
    priority: 10
    enabled: true
  - name: "Cost Rule 2"
    backend_id: "b"
    model: "*"
    price_type: "cost"
    input_price_per_m: 3.0
    output_price_per_m: 4.0
    priority: 20
    enabled: true
  - name: "Revenue Rule 1"
    backend_id: "a"
    model: "*"
    price_type: "revenue"
    input_price_per_m: 5.0
    output_price_per_m: 6.0
    priority: 30
    enabled: true
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	costRules, err := store.ListRulesByType(ctx, billing.PriceTypeCost)
	require.NoError(t, err)
	assert.Len(t, costRules, 2)
	for _, r := range costRules {
		assert.Equal(t, billing.PriceTypeCost, r.PriceType)
	}

	revenueRules, err := store.ListRulesByType(ctx, billing.PriceTypeRevenue)
	require.NoError(t, err)
	assert.Len(t, revenueRules, 1)
	assert.Equal(t, billing.PriceTypeRevenue, revenueRules[0].PriceType)

	emptyRules, err := store.ListRulesByType(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Len(t, emptyRules, 0)
}

func TestConfigRuleStore_GetRuleByModelAndType(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "Wildcard"
    backend_id: "test"
    model: "*"
    price_type: "cost"
    input_price_per_m: 1.0
    output_price_per_m: 2.0
    priority: 10
    enabled: true
  - name: "Specific"
    backend_id: "test"
    model: "gpt-4o"
    price_type: "cost"
    input_price_per_m: 5.0
    output_price_per_m: 10.0
    priority: 100
    enabled: true
  - name: "Revenue Specific"
    backend_id: "test"
    model: "gpt-4o"
    price_type: "revenue"
    input_price_per_m: 8.0
    output_price_per_m: 16.0
    priority: 50
    enabled: true
  - name: "Disabled Specific"
    backend_id: "test"
    model: "gpt-4o"
    price_type: "cost"
    input_price_per_m: 0.5
    output_price_per_m: 1.0
    priority: 200
    enabled: false
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	// Should match "Specific" cost rule (highest priority enabled)
	rule, err := store.GetRuleByModelAndType(ctx, "test", "gpt-4o", billing.PriceTypeCost)
	require.NoError(t, err)
	assert.Equal(t, "Specific", rule.Name)
	assert.Equal(t, 5.0, rule.InputPricePerM)
	assert.Equal(t, 10.0, rule.OutputPricePerM)

	// Should match "Revenue Specific"
	rule, err = store.GetRuleByModelAndType(ctx, "test", "gpt-4o", billing.PriceTypeRevenue)
	require.NoError(t, err)
	assert.Equal(t, "Revenue Specific", rule.Name)

	// Wildcard match for different model
	rule, err = store.GetRuleByModelAndType(ctx, "test", "llama-3", billing.PriceTypeCost)
	require.NoError(t, err)
	assert.Equal(t, "Wildcard", rule.Name)

	// No match
	_, err = store.GetRuleByModelAndType(ctx, "unknown", "model", billing.PriceTypeCost)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pricing rule found")
}

func TestConfigRuleStore_ReadOnly(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "Test"
    backend_id: "test"
    model: "*"
    price_type: "cost"
    input_price_per_m: 1.0
    output_price_per_m: 2.0
    priority: 10
    enabled: true
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	// Load first
	_, err := store.ListRules(ctx)
	require.NoError(t, err)

	newRule := &billing.PricingRule{Name: "New"}
	assert.Error(t, store.CreateRule(ctx, newRule))
	assert.Error(t, store.UpdateRule(ctx, 1, newRule))
	assert.Error(t, store.DeleteRule(ctx, 1))
	assert.Error(t, store.ImportFromYAML(ctx, []byte("")))
	_, err = store.ExportToYAML(ctx)
	assert.Error(t, err)
}

func TestConfigRuleStore_CountRules(t *testing.T) {
	yaml := createTestYAML(t, `
version: "2.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "A"
    backend_id: "a"
    model: "*"
    price_type: "cost"
    input_price_per_m: 1.0
    output_price_per_m: 2.0
    priority: 10
    enabled: true
  - name: "B"
    backend_id: "b"
    model: "*"
    price_type: "cost"
    input_price_per_m: 3.0
    output_price_per_m: 4.0
    priority: 20
    enabled: false
`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	count, err := store.CountRules(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Second call should use cached load
	count2, err := store.CountRules(ctx)
	require.NoError(t, err)
	assert.Equal(t, count2, count)
}

func TestConfigRuleStore_InvalidYAML(t *testing.T) {
	yaml := createTestYAML(t, `this is not valid yaml: [`)
	store := NewConfigRuleStore(yaml)
	ctx := context.Background()

	_, err := store.ListRules(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load pricing YAML")
}

func TestConfigRuleStore_NonexistentFile(t *testing.T) {
	store := NewConfigRuleStore("/nonexistent/path/pricing.yaml")
	ctx := context.Background()

	_, err := store.ListRules(ctx)
	assert.Error(t, err)
}

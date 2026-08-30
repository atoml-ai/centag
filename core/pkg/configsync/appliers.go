package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"centag/core/pkg/billing"
)

// NormalizeBaseURL canonicalizes a base_url for matching: trailing slashes
// stripped and host lower-cased (path case preserved) — TC-PAP-004/005.
func NormalizeBaseURL(url string) string {
	u := strings.TrimRight(strings.TrimSpace(url), "/")
	if scheme, rest, ok := strings.Cut(u, "://"); ok {
		host := rest
		if i := strings.Index(rest, "/"); i >= 0 {
			host, rest = rest[:i], rest[i:]
		} else {
			rest = ""
		}
		u = scheme + "://" + strings.ToLower(host) + rest
	}
	return u
}

// PriceStore is the minimal interface for price persistence.
// billing.RuleStore satisfies this interface.
type PriceStore interface {
	CreateRule(ctx context.Context, rule *billing.PricingRule) error
	UpdateRule(ctx context.Context, id int64, rule *billing.PricingRule) error
	GetRuleByModelAndType(ctx context.Context, backendID, model string, priceType billing.PriceType) (*billing.PricingRule, error)
}

// --- PriceApplier ---

// BackendMapper maps a remote base_url to local backend IDs.
// Multiple backends may share one base_url — rules are replicated to all
// matches (TC-PAP-006). Return nil/empty for no match.
type BackendMapper func(baseURL string) []string

// PriceApplierSyncResult holds the outcome of a price sync operation.
type PriceApplierSyncResult struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped_manual"`
}

// ApplyPrices syncs remote model prices into the billing RuleStore.
// Rules with Source="manual" are skipped (Team mode). Personal mode
// should call with skipManual=false to do a full silent overwrite.
func ApplyPrices(ctx context.Context, prices []ProviderPrice, mapBackend BackendMapper, store PriceStore, skipManual bool) (*PriceApplierSyncResult, error) {
	result := &PriceApplierSyncResult{}
	for _, p := range prices {
		if !p.Enabled {
			continue // disabled provider row: no rules generated (TC-PAP-008)
		}
		backendIDs := mapBackend(NormalizeBaseURL(p.BaseURL))
		if len(backendIDs) == 0 {
			continue // no matching local backend
		}
		for _, backendID := range backendIDs {
			for _, m := range p.Models {
				skip, err := upsertPriceRule(ctx, store, backendID, m, p.Currency, skipManual)
				if err != nil {
					return result, fmt.Errorf("upsert price for %s/%s: %w", backendID, m.Model, err)
				}
				if skip {
					result.Skipped++
				} else {
					result.Applied++
				}
			}
		}
	}
	return result, nil
}

func upsertPriceRule(ctx context.Context, store PriceStore, backendID string, m ModelPrice, currency string, skipManual bool) (skipped bool, err error) {
	existing, err := store.GetRuleByModelAndType(ctx, backendID, m.Model, billing.PriceTypeCost)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return false, err
	}
	if existing != nil && existing.Source == "manual" && skipManual {
		return true, nil // skip manual rules
	}
	rule := &billing.PricingRule{
		Name:            fmt.Sprintf("%s/%s", backendID, m.Model),
		BackendID:       backendID,
		Model:           m.Model,
		PriceType:       billing.PriceTypeCost,
		InputPricePerM:  m.InputPricePerM,
		OutputPricePerM: m.OutputPricePerM,
		Currency:        currency,
		Priority:        10,
		Enabled:         true,
		Source:          "config",
	}
	if existing != nil {
		return false, store.UpdateRule(ctx, existing.ID, rule)
	}
	return false, store.CreateRule(ctx, rule)
}

// --- VersionApplier ---

// VersionInfo is the parsed content of a release.* row's value field.
type VersionInfo struct {
	Version       string `json:"version"`
	PackageURL    string `json:"package_url"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	MinCompatible string `json:"min_compatible"`
	ForceUpdate   bool   `json:"force_update"`
	ReleaseNotes  string `json:"release_notes"`
}

// VersionProvider can be injected into SystemUpdateHandler to provide
// remote version info as an alternative to GitHub OTA.
type VersionProvider interface {
	CheckLatest(ctx context.Context, currentVersion string) (*VersionInfo, error)
}

// versionProviderAdapter adapts the VersionProvider interface to the
// SystemUpdateHandler's expected interface (defined in system_update.go).
type versionProviderAdapter struct {
	provider VersionProvider
}

// ApplyVersions filters release.* rows for the given channel, selects the
// best match, and returns the VersionInfo (to be injected into the update handler).
func ApplyVersions(rows []Row, channel string) *VersionInfo {
	var releaseRows []Row
	for _, r := range rows {
		if strings.HasPrefix(r.Key, "release.") && r.Enabled {
			if r.Channel == "" || r.Channel == channel {
				releaseRows = append(releaseRows, r)
			}
		}
	}
	best := SelectBestRow(releaseRows)
	if best == nil {
		return nil
	}
	var info VersionInfo
	if err := json.Unmarshal(best.Value, &info); err != nil {
		return nil
	}
	return &info
}

// --- BootstrapApplier ---

// ProviderFactory constructs a Provider from a config row's value.
type ProviderFactory func(value json.RawMessage) (Provider, error)

// BootstrapApplier parses table.* rows and constructs/updates Providers.
type BootstrapApplier struct {
	factories map[string]ProviderFactory
	providers map[string]Provider
}

// NewBootstrapApplier creates a BootstrapApplier with registered factories.
func NewBootstrapApplier(factories map[string]ProviderFactory) *BootstrapApplier {
	return &BootstrapApplier{
		factories: factories,
		providers: make(map[string]Provider),
	}
}

// Apply parses table.* rows and invokes the matching factory.
func (a *BootstrapApplier) Apply(rows []Row) error {
	for _, r := range rows {
		if !strings.HasPrefix(r.Key, "table.") || !r.Enabled {
			continue
		}
		tableKey := r.Key // e.g. "table.model_price"
		factory, ok := a.factories[tableKey]
		if !ok {
			continue
		}
		provider, err := factory(r.Value)
		if err != nil {
			return fmt.Errorf("bootstrap %s: %w", tableKey, err)
		}
		a.providers[tableKey] = provider
	}
	return nil
}

// GetProvider returns the provider for a given table key (may be nil).
func (a *BootstrapApplier) GetProvider(key string) Provider {
	return a.providers[key]
}

// --- GenericApplier ---

// GenericApplier stores feature.* rows in a simple key-value map.
type GenericApplier struct {
	values map[string]json.RawMessage
}

// NewGenericApplier creates a GenericApplier.
func NewGenericApplier() *GenericApplier {
	return &GenericApplier{values: make(map[string]json.RawMessage)}
}

// Apply stores enabled feature.* rows. Disabled or non-feature rows are skipped.
func (a *GenericApplier) Apply(rows []Row) {
	for _, r := range rows {
		if strings.HasPrefix(r.Key, "feature.") && r.Enabled {
			a.values[r.Key] = r.Value
		}
	}
}

// Get returns the value for a feature key (nil if not found).
func (a *GenericApplier) Get(key string) json.RawMessage {
	return a.values[key]
}

// All returns a copy of all feature values.
func (a *GenericApplier) All() map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(a.values))
	for k, v := range a.values {
		out[k] = v
	}
	return out
}

// --- SyncResult combines all applier outcomes ---

// SyncResult holds the combined result of applying a snapshot.
type SyncResult struct {
	Prices    *PriceApplierSyncResult `json:"prices,omitempty"`
	Version   *VersionInfo            `json:"version,omitempty"`
	Features  int                     `json:"features_applied"`
	SyncTime  time.Time               `json:"sync_time"`
}

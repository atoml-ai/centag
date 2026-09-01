package configsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/billing"
	"centag/core/pkg/logger"
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
			// No matching local backend — derive backend_id from provider name
			// so prices still appear in billing rules even without backend config.
			fallbackID := DeriveBackendID(p)
			if fallbackID != "" {
				backendIDs = []string{fallbackID}
			}
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

// DeriveBackendID returns a stable backend identifier from a provider's
// base_url or provider_name, suitable for billing rules when no local
// backend is configured.
func DeriveBackendID(p ProviderPrice) string {
	// Prefer provider name if set.
	if name := strings.TrimSpace(p.ProviderName); name != "" {
		return strings.ToLower(name)
	}
	// Extract host from base_url as last resort.
	host := NormalizeBaseURL(p.BaseURL)
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	// Strip common prefixes.
	host = strings.TrimPrefix(host, "api.")
	return host
}

func upsertPriceRule(ctx context.Context, store PriceStore, backendID string, m ModelPrice, currency string, skipManual bool) (skipped bool, err error) {
	existing, err := store.GetRuleByModelAndType(ctx, backendID, m.Model, billing.PriceTypeCost)
	if err != nil && !errors.Is(err, billing.ErrRuleNotFound) {
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

// --- DBConfigApplier ---

// DBConfigApplier stores config data in the database.
type DBConfigApplier struct {
	store *DBConfigStore
}

// NewDBConfigApplier creates a DBConfigApplier.
func NewDBConfigApplier(store *DBConfigStore) *DBConfigApplier {
	return &DBConfigApplier{store: store}
}

// ApplyFromRows stores config rows to the database.
func (a *DBConfigApplier) ApplyFromRows(rows []Row) error {
	if a.store == nil {
		return nil
	}
	for _, r := range rows {
		if r.Enabled {
			if err := a.store.Upsert(r.Key, string(r.Value)); err != nil {
				logger.Warnf("configsync: upsert config %s failed: %v", r.Key, err)
				continue
			}
		}
	}
	return nil
}

// LoadFromDB loads all config from database and applies to GenericApplier.
func (a *DBConfigApplier) LoadFromDB(applier *GenericApplier) error {
	if a.store == nil {
		return nil
	}
	count, err := a.store.Count()
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	data, err := a.store.ListAll()
	if err != nil {
		return err
	}
	for key, value := range data {
		applier.values[key] = json.RawMessage(value)
	}
	return nil
}

// --- ClashRulesApplier ---

// ClashRulesApplier stores clash.rules configuration from remote configsync.
type ClashRulesApplier struct {
	rulesJSON string // raw JSON/YAML content of clash rules
	mu        sync.RWMutex
}

// NewClashRulesApplier creates a ClashRulesApplier.
func NewClashRulesApplier() *ClashRulesApplier {
	return &ClashRulesApplier{}
}

// Apply stores enabled clash.rules row content.
func (a *ClashRulesApplier) Apply(rows []Row) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range rows {
		if r.Key == "clash.rules" && r.Enabled {
			a.rulesJSON = string(r.Value)
			return
		}
	}
}

// GetRules returns the clash rules content (empty string if not configured).
func (a *ClashRulesApplier) GetRules() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rulesJSON
}

// --- BackendConfig ---

// BackendConfig represents a backend configuration for remote sync (without sensitive fields).
type BackendConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	BaseURL          string            `json:"base_url"`
	Enabled          bool              `json:"enabled"`
	Timeout          int               `json:"timeout"`
	MaxRetries       int               `json:"max_retries"`
	Description      string            `json:"description"`
	ProbeModel       string            `json:"probe_model"`
	AutoFetchModels  bool              `json:"auto_fetch_models"`
	SupportedModels  []ModelMapping    `json:"supported_models,omitempty"`
	Capabilities     ModelCapabilities `json:"capabilities,omitempty"`
	Weight           int               `json:"weight"`
	Priority         int               `json:"priority"`
	FallbackBackends []string          `json:"fallback_backends,omitempty"`
}

// ModelMapping represents a model mapping configuration.
type ModelMapping struct {
	RequestedModel     string  `json:"requested_model"`
	ActualModel        string  `json:"actual_model"`
	CompatibilityScore float64 `json:"compatibility_score"`
	IsExact            bool    `json:"is_exact"`
}

// ModelCapabilities represents backend model capabilities.
type ModelCapabilities struct {
	MaxContextTokens int      `json:"max_context_tokens"`
	Features         []string `json:"features,omitempty"`
	SupportsImages   bool     `json:"supports_images"`
	SupportsTools    bool     `json:"supports_tools"`
}

// BackendStore is the interface for backend persistence.
type BackendStore interface {
	List() []BackendConfig
	Upsert(cfg BackendConfig) error
}

// BackendApplier syncs backend configurations from remote configsync.
type BackendApplier struct {
	store BackendStore
	mu    sync.RWMutex
}

// NewBackendApplier creates a BackendApplier.
func NewBackendApplier(store BackendStore) *BackendApplier {
	return &BackendApplier{store: store}
}

// Apply processes backend.* rows and upserts non-sensitive fields.
func (a *BackendApplier) Apply(rows []Row) {
	if a.store == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range rows {
		if !strings.HasPrefix(r.Key, "backend.") || !r.Enabled {
			continue
		}
		var cfg BackendConfig
		if err := json.Unmarshal(r.Value, &cfg); err != nil {
			continue
		}
		// Skip empty IDs
		if cfg.ID == "" {
			continue
		}
		// Upsert the configuration (api_key is not synced)
		if err := a.store.Upsert(cfg); err != nil {
			continue
		}
	}
}

// --- PipelineTemplate ---

// PipelineTemplate represents a pipeline template for remote sync.
type PipelineTemplate struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Description   string                       `json:"description,omitempty"`
	ShortcutCode  string                       `json:"shortcut_code,omitempty"`
	SchemaVersion string                       `json:"schema_version,omitempty"`
	Version       string                       `json:"version,omitempty"`
	Edition       string                       `json:"edition,omitempty"` // "all", "personal", "team"
	Nodes         []PipelineNodeConfig         `json:"nodes"`
	GlobalConfig  *GlobalPipelineConfig        `json:"global_config,omitempty"`
	Metadata      map[string]interface{}       `json:"metadata,omitempty"`
}

// PipelineNodeConfig represents a pipeline node configuration.
type PipelineNodeConfig struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type,omitempty"`
	Kind            string                 `json:"kind,omitempty"`
	Implementation  string                 `json:"implementation,omitempty"`
	Name            string                 `json:"name"`
	Backend         string                 `json:"backend,omitempty"`
	Model           string                 `json:"model,omitempty"`
	Config          NodeConfig             `json:"config"`
	Inputs          map[string]string      `json:"inputs,omitempty"`
	Outputs         map[string]interface{} `json:"outputs,omitempty"`
	ConfigSchemaRef string                 `json:"config_schema_ref,omitempty"`
	SecretsRef      map[string]string      `json:"secrets_ref,omitempty"`
	Permissions     []string               `json:"permissions,omitempty"`
	Timeout         int                    `json:"timeout,omitempty"`
	Retry           *RetryConfig           `json:"retry,omitempty"`
	Condition       string                 `json:"condition,omitempty"`
	NextNodes       []string               `json:"next_nodes,omitempty"`
	DependsOn       []string               `json:"depends_on,omitempty"`
	RouteConfig     *RouteConfig           `json:"route_config,omitempty"`
}

// NodeConfig represents node-specific configuration.
type NodeConfig struct {
	Backend        string                 `json:"backend,omitempty"`
	Model          string                 `json:"model,omitempty"`
	PromptTemplate string                 `json:"prompt_template,omitempty"`
	SystemPrompt   string                 `json:"system_prompt,omitempty"`
	Temperature    *float64               `json:"temperature,omitempty"`
	MaxTokens      *int                   `json:"max_tokens,omitempty"`
	CustomConfig   map[string]interface{} `json:"custom_config,omitempty"`
	TemplateVars   map[string]string      `json:"template_vars,omitempty"`
}

// RetryConfig represents retry configuration.
type RetryConfig struct {
	MaxAttempts     int    `json:"max_attempts"`
	BackoffStrategy string `json:"backoff_strategy"`
	InitialDelay    int    `json:"initial_delay"`
	MaxDelay        int    `json:"max_delay"`
}

// RouteConfig represents route configuration.
type RouteConfig struct {
	RouterNodeID string `json:"router_node_id"`
	RouteValue   string `json:"route_value"`
	IsDefault    bool   `json:"is_default"`
}

// GlobalPipelineConfig represents global pipeline configuration.
type GlobalPipelineConfig struct {
	Timeout         int                    `json:"timeout"`
	MaxRetries      int                    `json:"max_retries"`
	BypassOnError   bool                   `json:"bypass_on_error"`
	ParallelLimit   int                    `json:"parallel_limit"`
	LogLevel        string                 `json:"log_level,omitempty"`
	SystemPrompt    string                 `json:"system_prompt,omitempty"`
	FallbackGroups  []FallbackGroup        `json:"fallback_groups,omitempty"`
	Storage         *StorageHookConfig     `json:"storage,omitempty"`
	Hooks           []HookConfig           `json:"hooks,omitempty"`
}

// FallbackGroup represents fallback group configuration.
type FallbackGroup struct {
	PrimaryNodeID  string   `json:"primary_node_id"`
	FallbackNodes  []string `json:"fallback_nodes"`
	MaxAttempts    int      `json:"max_attempts"`
}

// StorageHookConfig represents storage hook configuration.
type StorageHookConfig struct {
	Enabled       bool   `json:"enabled"`
	Namespace     string `json:"namespace"`
	AutoSave      bool   `json:"auto_save"`
	SaveInterval  int    `json:"save_interval"`
	RetentionDays int    `json:"retention_days"`
}

// HookConfig represents hook configuration.
type HookConfig struct {
	Type        string                 `json:"type"`
	On          []string               `json:"on"`
	StorageName string                 `json:"storage_name,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// PipelineTemplateStore is the interface for pipeline template persistence.
type PipelineTemplateStore interface {
	Upsert(tmpl PipelineTemplate) error
}

// PipelineTemplateApplier syncs pipeline templates from remote configsync.
type PipelineTemplateApplier struct {
	store PipelineTemplateStore
	mu    sync.RWMutex
}

// NewPipelineTemplateApplier creates a PipelineTemplateApplier.
func NewPipelineTemplateApplier(store PipelineTemplateStore) *PipelineTemplateApplier {
	return &PipelineTemplateApplier{store: store}
}

// Apply processes pipeline_template.* rows and upserts templates.
func (a *PipelineTemplateApplier) Apply(rows []Row) {
	if a.store == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range rows {
		if !strings.HasPrefix(r.Key, "pipeline_template.") || !r.Enabled {
			continue
		}
		var tmpl PipelineTemplate
		if err := json.Unmarshal(r.Value, &tmpl); err != nil {
			continue
		}
		// Skip empty IDs
		if tmpl.ID == "" {
			continue
		}
		// Upsert the template
		if err := a.store.Upsert(tmpl); err != nil {
			continue
		}
	}
}

// ApplyFromTemplates directly applies pipeline templates from remote configsync.
func (a *PipelineTemplateApplier) ApplyFromTemplates(templates []PipelineTemplate) {
	if a.store == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tmpl := range templates {
		// Skip empty IDs
		if tmpl.ID == "" {
			continue
		}
		// Upsert the template
		if err := a.store.Upsert(tmpl); err != nil {
			logger.Warnf("configsync: upsert pipeline template %s failed: %v", tmpl.ID, err)
			continue
		}
	}
}

// --- SyncResult combines all applier outcomes ---

// SyncResult holds the combined result of applying a snapshot.
type SyncResult struct {
	Prices      *PriceApplierSyncResult `json:"prices,omitempty"`
	Version     *VersionInfo            `json:"version,omitempty"`
	Features    int                     `json:"features_applied"`
	ClashRules  bool                    `json:"clash_rules_applied"`
	Backends    int                     `json:"backends_applied"`
	SyncTime    time.Time               `json:"sync_time"`
}

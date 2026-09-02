// Package configsync provides a framework for remote configuration synchronization.
//
// It defines a Provider SPI for public snapshot storage channels.
// config models, validation, version matching, and a polling scheduler.
// The framework is channel-agnostic — new channels require only a Provider
// implementation and channel descriptor registration.
package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Row is a single record from the centag_config table (bootstrap table).
type Row struct {
	Edition    string          `json:"edition"`     // "all", "personal", "team"
	Key        string          `json:"config_key"`  // e.g. "table.model_price", "release.channel.stable"
	Channel    string          `json:"channel"`     // "stable", "beta", "dev"
	MinVersion string          `json:"min_version"` // inclusive; empty = no lower bound
	MaxVersion string          `json:"max_version"` // inclusive; empty = no upper bound
	Priority   int             `json:"priority"`    // higher wins
	Value      json.RawMessage `json:"value"`       // JSON payload
	Enabled    bool            `json:"enabled"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Remark     string          `json:"remark,omitempty"`
}

// Query specifies client-side filters for config matching.
type Query struct {
	Edition string // client edition: "personal", "team"
	Version string // client semver version, e.g. "0.3.4"
	Channel string // build channel: "stable", "beta", "dev"
}

// ProviderPrice is a provider endpoint with its model prices.
type ProviderPrice struct {
	BaseURL      string       `json:"base_url"`
	ProviderName string       `json:"provider_name"`
	Currency     string       `json:"currency"` // "USD" or "CNY"
	Models       []ModelPrice `json:"models"`
	Enabled      bool         `json:"enabled"` // disabled rows produce no rules
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ModelPrice is a per-model pricing entry.
type ModelPrice struct {
	Model           string  `json:"model"`
	Name            string  `json:"name"`
	InputPricePerM  float64 `json:"input_price_per_m"`  // USD per 1M tokens
	OutputPricePerM float64 `json:"output_price_per_m"` // USD per 1M tokens
	CostMultiplier  float64 `json:"cost_multiplier"`    // e.g. 1.0
}

// Provider is the storage channel SPI. Implementations fetch config and
// optional model prices from a specific storage backend.
type Provider interface {
	// FetchConfig returns config rows matching the given query.
	FetchConfig(ctx context.Context, q Query) ([]Row, error)

	// FetchModelPrices returns model prices from the storage channel.
	// Channels that don't support this return ErrNotSupported.
	FetchModelPrices(ctx context.Context) ([]ProviderPrice, error)

	// FetchAll returns both config rows and model prices in a single fetch.
	// Default implementation calls FetchConfig + FetchModelPrices separately.
	FetchAll(ctx context.Context, q Query) ([]Row, []ProviderPrice, error)
}

// ErrNotSupported is returned by Provider implementations that don't
// support a particular capability (e.g. snapshot channel has no price table).
var ErrNotSupported = fmt.Errorf("configsync: not supported by this channel")

// Status holds the current sync state for display on management endpoints.
type Status struct {
	LastSyncTime time.Time         `json:"last_sync_time"`
	LastSyncOK   bool              `json:"last_sync_ok"`
	LastError    string            `json:"last_error,omitempty"`
	SyncCount    int               `json:"sync_count"`
	ErrorCount   int               `json:"error_count"`
	ConfigValues map[string]string `json:"config_values,omitempty"` // key → current value summary
	PriceApplied int               `json:"price_applied"`
	PriceSkipped int               `json:"price_skipped_manual"`
}

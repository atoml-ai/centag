// Package database defines the plugin interface and store contracts for user
// management, API key management and per-user/system configuration storage.
//
// The design deliberately mirrors the storage plugin pattern so that the
// underlying database engine (SQLite, PostgreSQL, …) can be swapped by
// registering a different DatabasePlugin implementation.
package database

import (
	"context"
	"time"
)

// DatabasePlugin is the top-level interface that every database backend must
// implement.  Each method returns a specialised store that handles one domain.
type DatabasePlugin interface {
	// Name returns the unique identifier of this plugin (e.g. "postgresql").
	Name() string

	// UserStore returns the store for user CRUD operations.
	UserStore() UserStore

	// APIKeyStore returns the store for API key management.
	APIKeyStore() APIKeyStore

	// RefreshTokenStore returns the store for JWT refresh token management.
	RefreshTokenStore() RefreshTokenStore

	// SystemConfigStore returns the store for system-level key/value config.
	SystemConfigStore() SystemConfigStore

	// UserConfigStore returns the store for per-user configuration blobs.
	UserConfigStore() UserConfigStore

	// ClashRuleStore returns the store for per-user Clash subscription rules.
	ClashRuleStore() ClashRuleStore

	// TenantStore returns the store for tenant management (multi-tenant).
	// Returns nil if the plugin does not support multi-tenant yet.
	TenantStore() TenantStore

	// Migrate runs all pending schema migrations in version order.
	// It is safe to call on every startup; already-applied migrations are
	// skipped.
	Migrate(ctx context.Context) error

	// HealthCheck verifies that the database is reachable and operational.
	HealthCheck(ctx context.Context) error

	// Close releases all resources held by the plugin.
	Close() error
}

// UserStore provides CRUD operations for User records.
type UserStore interface {
	// Create inserts a new user.  The user.ID field is populated on success.
	Create(ctx context.Context, user *User) error

	// GetByID returns the user with the given primary key, or
	// ErrNotFound if no such user exists.
	GetByID(ctx context.Context, id int64) (*User, error)

	// GetByUsername returns the user with the given username, or
	// ErrNotFound if no such user exists.
	GetByUsername(ctx context.Context, username string) (*User, error)

	// Update persists changes to an existing user record.
	Update(ctx context.Context, user *User) error

	// Delete removes the user and all associated data (cascade).
	Delete(ctx context.Context, id int64) error

	// List returns all users ordered by ID.
	List(ctx context.Context) ([]*User, error)

	// Count returns the total number of user records (used for first-run
	// detection).
	Count(ctx context.Context) (int64, error)
}

// APIKeyStore manages API keys for LLM proxy authentication.
type APIKeyStore interface {
	// Create inserts a new API key.  key.ID is populated on success.
	Create(ctx context.Context, key *APIKey) error

	// GetByHash looks up an API key by its SHA-256 hash (the value presented
	// in Authorization headers is hashed before lookup).
	GetByHash(ctx context.Context, keyHash string) (*APIKey, error)

	// GetByID returns a single API key record.
	GetByID(ctx context.Context, id int64) (*APIKey, error)

	// ListByUserID returns all API keys belonging to a user.
	ListByUserID(ctx context.Context, userID int64) ([]*APIKey, error)

	// ListByTenantID returns all API keys belonging to a tenant.
	// Used for multi-tenant isolation; returns empty slice for single-user mode.
	ListByTenantID(ctx context.Context, tenantID string) ([]*APIKey, error)

	// Update persists changes (name, enabled, expires_at) to an existing key.
	Update(ctx context.Context, key *APIKey) error

	// Delete removes a single API key.
	Delete(ctx context.Context, id int64) error

	// UpdateLastUsed records the timestamp of the most recent successful use.
	UpdateLastUsed(ctx context.Context, id int64, t time.Time) error

	// UpdateUsedUSD atomically adds usedUSD to the key's running total.
	// Implemented as "UPDATE api_keys SET used_usd = used_usd + $1 WHERE id = $2"
	// to avoid read-modify-write races.
	UpdateUsedUSD(ctx context.Context, id int64, usedUSD float64) error

	// ListAll returns a paginated slice of all API keys, ordered by id.
	// total is the total number of keys matching the query (ignoring offset/limit).
	ListAll(ctx context.Context, offset, limit int) ([]*APIKey, int64, error)
}

// SystemConfigStore is a simple key/value store for system-level settings.
// Values are stored as JSON strings; encoding/decoding is handled by the
// caller.
type SystemConfigStore interface {
	// Get retrieves the JSON value for a given key.  Returns ErrNotFound when
	// the key does not exist.
	Get(ctx context.Context, key string) (string, error)

	// Set creates or updates a key.
	Set(ctx context.Context, key string, value string) error

	// Delete removes a key.  It is not an error to delete a non-existent key.
	Delete(ctx context.Context, key string) error

	// List returns all key/value pairs.
	List(ctx context.Context) (map[string]string, error)
}

// UserConfigStore stores and retrieves per-user configuration blobs.
type UserConfigStore interface {
	// Get returns the configuration for the given user.  If no row exists yet,
	// an empty UserConfig (with defaults) is returned rather than ErrNotFound.
	Get(ctx context.Context, userID int64) (*UserConfig, error)

	// Upsert creates or completely replaces the configuration for a user.
	Upsert(ctx context.Context, cfg *UserConfig) error
}

// ClashRuleStore manages per-user Clash subscription rules and tokens.
// Each user may own multiple rules; every rule has its own globally-unique
// subscribe token that identifies it in public subscription URLs.
type ClashRuleStore interface {
	// ListByUserID returns all rules owned by the given user, ordered by
	// created_at ascending.
	ListByUserID(ctx context.Context, userID int64) ([]*ClashRule, error)

	// GetByID returns a single rule by primary key.
	// Returns ErrNotFound when the rule does not exist.
	GetByID(ctx context.Context, id int64) (*ClashRule, error)

	// GetByToken looks up a rule by its subscribe token.
	// Returns ErrNotFound when the token is unknown.
	GetByToken(ctx context.Context, token string) (*ClashRule, error)

	// Create inserts a new rule.  rule.ID is populated on success.
	Create(ctx context.Context, rule *ClashRule) error

	// Update persists changes (Name, RuleContent, SubscribeToken) to an
	// existing rule identified by rule.ID.
	Update(ctx context.Context, rule *ClashRule) error

	// Delete removes a single rule by primary key.
	Delete(ctx context.Context, id int64) error
}

// TenantStore manages tenant-level data and user-to-tenant mappings.
// In the "one user = one tenant" model, each user has exactly one tenant.
type TenantStore interface {
	// CreateTenant creates a new tenant record for a user.
	// tenant.UserID must be set. tenant.ID is populated on success.
	CreateTenant(ctx context.Context, tenant *Tenant) error

	// GetTenantByUserID returns the tenant for a specific user.
	// Returns ErrNotFound if the user has no tenant.
	GetTenantByUserID(ctx context.Context, userID int64) (*Tenant, error)

	// GetTenantByID returns a tenant by its unique ID.
	// Returns ErrNotFound when the tenant does not exist.
	GetTenantByID(ctx context.Context, tenantID string) (*Tenant, error)

	// UpdateTenant persists changes to an existing tenant.
	UpdateTenant(ctx context.Context, tenant *Tenant) error

	// DeleteTenant removes a tenant and all associated data (cascade).
	DeleteTenant(ctx context.Context, tenantID string) error

	// ListTenants returns all tenants in the system.
	ListTenants(ctx context.Context) ([]*Tenant, error)

	// GetTenantQuota returns the quota for a tenant.
	GetTenantQuota(ctx context.Context, tenantID string) (*TenantQuota, error)

	// SetTenantQuota creates or updates a tenant's quota.
	SetTenantQuota(ctx context.Context, quota *TenantQuota) error
}

// Tenant represents a tenant in the one-user-one-tenant model.
type Tenant struct {
	ID          string    `json:"id"`
	UserID      int64     `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // active, suspended
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TenantQuota defines resource limits for a tenant.
type TenantQuota struct {
	TenantID           string    `json:"tenant_id"`
	DailyTokenLimit    int64     `json:"daily_token_limit"`    // 0 = unlimited
	MonthlyTokenLimit  int64     `json:"monthly_token_limit"`  // 0 = unlimited
	DailyRequestLimit  int64     `json:"daily_request_limit"`  // 0 = unlimited
	MonthlyRequestLimit int64    `json:"monthly_request_limit"` // 0 = unlimited
	MaxBackends        int       `json:"max_backends"`         // 0 = unlimited
	MaxAPIKeys         int       `json:"max_api_keys"`         // 0 = unlimited
	UsedTodayTokens    int64     `json:"used_today_tokens"`
	UsedTodayRequests  int64     `json:"used_today_requests"`
	UsedMonthTokens    int64     `json:"used_month_tokens"`
	UsedMonthRequests  int64     `json:"used_month_requests"`
	ResetDate          time.Time `json:"reset_date"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// RefreshTokenStore manages refresh tokens for JWT authentication.
type RefreshTokenStore interface {
	// Create persists a new refresh token record.  token.ID is set on success.
	Create(ctx context.Context, token *RefreshToken) error

	// GetByHash looks up a refresh token by its SHA-256 hash.
	// Returns ErrNotFound when no matching, non-revoked token exists.
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)

	// Revoke marks a single refresh token as revoked.
	Revoke(ctx context.Context, hash string) error

	// RevokeAllForUser revokes every refresh token belonging to a user.
	// Used on password changes or forced logout-everywhere.
	RevokeAllForUser(ctx context.Context, userID int64) error

	// DeleteExpired removes all tokens whose expires_at is in the past.
	DeleteExpired(ctx context.Context) error
}

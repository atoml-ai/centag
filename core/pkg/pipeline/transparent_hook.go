package pipeline

import (
	"context"

	"centag/core/pkg/backend"
)

// BackendEndpoint describes a resolved upstream API endpoint.
type BackendEndpoint struct {
	BaseURL     string
	APIKey      string
	AccountPool *backend.AccountPoolConfig
}

// ResolveBackendEndpoint resolves backend_id to base URL and API key (wired in server).
var ResolveBackendEndpoint func(backendID string) (*BackendEndpoint, error)

// ListEnabledBackendsForMatch lists enabled backends for transparent model routing (wired in server).
var ListEnabledBackendsForMatch func() []*backend.BackendConfig

// FilterAllowedBackend reports whether the given backendID is in the user's plan allowlist.
// Returns true when no allowlist is configured (allow-all). Wired in pro (team plugin).
var FilterAllowedBackend func(ctx context.Context, backendID string) bool

// IsBackendAllowed checks if a backend is in the user's plan allowlist.
// This is a convenience wrapper around FilterAllowedBackend that returns true when no hook is set.
func IsBackendAllowed(ctx context.Context, backendID string) bool {
	if FilterAllowedBackend == nil {
		return true
	}
	return FilterAllowedBackend(ctx, backendID)
}

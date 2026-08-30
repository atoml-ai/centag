package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ErrNoRelease is returned by a VersionProviderAdapter when no enabled
// release row matches. The update handler falls back to GitHub OTA on this
// error (TC-VAP-003).
var ErrNoRelease = fmt.Errorf("configsync: no matching release row")

// VersionProviderAdapter adapts config rows into the VersionProvider shape
// consumed by the core system update handler (internal.VersionProvider).
type VersionProviderAdapter struct {
	// RowsFn returns the latest config rows (called per check).
	RowsFn func(ctx context.Context) ([]Row, error)
	// Channel is the build channel to match release rows against.
	Channel string
}

// NewVersionProviderAdapter builds an adapter over a row source.
func NewVersionProviderAdapter(rowsFn func(ctx context.Context) ([]Row, error), channel string) *VersionProviderAdapter {
	return &VersionProviderAdapter{RowsFn: rowsFn, Channel: channel}
}

// VersionCheckResult mirrors internal.VersionCheckResult without importing
// the internal package (pkg → internal dependency is forbidden).
type VersionCheckResult struct {
	UpdateAvailable bool   `json:"update_available"`
	Version         string `json:"version"`
	DownloadURL     string `json:"download_url,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	Message         string `json:"message,omitempty"`
}

// CheckLatest resolves the best matching release.* row for the current
// version. Enforces min_compatible so versions below the floor are never
// prompted to update (TC-VAP-007 cross-major protection).
func (a *VersionProviderAdapter) CheckLatest(ctx context.Context, currentVersion string) (*VersionCheckResult, error) {
	rows, err := a.RowsFn(ctx)
	if err != nil {
		return nil, err
	}
	info := ApplyVersions(rows, a.Channel)
	if info == nil || strings.TrimSpace(info.Version) == "" {
		return nil, ErrNoRelease
	}
	if info.MinCompatible != "" && currentVersion != "" && currentVersion != "dev" &&
		CompareVersions(currentVersion, info.MinCompatible) < 0 {
		return &VersionCheckResult{
			UpdateAvailable: false,
			Message:         fmt.Sprintf("current version %s is below min_compatible %s; update prompt suppressed", currentVersion, info.MinCompatible),
		}, nil
	}
	available := currentVersion == "" || currentVersion == "dev" || CompareVersions(info.Version, currentVersion) > 0
	msg := ""
	if !available {
		msg = "already up to date"
	}
	return &VersionCheckResult{
		UpdateAvailable: available,
		Version:         info.Version,
		DownloadURL:     info.PackageURL,
		SHA256:          info.SHA256,
		Message:         msg,
	}, nil
}

// MarshalValue is a helper for building release row payloads in tests/tools.
func MarshalValue(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

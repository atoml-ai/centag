// Package entrypoint provides the shared server startup logic for all distributions.
//
// The Run() function is provided by edition-specific files:
//   - entrypoint_full.go   (//go:build !minimal) — full version with database
//   - entrypoint_minimal.go (//go:build minimal) — file-only version, no database
package entrypoint

var (
	// Version is injected at build time via -ldflags.
	Version = "dev"
	// BuildTime is injected at build time via -ldflags.
	BuildTime = "unknown"
)

// Package editionmodule provides the open-core contract for commercial
// edition add-ons (e.g. centag-pro). Open source never imports private modules;
// closed modules register themselves via blank import from a private assemble step.
package editionmodule

import (
	"sync"

	"github.com/gin-gonic/gin"
)

// AdminDeps is the whitelist of dependencies exposed to edition modules.
// Wave A keeps this minimal; extend carefully and document in closed-repo README.
type AdminDeps struct{}

// Module is a commercial / enterprise add-on mounted on admin routes.
// Do not use pipeline BusinessPlugin for admin surfaces.
type Module interface {
	Name() string
	RegisterAdmin(rg *gin.RouterGroup, deps AdminDeps) error
	// EnrichCapabilities may add commercial capability flags.
	// Implementations should copy base before mutating; return base unchanged if unused.
	EnrichCapabilities(base map[string]bool) map[string]bool
}

// LicenseGate optionally gates commercial features. Open core defaults to allow-all.
type LicenseGate interface {
	Enabled(feature string) bool
}

type allowAllGate struct{}

func (allowAllGate) Enabled(string) bool { return true }

var (
	mu      sync.RWMutex
	modules []Module
	gate    LicenseGate = allowAllGate{}
)

// Register adds a module. Safe for init() from closed packages.
func Register(m Module) {
	if m == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	modules = append(modules, m)
}

// SetLicenseGate installs a license gate (typically from a closed module).
// Passing nil restores the open-core allow-all default.
func SetLicenseGate(g LicenseGate) {
	mu.Lock()
	defer mu.Unlock()
	if g == nil {
		gate = allowAllGate{}
		return
	}
	gate = g
}

// License returns the active license gate.
func License() LicenseGate {
	mu.RLock()
	defer mu.RUnlock()
	return gate
}

// Modules returns a snapshot of registered modules.
func Modules() []Module {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Module, len(modules))
	copy(out, modules)
	return out
}

// ResetForTest clears registry state (tests only).
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	modules = nil
	gate = allowAllGate{}
}

// MountAdmin registers all modules under rg (typically /api/v1/admin/pro).
func MountAdmin(rg *gin.RouterGroup, deps AdminDeps) error {
	for _, m := range Modules() {
		if err := m.RegisterAdmin(rg, deps); err != nil {
			return err
		}
	}
	return nil
}

// EnrichCapabilities merges capability maps from all modules.
func EnrichCapabilities(base map[string]bool) map[string]bool {
	if base == nil {
		base = map[string]bool{}
	}
	out := base
	for _, m := range Modules() {
		out = m.EnrichCapabilities(out)
		if out == nil {
			out = map[string]bool{}
		}
	}
	return out
}

// Package extension is the open-core Extension Host contract for commercial
// edition plugins (centag-pro). Open distributions (minimal/personal) never
// load private plugins. Team/enterprise binaries are built in centag-pro and
// blank-import plugin bundles that register here via init().
//
// Host whitelist (E2.0+): Edition, Deps, RegisterTeamAdmin, RegisterUserAPI,
// RegisterSystemAPI, RegisterProtectedMiddleware, RegisterBillingHook, RegisterCloser.
// Route paths stay on existing prefixes; /api/v1/admin/pro remains editionmodule-only.
package extension

import (
	"sync"

	"centag/core/pkg/hooks"

	"github.com/gin-gonic/gin"
)

// Plugin is a commercial capability pack (team base, future SSO, etc.).
// Implementations live in private repos; open core only defines the contract.
type Plugin interface {
	Name() string
	Init(host Host) error
}

// nopHost is used when Init is invoked before a real host is bound (tests).
// All Register* methods are no-ops so InitAll(nil) stays side-effect free.
type nopHost struct{ edition string }

func (h nopHost) Edition() string {
	if h.edition == "" {
		return "team"
	}
	return h.edition
}

func (nopHost) Deps() Deps                                  { return Deps{} }
func (nopHost) RegisterTeamAdmin(RouteRegistrar)            {}
func (nopHost) RegisterUserAPI(RouteRegistrar)              {}
func (nopHost) RegisterSystemAPI(RouteRegistrar)            {}
func (nopHost) RegisterProtectedMiddleware(gin.HandlerFunc) {}
func (nopHost) RegisterBillingHook(hooks.BillingHook)       {}
func (nopHost) RegisterCloser(func())                       {}

var _ Host = nopHost{}

var (
	mu      sync.RWMutex
	plugins []Plugin
)

// Register adds a plugin. Safe for init() from private packages.
func Register(p Plugin) {
	if p == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	plugins = append(plugins, p)
}

// Plugins returns a snapshot of registered plugins.
func Plugins() []Plugin {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Plugin, len(plugins))
	copy(out, plugins)
	return out
}

// ResetForTest clears the registry (tests only).
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	plugins = nil
}

// InitAll runs Init on every registered plugin.
func InitAll(host Host) error {
	if host == nil {
		host = nopHost{}
	}
	for _, p := range Plugins() {
		if err := p.Init(host); err != nil {
			return err
		}
	}
	return nil
}

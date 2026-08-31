package extension

import (
	"sync"

	"centag/core/pkg/hooks"

	"github.com/gin-gonic/gin"
)

// RouteRegistrar mounts commercial product routes onto an existing open-core
// router group. Paths stay stable (e.g. /api/v1/admin/users); plugins must not
// invent parallel CRUD under /admin/pro.
type RouteRegistrar func(rg *gin.RouterGroup)

// Deps is the whitelist of open-core handles exposed to commercial plugins.
// The server fills fields before InitAll; plugins must tolerate nil until a
// later E2 wave documents a field as required.
//
// Prefer narrow interfaces here over importing server internals. Opaque `any`
// slots are reserved for E2.1+ handler migration without widening the Host
// method set again.
type Deps struct {
	HookManager hooks.HookManager

	// BackendManager, PipelineRegistry, Database are filled by the server as
	// concrete pointers (typed any to avoid import cycles). Plugins cast.
	BackendManager   any
	PipelineRegistry any
	Database         any
	// TokenUsageService should be tokenusageapi.AdminService (Team admin usage).
	TokenUsageService any
	UserStore         any
	APIKeyStore       any
	TenantStore       any
	// SystemUpdate should be systemupdateapi.Handler.
	SystemUpdate any
	// ABEvalHandler should be abevalapi.AdminService (admin query only).
	ABEvalHandler any
	ModeManager   any
	// ConfigSyncService provides configsync status and price application.
	// Should implement ConfigSyncService interface.
	ConfigSyncService any
}

// ConfigSyncService is the interface plugins use to interact with configsync.
type ConfigSyncService interface {
	// ConfigSyncStatus returns the current sync status as a map.
	ConfigSyncStatus() map[string]any
	// ApplyConfigSyncPrices applies prices from the current snapshot to billing rules.
	ApplyConfigSyncPrices() error
	// ConfigSyncScheduler returns the scheduler for triggering manual sync.
	ConfigSyncScheduler() any
}

// Host is the whitelist of capabilities the open core exposes to plugins.
// Grow carefully; every new method needs tests and a note in centag-pro README.
type Host interface {
	// Edition returns the running product edition string (personal|team|minimal).
	Edition() string

	// Deps returns open-core handles for constructing commercial handlers.
	Deps() Deps

	// RegisterTeamAdmin queues routes on /api/v1/admin (JWT+admin+team gate).
	RegisterTeamAdmin(fn RouteRegistrar)

	// RegisterUserAPI queues routes on /api/v1/user (JWT user API).
	RegisterUserAPI(fn RouteRegistrar)

	// RegisterSystemAPI queues routes on /api/v1/system (proxy-auth + team gate).
	RegisterSystemAPI(fn RouteRegistrar)

	// RegisterProtectedMiddleware queues middleware on the proxy-auth v1 group
	// (after auth, before business handlers) — e.g. tenant quota.
	RegisterProtectedMiddleware(mw gin.HandlerFunc)

	// RegisterBillingHook queues a billing hook; the server flushes onto HookManager.
	RegisterBillingHook(hook hooks.BillingHook)

	// RegisterCloser queues shutdown work (e.g. billing.Service.Close); server calls Close().
	RegisterCloser(fn func())
}

// RuntimeHost is the server-owned Host implementation: plugins register during
// Init; the server applies queued work while wiring gin groups (R13).
type RuntimeHost struct {
	mu sync.Mutex

	edition string
	deps    Deps

	teamAdmin []RouteRegistrar
	userAPI   []RouteRegistrar
	systemAPI []RouteRegistrar
	protected []gin.HandlerFunc
	billing   []hooks.BillingHook
	closers   []func()
}

// NewRuntimeHost builds a Host bound to the given edition and dependency bag.
func NewRuntimeHost(edition string, deps Deps) *RuntimeHost {
	if edition == "" {
		edition = "team"
	}
	return &RuntimeHost{edition: edition, deps: deps}
}

func (h *RuntimeHost) Edition() string { return h.edition }

func (h *RuntimeHost) Deps() Deps {
	if h == nil {
		return Deps{}
	}
	return h.deps
}

func (h *RuntimeHost) RegisterTeamAdmin(fn RouteRegistrar) {
	if h == nil || fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.teamAdmin = append(h.teamAdmin, fn)
}

func (h *RuntimeHost) RegisterUserAPI(fn RouteRegistrar) {
	if h == nil || fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.userAPI = append(h.userAPI, fn)
}

func (h *RuntimeHost) RegisterSystemAPI(fn RouteRegistrar) {
	if h == nil || fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.systemAPI = append(h.systemAPI, fn)
}

func (h *RuntimeHost) RegisterProtectedMiddleware(mw gin.HandlerFunc) {
	if h == nil || mw == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.protected = append(h.protected, mw)
}

func (h *RuntimeHost) RegisterBillingHook(hook hooks.BillingHook) {
	if h == nil || hook == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.billing = append(h.billing, hook)
}

func (h *RuntimeHost) RegisterCloser(fn func()) {
	if h == nil || fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closers = append(h.closers, fn)
}

// Close runs queued closers in reverse registration order (nil-safe).
func (h *RuntimeHost) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	fns := append([]func(){}, h.closers...)
	h.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// ApplyTeamAdmin runs queued team-admin registrars on rg (nil-safe).
func (h *RuntimeHost) ApplyTeamAdmin(rg *gin.RouterGroup) {
	if h == nil || rg == nil {
		return
	}
	h.mu.Lock()
	fns := append([]RouteRegistrar(nil), h.teamAdmin...)
	h.mu.Unlock()
	for _, fn := range fns {
		fn(rg)
	}
}

// ApplyUserAPI runs queued user-API registrars on rg (nil-safe).
func (h *RuntimeHost) ApplyUserAPI(rg *gin.RouterGroup) {
	if h == nil || rg == nil {
		return
	}
	h.mu.Lock()
	fns := append([]RouteRegistrar(nil), h.userAPI...)
	h.mu.Unlock()
	for _, fn := range fns {
		fn(rg)
	}
}

// ApplySystemAPI runs queued system-API registrars on rg (nil-safe).
func (h *RuntimeHost) ApplySystemAPI(rg *gin.RouterGroup) {
	if h == nil || rg == nil {
		return
	}
	h.mu.Lock()
	fns := append([]RouteRegistrar(nil), h.systemAPI...)
	h.mu.Unlock()
	for _, fn := range fns {
		fn(rg)
	}
}

// ProtectedMiddlewares returns a snapshot of queued protected middlewares.
func (h *RuntimeHost) ProtectedMiddlewares() []gin.HandlerFunc {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]gin.HandlerFunc, len(h.protected))
	copy(out, h.protected)
	return out
}

// FlushBillingHooks registers queued billing hooks onto hm (nil-safe).
func (h *RuntimeHost) FlushBillingHooks(hm hooks.HookManager) {
	if h == nil || hm == nil {
		return
	}
	h.mu.Lock()
	hooksCopy := append([]hooks.BillingHook(nil), h.billing...)
	h.mu.Unlock()
	for _, hook := range hooksCopy {
		hm.RegisterBillingHook(hook)
	}
}

// Ensure RuntimeHost implements Host.
var _ Host = (*RuntimeHost)(nil)

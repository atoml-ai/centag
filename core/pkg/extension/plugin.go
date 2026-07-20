// Package extension is the open-core Extension Host contract for commercial
// edition plugins (centag-pro). Open distributions (minimal/personal) never
// load private plugins. Team/enterprise binaries are built in centag-pro and
// blank-import plugin bundles that register here via init().
package extension

import "sync"

// Plugin is a commercial capability pack (team base, future SSO, etc.).
// Implementations live in private repos; open core only defines the contract.
type Plugin interface {
	Name() string
	Init(host Host) error
}

// Host is the whitelist of capabilities the open core exposes to plugins.
// Wave E0 keeps this small; grow carefully with versioned docs in centag-pro.
type Host interface {
	// Edition returns the running product edition string (personal|team|minimal).
	Edition() string
}

// nopHost is used when Init is invoked before a real host is bound (tests).
type nopHost struct{ edition string }

func (h nopHost) Edition() string {
	if h.edition == "" {
		return "team"
	}
	return h.edition
}

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

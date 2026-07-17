package conversation

import "sync"

var (
	defaultMu    sync.RWMutex
	defaultStore Store
)

// SetDefault sets the process-wide conversation store.
func SetDefault(s Store) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultStore = s
}

// Default returns the process-wide conversation store, or nil.
func Default() Store {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultStore
}

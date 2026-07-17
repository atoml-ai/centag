package hooks

import "sync"

var (
	defaultMu      sync.RWMutex
	defaultManager HookManager
)

// SetDefault sets the process-wide HookManager (typically at server startup).
func SetDefault(m HookManager) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultManager = m
}

// Default returns the process-wide HookManager, or nil if unset.
func Default() HookManager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultManager
}

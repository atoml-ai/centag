//go:build !tray

package main

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Lite launcher: no systray / no CGO. Starts sidecar (and optional browser in main),
// then blocks until SIGINT/SIGTERM.
type launcherApp struct {
	cfg     Config
	sidecar *sidecarProcess
	mu      sync.Mutex
}

func (a *launcherApp) run(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "centag-launcher: lite mode (no tray); sidecar at %s — Ctrl+C to stop\n", a.cfg.baseURL())
	<-ctx.Done()
	a.shutdown()
	return nil
}

func (a *launcherApp) shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sidecar != nil {
		_ = a.sidecar.stop()
		a.sidecar = nil
	}
}

func quitMenu(enabled bool) {
	_ = enabled
}

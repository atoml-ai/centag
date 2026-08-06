//go:build !tray

package main

import (
	"context"
	"fmt"
	"os"
)

// Lite launcher: no systray / no CGO. Starts sidecar (and optional browser in main),
// then blocks until SIGINT/SIGTERM.
type launcherApp struct {
	cfg Config
	hub *sidecarHub
}

func (a *launcherApp) run(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "centag-launcher: lite mode (no tray); sidecar at %s — Ctrl+C to stop\n", a.cfg.baseURL())
	<-ctx.Done()
	a.shutdown()
	return nil
}

func (a *launcherApp) shutdown() {
	if a.hub != nil {
		a.hub.shutdown()
	}
}

func quitMenu(enabled bool) {
	_ = enabled
}

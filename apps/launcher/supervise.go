package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// sidecarHub owns the sidecar process and optional crash supervision.
type sidecarHub struct {
	cfg     Config
	binary  string
	dataDir string

	mu        sync.Mutex
	sidecar   *sidecarProcess
	stopping  bool
}

func newSidecarHub(cfg Config, binary, dataDir string, initial *sidecarProcess) *sidecarHub {
	return &sidecarHub{
		cfg:     cfg,
		binary:  binary,
		dataDir: dataDir,
		sidecar: initial,
	}
}

func (h *sidecarHub) shutdown() {
	h.mu.Lock()
	h.stopping = true
	proc := h.sidecar
	h.sidecar = nil
	h.mu.Unlock()
	if proc != nil {
		_ = proc.stop()
	}
}

// watch restarts the sidecar after unexpected exits when Supervise is enabled.
// Debug / explicit disable skips the restart loop after the first exit.
func (h *sidecarHub) watch(ctx context.Context) {
	if h == nil || !h.cfg.Supervise {
		return
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		h.mu.Lock()
		proc := h.sidecar
		stopping := h.stopping
		h.mu.Unlock()
		if stopping || ctx.Err() != nil {
			return
		}
		if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		waitErr := proc.wait()

		h.mu.Lock()
		if h.stopping || ctx.Err() != nil {
			h.mu.Unlock()
			return
		}
		if h.sidecar == proc {
			h.sidecar = nil
		}
		if proc.logFile != nil {
			_ = proc.logFile.Close()
			proc.logFile = nil
		}
		cfg := h.cfg
		binary := h.binary
		dataDir := h.dataDir
		h.mu.Unlock()

		fmt.Fprintf(os.Stderr, "centag-launcher: sidecar exited (%v); restarting in %s\n", waitErr, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		h.mu.Lock()
		if h.stopping {
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()

		newProc, err := startSidecar(ctx, cfg, binary, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: restart failed: %v\n", err)
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}

		h.mu.Lock()
		if h.stopping {
			h.mu.Unlock()
			_ = newProc.stop()
			return
		}
		h.sidecar = newProc
		h.mu.Unlock()
		backoff = time.Second
	}
}

func launcherSuperviseEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CENTAG_LAUNCHER_SUPERVISE")))
	switch v {
	case "0", "false", "off", "no":
		return false
	case "1", "true", "on", "yes":
		return true
	}
	// debug 模式默认不自动拉起，便于开发观察退出
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROXY_SERVER_MODE")))
	if mode == "debug" {
		return false
	}
	return true
}

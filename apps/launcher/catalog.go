package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const catalogTTL = 30 * time.Second

// Launch modes reported by the catalog (core/internal/agent LaunchMode).
const (
	launchModeWrapRun     = "wrap_run"
	launchModeSystemProxy = "system_proxy"
)

// catalogFetch / catalogNow are indirection points for tests.
var (
	catalogFetch = listCatalogApps
	catalogNow   = time.Now
)

type catalogCacheState struct {
	mu     sync.Mutex
	binary string
	at     time.Time
	apps   []catalogApp
	err    error
}

var appCatalogCache catalogCacheState

// listCatalogAppsCached wraps listCatalogApps with a 30s TTL cache so repeated
// tray/menu reads do not re-exec the sidecar each time.
func listCatalogAppsCached(binary string) ([]catalogApp, error) {
	appCatalogCache.mu.Lock()
	defer appCatalogCache.mu.Unlock()
	if appCatalogCache.binary == binary && catalogNow().Sub(appCatalogCache.at) < catalogTTL {
		return appCatalogCache.apps, appCatalogCache.err
	}
	apps, err := catalogFetch(binary)
	appCatalogCache.binary = binary
	appCatalogCache.at = catalogNow()
	appCatalogCache.apps = apps
	appCatalogCache.err = err
	return apps, err
}

// catalogApp mirrors `centag wrap apps --json` entries (see core/internal/agent
// ProxyApp + the wrap CLI's local install detection).
type catalogApp struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"display_name"`
	Vendor      string         `json:"vendor,omitempty"`
	Category    string         `json:"category,omitempty"`
	LaunchMode  string         `json:"launch_mode"`
	Argv        []string       `json:"argv,omitempty"`
	InstallURL  string         `json:"install_url,omitempty"`
	InstallHint string         `json:"install_hint,omitempty"`
	Installed   bool           `json:"installed"`
	Path        string         `json:"path,omitempty"`
	Install     catalogInstall `json:"install,omitempty"`
}

// catalogInstall mirrors the install spec from the catalog payload
// (core/internal/agent InstallSpec).
type catalogInstall struct {
	CLIBinaries []string `json:"cli_binaries,omitempty"`
	MacApps     []string `json:"mac_apps,omitempty"`
	WinExes     []string `json:"win_exes,omitempty"`
	WinMSIX     []string `json:"win_msix,omitempty"`
	WinChromium bool     `json:"win_chromium,omitempty"`
}

type catalogPayload struct {
	Apps []catalogApp `json:"apps"`
}

// listCatalogApps runs `<binary> wrap apps --json` to get the proxy-launch app
// catalog plus local install detection from the sidecar binary (offline; no
// server required). The launcher never imports Centag core.
func listCatalogApps(binary string) ([]catalogApp, error) {
	binary = strings.TrimSpace(binary)
	if binary == "" {
		return nil, fmt.Errorf("sidecar binary not resolved")
	}
	cmd := exec.Command(binary, "wrap", "apps", "--json")
	hideSidecarWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(ee.Stderr))
			if msg != "" {
				return nil, fmt.Errorf("wrap apps: %w: %s", err, msg)
			}
		}
		return nil, fmt.Errorf("wrap apps: %w", err)
	}
	return parseCatalog(out)
}

func parseCatalog(data []byte) ([]catalogApp, error) {
	var p catalogPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	return p.Apps, nil
}

// installedApps filters to installed entries and sorts by Vendor + ID, so the
// tray submenu is stable and grouped.
func installedApps(apps []catalogApp) []catalogApp {
	out := make([]catalogApp, 0, len(apps))
	for _, a := range apps {
		if a.Installed {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Vendor != out[j].Vendor {
			return out[i].Vendor < out[j].Vendor
		}
		return out[i].ID < out[j].ID
	})
	return out
}

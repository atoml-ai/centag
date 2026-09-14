// Package sysproxy reads the host's system proxy settings so the MITM egress
// resolver can relay non-LLM traffic through the proxy the user already runs
// (e.g. a VPN/accelerator bound to WinINET).
package sysproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// State is a normalized snapshot of the host system proxy.
//
// Manual is the raw ProxyServer value and is populated even when the proxy is
// disabled: Centag's wrap takeover sets a PAC URL and clears ProxyEnable, so
// the pre-existing manual proxy survives only as this residual value.
type State struct {
	Mode    string // off | manual | pac
	PACURL  string
	Manual  string
	HTTP    string
	HTTPS   string
	Enabled bool
}

// IsPACURL reports whether the URL points at Centag's PAC endpoint.
func IsPACURL(u string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(u)), "/api/v1/proxy/pac")
}

// SnapshotDir returns the directory holding the wrap proxy snapshot.
func SnapshotDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.TempDir()
		}
		return filepath.Join(base, "Centag")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".centag")
}

// ReadSnapshot loads the wrap proxy snapshot written before Centag took over.
// It returns ok=false when the snapshot is missing/unreadable/self-referential.
func ReadSnapshot() (State, bool) {
	dir := SnapshotDir()
	if dir == "" {
		return State{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, "proxy-snapshot.json"))
	if err != nil {
		return State{}, false
	}
	var snap struct {
		Proxy struct {
			Mode   string `json:"mode"`
			PACURL string `json:"pac_url"`
			HTTP   string `json:"http"`
			HTTPS  string `json:"https"`
		} `json:"proxy"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return State{}, false
	}
	p := snap.Proxy
	st := State{Mode: p.Mode, PACURL: p.PACURL, HTTP: p.HTTP, HTTPS: p.HTTPS}
	if st.Mode == "manual" && st.HTTP == "" {
		st.HTTP = st.HTTPS
	}
	if st.Mode == "" {
		st.Mode = "off"
	}
	return st, true
}

//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultMITMProxyDarwin = "127.0.0.1:8081"

// mitmAddressDarwin resolves the Centag MITM proxy address from the snapshot.
func mitmAddressDarwin() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultMITMProxyDarwin
	}
	data, err := os.ReadFile(filepath.Join(home, ".centag", "proxy-snapshot.json"))
	if err != nil {
		return defaultMITMProxyDarwin
	}
	var snap struct {
		Centag struct {
			MITMProxy string `json:"mitm_proxy"`
		} `json:"centag"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return defaultMITMProxyDarwin
	}
	if s := strings.TrimSpace(snap.Centag.MITMProxy); s != "" {
		return s
	}
	return defaultMITMProxyDarwin
}

// openAppChromium launches a Chromium/Electron desktop app with the Centag
// force-proxy switches on macOS. Uses environment variables for Electron apps.
func openAppChromium(app catalogApp) error {
	path := strings.TrimSpace(app.Path)
	if path == "" {
		return fmt.Errorf("application path not detected")
	}

	mitm := mitmAddressDarwin()
	proxyURL := "http://" + mitm

	// Build environment with proxy settings
	env := append(os.Environ(),
		"HTTP_PROXY="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"NO_PROXY=localhost,127.0.0.1,::1",
	)

	// For .app bundles, we need to launch the inner executable with env vars
	// because `open -a` doesn't support setting environment variables
	if strings.HasSuffix(path, ".app") {
		// Resolve to inner executable
		if exe, ok := resolveMacAppBundle(path); ok && exe != "" {
			cmd := exec.Command(exe)
			cmd.Env = env
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("start %s: %w", filepath.Base(path), err)
			}
			return nil
		}
	}

	// Fallback: try launching directly
	cmd := exec.Command(path)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", filepath.Base(path), err)
	}
	return nil
}

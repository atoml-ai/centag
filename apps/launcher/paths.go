package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// defaultDataDir returns the unified per-edition home ~/.centag/lib/<edition>:
// installed sidecar + static + config + runtime data (storage/logs/data),
// matching the install.sh server layout (PROJECT_ROOT=lib/<edition>).
func defaultDataDir(edition Edition) (string, error) {
	dir := editionLibDir(edition)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func resolveDataDir(cfg Config) (string, error) {
	if cfg.DataDir != "" {
		return filepath.Abs(cfg.DataDir)
	}
	return defaultDataDir(cfg.Edition)
}

// appBundleResourcesDir returns Contents/Resources when exe lives in Contents/MacOS.
func appBundleResourcesDir(exe string) string {
	macOSDir := filepath.Dir(exe)
	if filepath.Base(macOSDir) != "MacOS" {
		return ""
	}
	contents := filepath.Dir(macOSDir)
	if filepath.Base(contents) != "Contents" {
		return ""
	}
	return filepath.Join(contents, "Resources")
}

// resolveSidecarBinary finds the Centag binary without importing the core module.
//
// Priority:
//  1. explicit --bin / CENTAG_BIN
//  2. desktop bundle payload → install/upgrade into ~/.centag/lib/<edition>
//  3. already-installed ~/.centag/lib/<edition> (server layout)
//  4. cwd-relative candidates (dev)
func resolveSidecarBinary(cfg Config) (string, error) {
	if cfg.BinPath != "" {
		return absExisting(cfg.BinPath)
	}
	if v := os.Getenv("CENTAG_BIN"); v != "" {
		return absExisting(v)
	}

	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		if payloadDir := findPayloadDir(exe, cfg.Edition); payloadDir != "" {
			bin, installErr := ensureSidecarInstalled(payloadDir, cfg.Edition)
			if installErr == nil {
				return bin, nil
			}
			fmt.Fprintf(os.Stderr, "centag-launcher: sidecar install failed: %v (falling back to payload)\n", installErr)
			if p := payloadBinaryPath(payloadDir, cfg.Edition); p != "" {
				return absExisting(p)
			}
		}
	}

	searchRoots := []string{editionLibDir(cfg.Edition)}
	if wd, err := os.Getwd(); err == nil {
		searchRoots = append(searchRoots,
			wd,
			filepath.Join(wd, "bin", "server"),
			filepath.Join(wd, "..", "..", "bin", "server"),
		)
	}

	for _, root := range searchRoots {
		for _, name := range sidecarCandidateNames(cfg.Edition) {
			p := filepath.Join(root, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return filepath.Abs(p)
			}
		}
	}
	return "", fmt.Errorf("centag sidecar not found (set --bin or CENTAG_BIN); tried %v under %v", sidecarCandidateNames(cfg.Edition), searchRoots)
}

func sidecarCandidateNames(edition Edition) []string {
	var base []string
	switch edition {
	case EditionMinimal:
		base = []string{"centag-minimal", "centag"}
	default:
		base = []string{"centag-personal", "centag"}
	}
	if runtime.GOOS != "windows" {
		return base
	}
	out := make([]string, 0, len(base)*2)
	for _, n := range base {
		out = append(out, n+".exe", n)
	}
	return out
}

func absExisting(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("binary %s: %w", path, err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("binary %s is a directory", path)
	}
	return filepath.Abs(path)
}

func ensureDirs(dataDir string) error {
	for _, sub := range []string{"storage", "logs", "data"} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return err
		}
	}
	return nil
}

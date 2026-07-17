package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func userDataDir(edition Edition) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	name := "Centag"
	if edition == EditionMinimal {
		name = "CentagMinimal"
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", name), nil
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, name), nil
	default:
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, name), nil
	}
}

func resolveDataDir(cfg Config) (string, error) {
	if cfg.DataDir != "" {
		return filepath.Abs(cfg.DataDir)
	}
	dir, err := userDataDir(cfg.Edition)
	if err != nil {
		return "", err
	}
	return dir, nil
}

// resolveSidecarBinary finds the Centag binary without importing the core module.
func resolveSidecarBinary(cfg Config) (string, error) {
	if cfg.BinPath != "" {
		return absExisting(cfg.BinPath)
	}
	if v := os.Getenv("CENTAG_BIN"); v != "" {
		return absExisting(v)
	}

	candidates := sidecarCandidateNames(cfg.Edition)
	searchRoots := []string{}

	if exe, err := os.Executable(); err == nil {
		searchRoots = append(searchRoots, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		searchRoots = append(searchRoots,
			wd,
			filepath.Join(wd, "bin", "server"),
			filepath.Join(wd, "..", "..", "bin", "server"),
		)
	}

	for _, root := range searchRoots {
		for _, name := range candidates {
			p := filepath.Join(root, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return filepath.Abs(p)
			}
		}
	}
	return "", fmt.Errorf("centag sidecar not found (set --bin or CENTAG_BIN); tried %v under %v", candidates, searchRoots)
}

func sidecarCandidateNames(edition Edition) []string {
	switch edition {
	case EditionMinimal:
		return []string{"centag-minimal", "centag"}
	default:
		return []string{"centag-gateway", "centag"}
	}
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

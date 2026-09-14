//go:build linux

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// openApp opens an app/desktop entry by path on Linux.
func openApp(path, _ string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("application path not detected")
	}
	if err := exec.Command("xdg-open", path).Start(); err != nil {
		return fmt.Errorf("xdg-open: %w", err)
	}
	return nil
}

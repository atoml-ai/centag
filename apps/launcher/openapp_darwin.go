//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// openApp opens a macOS .app bundle (or executable) by path.
func openApp(path, name string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		if n := strings.TrimSpace(name); n != "" {
			return exec.Command("open", "-a", n).Start()
		}
		return fmt.Errorf("application path not detected")
	}
	return exec.Command("open", path).Start()
}

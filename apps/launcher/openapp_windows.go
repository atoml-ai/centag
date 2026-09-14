//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// openApp launches an installed GUI app by path (Start Menu target or .exe).
// Store apps (MSIX) resolve to an AUMID like `OpenAI.Codex_id!App`; those are
// opened via `explorer shell:AppsFolder\<AUMID>`.
func openApp(path, _ string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("application path not detected")
	}
	if isAUMID(path) {
		cmd := exec.Command("explorer.exe", "shell:AppsFolder\\"+path)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("open store app %q: %w", path, err)
		}
		return nil
	}
	cmd := exec.Command("cmd.exe", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open app: %w", err)
	}
	return nil
}

// isAUMID reports whether p looks like an Application User Model ID
// (`PackageFamilyName!AppId`).
func isAUMID(p string) bool {
	i := strings.LastIndex(p, "!")
	return i > 0 && strings.ContainsAny(p[:i], "_")
}

//go:build windows

package server

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func openSystemTerminal(commandLine string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	// start a new console that stays open after the agent exits.
	cmd := exec.Command("cmd.exe", "/c", "start", "", "cmd.exe", "/k", commandLine)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open cmd: %w", err)
	}
	return nil
}

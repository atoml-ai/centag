//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// launchInTerminal opens a visible console running commandLine and keeps it
// open after the agent exits (cmd /k). Interactive TUI agents (OpenCode, etc.)
// need a real TTY; launching them windowless with output redirected to a log
// makes them exit immediately. Mirrors core/pkg/server openSystemTerminal.
func launchInTerminal(commandLine, _ string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command("cmd.exe", "/c", "start", "", "cmd.exe", "/k", commandLine)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	return nil
}

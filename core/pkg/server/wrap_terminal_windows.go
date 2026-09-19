//go:build windows

package server

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// createNewConsole gives the wrapped agent its own visible console window.
const createNewConsole = 0x00000010

func openSystemTerminal(commandLine string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	// See apps/launcher/terminal_windows.go: pass a hand-built line through
	// SysProcAttr.CmdLine so Go does not re-escape the quotes into cmd-foreign
	// `\"` sequences, and add an outer quote pair to defeat cmd.exe /k quote
	// stripping. CREATE_NEW_CONSOLE keeps a visible, persistent TTY.
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd.exe /k "` + commandLine + `"`,
		CreationFlags: createNewConsole,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open cmd: %w", err)
	}
	return nil
}

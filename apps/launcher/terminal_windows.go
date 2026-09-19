//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// _CREATE_NEW_CONSOLE gives the wrapped agent its own visible console window.
const _CREATE_NEW_CONSOLE = 0x00000010

// launchInTerminal opens a visible console running commandLine and keeps it
// open after the agent exits (cmd /k). Interactive TUI agents (OpenCode, etc.)
// need a real TTY; launching them windowless with output redirected to a log
// makes them exit immediately. Mirrors core/pkg/server openSystemTerminal.
func launchInTerminal(commandLine, _ string) error {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return fmt.Errorf("empty command")
	}
	// Hand-build the command line and pass it via SysProcAttr.CmdLine. Passing
	// the already-quoted line to exec.Command as an argv element makes Go escape
	// every `"` as `\"`, which cmd.exe does not understand — it then echoes the
	// whole executable token verbatim:
	//   '\"C:\...\centag-personal.exe\"' is not recognized as a command.
	// cmd.exe also strips the first and last quote of a /k command that begins
	// with a quote, so wrap the line in one extra pair of quotes.
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       windowsTerminalCmdLine(commandLine),
		CreationFlags: _CREATE_NEW_CONSOLE,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	return nil
}

// windowsTerminalCmdLine returns the raw command line handed to cmd.exe.
func windowsTerminalCmdLine(commandLine string) string {
	return `cmd.exe /k "` + commandLine + `"`
}

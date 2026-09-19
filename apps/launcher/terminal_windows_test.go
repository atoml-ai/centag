//go:build windows

package main

import (
	"strings"
	"testing"
)

// TestWindowsTerminalCmdLine pins the raw cmd.exe line: the pre-quoted command
// must be handed to cmd.exe verbatim (via SysProcAttr.CmdLine) with one extra
// outer quote pair, otherwise cmd.exe rejects the executable token. Regression
// for: '\"C:\...\centag-personal.exe\"' is not recognized as a command.
func TestWindowsTerminalCmdLine(t *testing.T) {
	commandLine := `"C:\Users\me\.centag\lib\personal\centag-personal.exe" wrap run --server "http://127.0.0.1:20060" -- "opencode"`
	got := windowsTerminalCmdLine(commandLine)
	want := `cmd.exe /k "` + commandLine + `"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if strings.Contains(got, `\"`) {
		t.Fatalf("must not contain Go-escaped quotes: %q", got)
	}
}

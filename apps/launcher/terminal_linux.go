//go:build linux

package main

// launchInTerminal runs commandLine in the first available terminal emulator
// (Linux keeps a visible terminal; see openTerminal in run_wrap_linux.go).
func launchInTerminal(commandLine, _ string) error {
	return openTerminal(commandLine)
}

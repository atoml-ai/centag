package entrypoint

import (
	"fmt"
	"os"

	"centag/core/pkg/selfinstall"
)

// IsInstallCommand reports whether args request the install subcommand.
func IsInstallCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	return args[1] == "install"
}

// HandleInstallCommand sets up PATH, shims and env helpers for the running
// binary when args[1] is "install". Returns true if the process should stop
// (caller should return from main). Exits non-zero on failure.
func HandleInstallCommand(args []string) bool {
	if !IsInstallCommand(args) {
		return false
	}
	if code := runInstallCommand(args[2:]); code != 0 {
		os.Exit(code)
	}
	return true
}

// runInstallCommand executes install and returns a process exit code (0 = ok).
// Extracted for tests so we do not need to call os.Exit in unit tests.
func runInstallCommand(args []string) int {
	opts, help, err := selfinstall.ParseOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag install: %v\n", err)
		selfinstall.PrintInstallUsage(os.Stderr)
		return 2
	}
	if help {
		selfinstall.PrintInstallUsage(os.Stdout)
		return 0
	}
	if err := selfinstall.RunInit(opts); err != nil {
		fmt.Fprintf(os.Stderr, "centag install: %v\n", err)
		return 1
	}
	return 0
}

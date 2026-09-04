package entrypoint

import (
	"fmt"
	"os"

	"centag/core/pkg/selfinstall"
)

// IsUninstallCommand reports whether args request the uninstall subcommand
// (PATH entry / shim cleanup; runtime data is kept — see cleanup).
func IsUninstallCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "uninstall":
		return true
	default:
		return false
	}
}

// HandleUninstallCommand removes the shim, edition symlink and PATH entries
// written by HandleInstallCommand / scripts/install.sh. Returns true if the
// process should stop (caller should return from main). Exits non-zero on
// failure.
func HandleUninstallCommand(args []string) bool {
	if !IsUninstallCommand(args) {
		return false
	}
	if code := runUninstallCommand(args[2:]); code != 0 {
		os.Exit(code)
	}
	return true
}

// runUninstallCommand executes uninstall and returns a process exit code
// (0 = ok). Extracted for tests so we do not need to call os.Exit in tests.
func runUninstallCommand(args []string) int {
	opts, help, err := selfinstall.ParseOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag uninstall: %v\n", err)
		selfinstall.PrintInstallUsage(os.Stderr)
		return 2
	}
	if help {
		selfinstall.PrintInstallUsage(os.Stdout)
		return 0
	}
	if err := selfinstall.RunUninstall(opts); err != nil {
		fmt.Fprintf(os.Stderr, "centag uninstall: %v\n", err)
		return 1
	}
	return 0
}

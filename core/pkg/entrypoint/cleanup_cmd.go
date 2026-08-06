package entrypoint

import (
	"context"
	"fmt"
	"os"

	"centag/core/pkg/config"
)

// IsCleanupCommand reports whether args request the cleanup (uninstall data
// wipe) subcommand: cleanup / --cleanup / -cleanup.
func IsCleanupCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "cleanup", "--cleanup", "-cleanup":
		return true
	default:
		return false
	}
}

// runCleanupCommand executes cleanup and returns a process exit code (0 = ok).
// Extracted for tests so we do not need to call os.Exit in unit tests.
func runCleanupCommand() int {
	res := config.CleanupDeploymentData(context.Background(), "")
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "cleanup: error: %v\n", res.Error)
		return 1
	}
	if res.Cleaned {
		switch res.Driver {
		case "postgresql":
			fmt.Printf("cleanup: dropped PostgreSQL database (driver=%s)\n", res.Driver)
		case "sqlite":
			fmt.Printf("cleanup: removed SQLite database file (driver=%s)\n", res.Driver)
		default:
			fmt.Printf("cleanup: cleaned deployment data (driver=%s)\n", res.Driver)
		}
		return 0
	}
	if res.Skipped {
		fmt.Printf("cleanup: skipped (%s)\n", res.SkipReason)
		return 0
	}
	return 0
}

// HandleCleanupCommand runs the cross-platform uninstall data cleanup
// (DROP DATABASE for PostgreSQL; delete the SQLite db file for sqlite).
// Returns true if the process should stop (caller should return from main).
// Exits non-zero when cleanup fails so uninstall scripts can detect $? != 0.
func HandleCleanupCommand(args []string) bool {
	if !IsCleanupCommand(args) {
		return false
	}
	if code := runCleanupCommand(); code != 0 {
		os.Exit(code)
	}
	return true
}

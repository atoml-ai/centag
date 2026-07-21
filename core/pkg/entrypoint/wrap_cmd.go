package entrypoint

import (
	"fmt"
	"os"
)

// WrapCLIFunc runs wrap subcommands with args after "wrap"
// (e.g. ["run", "--", "opencode"]).
type WrapCLIFunc func(args []string) error

var wrapCLI WrapCLIFunc

// SetWrapCLI registers the wrap CLI implementation for this binary.
// Call from main/init before Run when the distribution includes wrap.
func SetWrapCLI(fn WrapCLIFunc) {
	wrapCLI = fn
}

// IsWrapCommand reports whether args request the wrap subcommand.
func IsWrapCommand(args []string) bool {
	return len(args) >= 2 && args[1] == "wrap"
}

// HandleWrapCommand runs the registered wrap CLI when args[1] == "wrap".
// Returns true if the process should stop (caller should return from main).
// Exits non-zero on wrap errors or when wrap is not registered.
func HandleWrapCommand(args []string) bool {
	if !IsWrapCommand(args) {
		return false
	}
	if wrapCLI == nil {
		fmt.Fprintf(os.Stderr, "wrap subcommand not available in this distribution\n")
		os.Exit(1)
	}
	if err := wrapCLI(args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "centag wrap: %v\n", err)
		os.Exit(1)
	}
	return true
}

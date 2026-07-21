package entrypoint

import (
	"fmt"
	"os"
)

// IsHelpCommand reports whether args request top-level help.
func IsHelpCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "help", "--help", "-h":
		return true
	default:
		return false
	}
}

// PrintUsage writes a short top-level usage to stdout.
func PrintUsage() {
	fmt.Print(`centag — Centag gateway / CLI

Usage:
  centag                     Start the gateway server
  centag version             Print version and exit
  centag wrap <command>      Process proxy / PAC helper (see: centag wrap help)
  centag help                Show this help

Wrap examples:
  centag wrap run -- opencode
  centag wrap run --server http://127.0.0.1:20060 -- opencode
  eval "$(centag wrap env --server http://127.0.0.1:20060)"
`)
}

// HandleHelpCommand prints usage when asked for help.
// Returns true if the process should stop.
func HandleHelpCommand(args []string) bool {
	if !IsHelpCommand(args) {
		return false
	}
	PrintUsage()
	return true
}

// ExitIfHelpCommand prints usage and exits 0.
func ExitIfHelpCommand() {
	if HandleHelpCommand(os.Args) {
		os.Exit(0)
	}
}

package entrypoint

import (
	"fmt"
	"os"
)

// IsVersionCommand reports whether args request a version print-and-exit
// (version / --version / -version / -V). Bare -v is intentionally not treated
// as version to leave room for a future verbose flag.
func IsVersionCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "version", "--version", "-version", "-V":
		return true
	default:
		return false
	}
}

// PrintVersion writes the binary version to stdout.
func PrintVersion(version, buildTime string) {
	if version == "" {
		version = "dev"
	}
	fmt.Printf("centag %s\n", version)
	if buildTime != "" && buildTime != "unknown" {
		fmt.Printf("build: %s\n", buildTime)
	}
}

// HandleVersionCommand prints version and exits 0 when args request it.
// Returns true if the process should stop (caller should return from main).
func HandleVersionCommand(version, buildTime string, args []string) bool {
	if !IsVersionCommand(args) {
		return false
	}
	PrintVersion(version, buildTime)
	return true
}

// ExitIfVersionCommand is a convenience for main: print version and os.Exit(0).
func ExitIfVersionCommand(version, buildTime string) {
	if HandleVersionCommand(version, buildTime, os.Args) {
		os.Exit(0)
	}
}

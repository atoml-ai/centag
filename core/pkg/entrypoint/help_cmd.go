package entrypoint

import (
	"fmt"
	"os"
	"strings"
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
  centag serve              Start the gateway server
  centag version            Print version and exit
  centag wrap <command>     Process proxy / PAC helper (see: centag wrap help)
  centag help               Show this help

Examples:
  centag serve                                                  # start gateway (default port 20060)
  centag wrap run --server http://127.0.0.1:20060 -- opencode   # launch an app behind the proxy
  eval "$(centag wrap env --server http://127.0.0.1:20060)"     # export proxy env in the current shell

Run ` + "`centag`" + ` without arguments to see this help.
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

// IsServeCommand reports whether args explicitly request the server.
func IsServeCommand(args []string) bool {
	return len(args) > 1 && args[1] == "serve"
}

// HandleNoArguments prints usage when the CLI is invoked without any command
// (common CLI convention: bare invocation must not start a long-running
// service) and reports whether the process should stop.
func HandleNoArguments(args []string) bool {
	return len(args) < 2
}

// knownCommands are the subcommands dispatched below the no-argument guard.
var knownCommands = map[string]bool{
	"serve": true, "version": true, "wrap": true, "cleanup": true,
	"help": true, "--help": true, "-h": true,
}

// HandleUnknownCommand reports whether args[1] is an unrecognized command
// instead of a known subcommand or a flag. The caller prints usage and exits.
func HandleUnknownCommand(args []string) bool {
	if len(args) < 2 || knownCommands[args[1]] || strings.HasPrefix(args[1], "-") {
		return false
	}
	return true
}

// ExitIfUnknownCommand prints usage and exits with code 2 when the first
// argument is an unrecognized command.
func ExitIfUnknownCommand() {
	if HandleUnknownCommand(os.Args) {
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		PrintUsage()
		os.Exit(2)
	}
}

// ExitIfHelpCommand prints usage and exits 0.
func ExitIfHelpCommand() {
	if HandleHelpCommand(os.Args) {
		os.Exit(0)
	}
}

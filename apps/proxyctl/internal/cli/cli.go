package cli

import (
	"fmt"
	"strings"

	"centag/apps/proxyctl/internal/engine"
)

// Allowed commands (whitelist). Unknown argv fails.
var allowed = map[string]bool{
	"enable":  true,
	"disable": true,
	"status":  true,
	"doctor":  true,
	"run":     true,
	"env":     true,
	"help":    true,
	"-h":      true,
	"--help":  true,
}

// Run executes proxyctl with a fixed subcommand whitelist.
func Run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return fmt.Errorf("missing command")
	}
	cmd := args[0]
	if !allowed[cmd] {
		return fmt.Errorf("unknown command %q (allowed: enable|disable|status|doctor|run|env)", cmd)
	}
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		printHelp()
		return nil
	}

	eng := engine.New()
	rest := args[1:]
	switch cmd {
	case "enable":
		server, err := parseServerFlag(rest)
		if err != nil {
			return err
		}
		return eng.Enable(server)
	case "disable":
		return eng.Disable()
	case "status":
		return eng.Status()
	case "doctor":
		server, err := parseServerFlag(rest)
		if err != nil {
			return err
		}
		return eng.Doctor(server)
	case "env":
		server, err := parseServerFlag(rest)
		if err != nil {
			return err
		}
		return eng.Env(server)
	case "run":
		server, argv, err := parseRunArgs(rest)
		if err != nil {
			return err
		}
		return eng.Run(server, argv)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func parseServerFlag(args []string) (string, error) {
	server := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--server" || a == "-s":
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", a)
			}
			i++
			server = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--server="):
			server = strings.TrimSpace(strings.TrimPrefix(a, "--server="))
		case a == "--help" || a == "-h":
			continue
		default:
			return "", fmt.Errorf("unknown flag %q", a)
		}
	}
	return server, nil
}

// parseRunArgs: [--server URL] -- <cmd> [args...]
// Also accepts: [--server URL] <cmd> [args...] when first non-flag is not --
func parseRunArgs(args []string) (server string, argv []string, err error) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			return server, args[i+1:], nil
		case a == "--server" || a == "-s":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s requires a value", a)
			}
			i++
			server = strings.TrimSpace(args[i])
			i++
		case strings.HasPrefix(a, "--server="):
			server = strings.TrimSpace(strings.TrimPrefix(a, "--server="))
			i++
		case a == "--help" || a == "-h":
			i++
		case strings.HasPrefix(a, "-"):
			return "", nil, fmt.Errorf("unknown flag %q", a)
		default:
			return server, args[i:], nil
		}
	}
	return server, nil, nil
}

func printHelp() {
	fmt.Print(`centag-proxyctl — Centag system PAC / process-proxy helper

Usage:
  centag-proxyctl enable [--server http://host:20060]
  centag-proxyctl disable
  centag-proxyctl status
  centag-proxyctl doctor [--server http://host:20060]
  centag-proxyctl env [--server http://host:20060]
  centag-proxyctl run [--server http://host:20060] -- <command> [args...]

Process proxy (recommended for OpenCode / CLI agents):
  Downloads CA, sets HTTPS_PROXY + NODE_EXTRA_CA_CERTS, then execs the command.
  Does NOT inject Centag API keys (MITM injects egress key on the server).

Examples:
  centag-proxyctl run -- opencode
  centag-proxyctl run --server http://192.168.1.4:20060 -- opencode
  eval "$(centag-proxyctl env --server http://192.168.1.4:20060)"
`)
}

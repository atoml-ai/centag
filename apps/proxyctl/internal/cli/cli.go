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
		return fmt.Errorf("unknown command %q (allowed: enable|disable|status|doctor)", cmd)
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

func printHelp() {
	fmt.Print(`centag-proxyctl — configure OS PAC/CA to use Centag as LLM egress

Usage:
  centag-proxyctl enable [--server http://host:20060]
  centag-proxyctl disable
  centag-proxyctl status
  centag-proxyctl doctor [--server http://host:20060]

Local mode (default): ensure local Centag MITM + install CA + write system PAC.
Remote/Team mode: --server points at team Centag API; does not stop remote MITM on disable.
`)
}

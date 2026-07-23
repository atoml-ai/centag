package cli

import (
	"fmt"
	"strings"

	"centag/apps/wrap/internal/engine"
)

// programName is the CLI brand shown in help (default: standalone binary).
var programName = "centag-wrap"

// SetProgramName sets the help/usage brand (e.g. "centag wrap" when embedded).
func SetProgramName(name string) {
	name = strings.TrimSpace(name)
	if name != "" {
		programName = name
	}
}

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

// commonFlags holds shared wrap flags.
type commonFlags struct {
	Server string
	Token  string
}

// Run executes wrap with a fixed subcommand whitelist.
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
		f, err := parseCommonFlags(rest)
		if err != nil {
			return err
		}
		return eng.Enable(f.Server, f.Token)
	case "disable":
		return eng.Disable()
	case "status":
		return eng.Status()
	case "doctor":
		f, err := parseCommonFlags(rest)
		if err != nil {
			return err
		}
		return eng.Doctor(f.Server, f.Token)
	case "env":
		f, err := parseCommonFlags(rest)
		if err != nil {
			return err
		}
		return eng.Env(f.Server, f.Token)
	case "run":
		f, argv, err := parseRunArgs(rest)
		if err != nil {
			return err
		}
		return eng.Run(f.Server, f.Token, argv)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func parseCommonFlags(args []string) (commonFlags, error) {
	var f commonFlags
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--server" || a == "-s":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires a value", a)
			}
			i++
			f.Server = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--server="):
			f.Server = strings.TrimSpace(strings.TrimPrefix(a, "--server="))
		case a == "--token" || a == "-t":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires a value", a)
			}
			i++
			f.Token = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--token="):
			f.Token = strings.TrimSpace(strings.TrimPrefix(a, "--token="))
		case a == "--help" || a == "-h":
			continue
		default:
			return f, fmt.Errorf("unknown flag %q", a)
		}
	}
	return f, nil
}

// parseRunArgs: [--server URL] [--token KEY] -- <cmd> [args...]
func parseRunArgs(args []string) (commonFlags, []string, error) {
	var f commonFlags
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			return f, args[i+1:], nil
		case a == "--server" || a == "-s":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("%s requires a value", a)
			}
			i++
			f.Server = strings.TrimSpace(args[i])
			i++
		case strings.HasPrefix(a, "--server="):
			f.Server = strings.TrimSpace(strings.TrimPrefix(a, "--server="))
			i++
		case a == "--token" || a == "-t":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("%s requires a value", a)
			}
			i++
			f.Token = strings.TrimSpace(args[i])
			i++
		case strings.HasPrefix(a, "--token="):
			f.Token = strings.TrimSpace(strings.TrimPrefix(a, "--token="))
			i++
		case a == "--help" || a == "-h":
			i++
		case strings.HasPrefix(a, "-"):
			return f, nil, fmt.Errorf("unknown flag %q", a)
		default:
			return f, args[i:], nil
		}
	}
	return f, nil, nil
}

func printHelp() {
	name := programName
	fmt.Printf(`%s — Centag system PAC / process-proxy helper

Usage:
  %s enable  [--server URL] [--token KEY]
  %s disable
  %s status
  %s doctor  [--server URL] [--token KEY]
  %s env     [--server URL] [--token KEY]
  %s run     [--server URL] [--token KEY] -- <command> [args...]

Flags:
  -s, --server URL   Centag API base (default: local or CENTAG_API_BASE)
  -t, --token KEY    Centag API key (llmproxy_*); overrides CENTAG_WRAP_TOKEN
                     Required for LAN MITM proxy auth

Process proxy (recommended for OpenCode / CLI agents):
  Downloads CA, sets HTTPS_PROXY (+ proxy auth when LAN) + NODE_EXTRA_CA_CERTS,
  then execs the command. Does NOT put Centag keys into the Agent Authorization header
  (MITM injects the server egress key).

Examples:
  %s run -- opencode
  %s run --server http://192.168.1.4:20060 --token llmproxy_xxx -- opencode
  eval "$(%s env --server http://192.168.1.4:20060 --token llmproxy_xxx)"
`, name, name, name, name, name, name, name, name, name, name)
	if name == "centag-wrap" {
		fmt.Print(`
Note: prefer "centag wrap …" when using the main Centag binary (same subcommands).
`)
	}
}

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
	"apps":    true,
	"help":    true,
	"-h":      true,
	"--help":  true,
}

// commonFlags holds shared wrap flags.
type commonFlags struct {
	Server      string
	Token       string
	Install     bool
	Uninstall   bool
	Force       bool
	CAOnly      bool
	ProxyOnly   bool
	SystemProxy bool
	NoProxy     string
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
		if f.CAOnly {
			return eng.EnableCAOnly(f.Server, f.Token)
		}
		return eng.Enable(f.Server, f.Token, f.Force, f.SystemProxy)
	case "disable":
		f, err := parseCommonFlags(rest)
		if err != nil {
			return err
		}
		return eng.DisableWithOptions(engine.DisableOptions{
			CAOnly:    f.CAOnly,
			ProxyOnly: f.ProxyOnly,
		})
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
		switch {
		case f.Uninstall:
			return eng.EnvUninstall(f.Server, f.Token, f.NoProxy)
		case f.Install:
			return eng.EnvInstall(f.Server, f.Token, f.NoProxy)
		default:
			return eng.Env(f.Server, f.Token, f.NoProxy)
		}
	case "apps":
		return runApps(rest)
	case "run":
		f, argv, err := parseRunArgs(rest)
		if err != nil {
			return err
		}
		return eng.Run(f.Server, f.Token, f.NoProxy, argv)
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
		case a == "--install":
			f.Install = true
		case a == "--uninstall":
			f.Uninstall = true
		case a == "--force":
			f.Force = true
		case a == "--ca-only":
			f.CAOnly = true
		case a == "--proxy-only":
			f.ProxyOnly = true
		case a == "--system-proxy":
			f.SystemProxy = true
		case a == "--no-proxy":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires a value", a)
			}
			i++
			f.NoProxy = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--no-proxy="):
			f.NoProxy = strings.TrimSpace(strings.TrimPrefix(a, "--no-proxy="))
		case a == "--help" || a == "-h":
			continue
		default:
			return f, fmt.Errorf("unknown flag %q", a)
		}
	}
	return f, nil
}

// parseRunArgs: [--server URL] [--token KEY] [--no-proxy LIST] -- <cmd> [args...]
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
		case a == "--no-proxy":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("%s requires a value", a)
			}
			i++
			f.NoProxy = strings.TrimSpace(args[i])
			i++
		case strings.HasPrefix(a, "--no-proxy="):
			f.NoProxy = strings.TrimSpace(strings.TrimPrefix(a, "--no-proxy="))
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
	fmt.Printf(`%s — Centag process-proxy helper (no system proxy by default)

Usage:
  %s enable  [--server URL] [--token KEY] [--force] [--system-proxy]
  %s disable  [--ca-only | --proxy-only]
  %s status
  %s apps    [--server URL] [--token KEY] [--installed] [--json]
  %s doctor  [--server URL] [--token KEY]
  %s env     [--server URL] [--token KEY] [--install | --uninstall] [--no-proxy LIST]
  %s run     [--server URL] [--token KEY] [--no-proxy LIST] -- <command> [args...]

Flags:
  -s, --server URL   Centag API base (default: local or CENTAG_API_BASE)
  -t, --token KEY    Centag API key (llmproxy_*); overrides CENTAG_WRAP_TOKEN
                     Required for LAN MITM proxy auth
      --install      (env) persist proxy env into your shell profile
      --uninstall    (env) remove the persisted proxy env
      --installed    (apps) only show apps detected on this machine
      --json         (apps) machine-readable output
      --force        (enable) re-assert the PAC when another program overwrote it
      --system-proxy (enable) ALSO take over the OS system proxy (PAC).
                     By default, enable only trusts the CA and ensures MITM,
                     leaving the OS system proxy untouched.
      --ca-only      (disable) only remove the CA certificate, keep OS proxy
      --proxy-only   (disable) only restore OS proxy, keep CA certificate
      --no-proxy     (run/env) comma-separated hosts/domains to bypass the proxy.
                     Appended to the default list (localhost,127.0.0.1,::1,
                     RFC1918 ranges,.localhost,.local,.lan,.example,.invalid).

Apps catalog (which local AI/agent apps Centag can proxy):
  %s apps prints the supported app catalog with best-effort local install
  detection (--installed filters to installed only; --json for machines).
  Offline via the embedded Agent registry; --server falls back to the sidecar API.

Process proxy (recommended for OpenCode / CLI agents):
  Downloads CA, sets HTTPS_PROXY (+ proxy auth when LAN) + NODE_EXTRA_CA_CERTS,
  then execs the command. Does NOT put Centag keys into the Agent Authorization header
  (MITM injects the server egress key).

Persistent env (remote / repeated use):
  %s env --install --server URL --token KEY   # auto-inject HTTPS_PROXY + CA on new shells
  %s env --uninstall                           # remove the injected block

Examples:
  %s run -- opencode
  %s run --server http://192.168.1.4:20060 --token llmproxy_xxx -- opencode
  %s run --no-proxy ".mycompany.com,10.0.0.0/8" -- opencode
  eval "$(%s env --server http://192.168.1.4:20060 --token llmproxy_xxx)"
`, name, name, name, name, name, name, name, name, name, name, name, name, name, name, name)
	if name == "centag-wrap" {
		fmt.Print(`
Note: prefer "centag wrap …" when using the main Centag binary (same subcommands).
`)
	}
}

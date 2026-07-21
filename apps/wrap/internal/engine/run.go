package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"centag/apps/wrap/internal/remote"
	"centag/apps/wrap/internal/snapshot"
)

const defaultNoProxy = "localhost,127.0.0.1,::1"

// ProcessEnv holds proxy + CA env for wrapping a third-party Agent.
type ProcessEnv struct {
	APIBase string
	MITM    string // host:port
	CAPath  string
	Vars    map[string]string
}

// ResolveAPIBase picks API base for run/env.
func ResolveAPIBase(server string) string {
	api := strings.TrimSpace(server)
	if api != "" {
		return strings.TrimRight(api, "/")
	}
	if snapshot.Exists() {
		if s, err := snapshot.Load(); err == nil && s.Centag.APIBase != "" {
			return strings.TrimRight(s.Centag.APIBase, "/")
		}
	}
	return envOr("CENTAG_API_BASE", defaultLocalAPI)
}

// PrepareProcessEnv downloads CA and resolves MITM for process-level HTTPS_PROXY.
// Does not modify system PAC or inject Centag API keys into the Agent.
func (e *Engine) PrepareProcessEnv(server string) (*ProcessEnv, error) {
	api := ResolveAPIBase(server)
	client, err := remote.New(api)
	if err != nil {
		return nil, err
	}
	client.Token = os.Getenv("CENTAG_WRAP_TOKEN")

	mitm, err := resolveMITM(client, api)
	if err != nil {
		return nil, err
	}

	// Remote team host must not advertise loopback MITM (employee ≠ server).
	// Local --server http://127.0.0.1:20060 with MITM 127.0.0.1:8081 is OK.
	if remote.RejectLoopbackMITMForRemote(api, mitm) {
		return nil, fmt.Errorf("MITM still points to %s; admin must enable LAN egress (allow_lan_clients + advertise_host) so PAC/advertise is the team host", mitm)
	}

	caPEM, err := client.DownloadCA()
	if err != nil {
		return nil, fmt.Errorf("download CA: %w", err)
	}
	caPath, err := writeCAFile(caPEM)
	if err != nil {
		return nil, err
	}

	proxyURL := "http://" + mitm
	vars := map[string]string{
		"HTTPS_PROXY":         proxyURL,
		"HTTP_PROXY":          proxyURL,
		"https_proxy":         proxyURL,
		"http_proxy":          proxyURL,
		"NO_PROXY":            defaultNoProxy,
		"no_proxy":            defaultNoProxy,
		"NODE_EXTRA_CA_CERTS": caPath,
		"SSL_CERT_FILE":       caPath,
	}
	return &ProcessEnv{APIBase: api, MITM: mitm, CAPath: caPath, Vars: vars}, nil
}

func resolveMITM(client *remote.Client, api string) (string, error) {
	if st, err := client.SetupStatus(); err == nil && strings.TrimSpace(st.MITMProxy) != "" {
		return strings.TrimSpace(st.MITMProxy), nil
	}
	pac, err := client.FetchPAC()
	if err != nil {
		return "", fmt.Errorf("resolve MITM: setup/status unavailable and PAC fetch failed: %w", err)
	}
	hostPort, err := remote.ParsePACProxyHostPort(pac)
	if err != nil {
		return "", fmt.Errorf("resolve MITM from PAC: %w", err)
	}
	_ = api
	return hostPort, nil
}

func writeCAFile(pem []byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ".centag", "wrap")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(path, pem, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// Env prints export lines for eval / debugging.
func (e *Engine) Env(server string) error {
	pe, err := e.PrepareProcessEnv(server)
	if err != nil {
		return err
	}
	fmt.Printf("# api=%s mitm=%s ca=%s\n", pe.APIBase, pe.MITM, pe.CAPath)
	for _, k := range []string{
		"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy",
		"NO_PROXY", "no_proxy", "NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE",
	} {
		fmt.Printf("export %s=%q\n", k, pe.Vars[k])
	}
	return nil
}

// Run wraps argv with process proxy env and executes it (replaces current process via Wait).
func (e *Engine) Run(server string, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("run requires a command after -- (example: centag-wrap run --server URL -- opencode)")
	}
	pe, err := e.PrepareProcessEnv(server)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("look up %q: %w", argv[0], err)
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnv(os.Environ(), pe.Vars)
	fmt.Fprintf(os.Stderr, "wrap run: HTTPS_PROXY=%s NODE_EXTRA_CA_CERTS=%s → %v\n",
		pe.Vars["HTTPS_PROXY"], pe.CAPath, argv)
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	return nil
}

func mergeEnv(base []string, overlay map[string]string) []string {
	drop := make(map[string]struct{}, len(overlay)*2)
	for k := range overlay {
		drop[k] = struct{}{}
		drop[strings.ToLower(k)] = struct{}{}
		drop[strings.ToUpper(k)] = struct{}{}
	}
	out := make([]string, 0, len(base)+len(overlay))
	for _, e := range base {
		i := strings.IndexByte(e, '=')
		if i <= 0 {
			out = append(out, e)
			continue
		}
		if _, skip := drop[e[:i]]; skip {
			continue
		}
		out = append(out, e)
	}
	for k, v := range overlay {
		out = append(out, k+"="+v)
	}
	return out
}

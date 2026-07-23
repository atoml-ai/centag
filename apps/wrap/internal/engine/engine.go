package engine

import (
	"fmt"
	"os"
	"strings"

	"centag/apps/wrap/internal/osproxy"
	"centag/apps/wrap/internal/remote"
	"centag/apps/wrap/internal/snapshot"
)

const defaultLocalAPI = "http://127.0.0.1:20060"

// Engine orchestrates enable/disable/doctor.
type Engine struct {
	OS osproxy.Backend
}

func New() *Engine {
	return &Engine{OS: osproxy.New()}
}

// resolveWrapToken prefers CLI --token, then CENTAG_WRAP_TOKEN env.
func resolveWrapToken(flagToken string) string {
	if t := strings.TrimSpace(flagToken); t != "" {
		return t
	}
	return strings.TrimSpace(os.Getenv("CENTAG_WRAP_TOKEN"))
}

func (e *Engine) Enable(server, token string) error {
	server = strings.TrimSpace(server)
	token = resolveWrapToken(token)
	if server == "" {
		return e.enableLocal(token)
	}
	return e.enableRemote(server, token)
}

func (e *Engine) enableLocal(token string) error {
	if snapshot.Exists() {
		return fmt.Errorf("already enabled (snapshot exists); run disable first")
	}
	api := envOr("CENTAG_API_BASE", defaultLocalAPI)
	client, err := remote.New(api)
	if err != nil {
		return err
	}
	client.Token = token

	prev, err := e.OS.ReadProxy()
	if err != nil {
		return fmt.Errorf("read proxy: %w", err)
	}
	snap := &snapshot.Snapshot{
		ClientMode: "local",
		Proxy:      prev,
		Centag:     snapshot.CentagRef{APIBase: api, ServerLabel: "local"},
	}
	if err := snapshot.Save(snap); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	rollback := func() {
		_ = e.OS.RestoreProxy(prev)
		_ = snapshot.Remove()
	}

	// Ensure MITM enabled via config API when token present (best-effort).
	_ = ensureLocalMITM(client)

	st, err := client.SetupStatus()
	if err != nil {
		// setup/status may require auth; fall back to public PAC/CA URLs
		st = &remote.SetupStatus{
			AllowLANClients: false,
			PACURL:          api + "/api/v1/proxy/pac",
			CADownloadURL:   api + "/api/v1/proxy/ca.crt",
			MITMProxy:       "127.0.0.1:8081",
		}
	}
	if st.AllowLANClients {
		fmt.Println("warning: local enable while server allow_lan_clients=true; PAC may advertise LAN host")
	}

	caPEM, err := client.DownloadCA()
	if err != nil {
		rollback()
		return fmt.Errorf("download CA: %w", err)
	}
	fp, err := e.OS.InstallCA(caPEM)
	if err != nil {
		rollback()
		return fmt.Errorf("install CA: %w", err)
	}
	snap.CA = snapshot.CAState{FingerprintSHA256: fp, InstalledByUs: true}
	snap.Centag.MITMProxy = st.MITMProxy
	_ = snapshot.Save(snap)

	pacURL := st.PACURL
	if pacURL == "" {
		pacURL = api + "/api/v1/proxy/pac"
	}
	if err := e.OS.WritePAC(pacURL); err != nil {
		_ = e.OS.UninstallCA(fp)
		rollback()
		return fmt.Errorf("write PAC: %w", err)
	}
	fmt.Printf("enabled local mode: pac=%s ca_fp=%s\n", pacURL, short(fp))
	return nil
}

func (e *Engine) enableRemote(server, token string) error {
	if snapshot.Exists() {
		return fmt.Errorf("already enabled (snapshot exists); run disable first")
	}
	client, err := remote.New(server)
	if err != nil {
		return err
	}
	client.Token = token

	prev, err := e.OS.ReadProxy()
	if err != nil {
		return err
	}
	snap := &snapshot.Snapshot{
		ClientMode: "remote",
		Proxy:      prev,
		Centag:     snapshot.CentagRef{APIBase: strings.TrimRight(server, "/"), ServerLabel: "team"},
	}
	if err := snapshot.Save(snap); err != nil {
		return err
	}
	rollback := func() {
		_ = e.OS.RestoreProxy(prev)
		_ = snapshot.Remove()
	}

	pacBody, err := client.FetchPAC()
	if err != nil {
		rollback()
		return fmt.Errorf("fetch PAC: %w", err)
	}
	st, err := client.SetupStatus()
	pacURL := strings.TrimRight(server, "/") + "/api/v1/proxy/pac"
	mitmProxy := ""
	if err == nil {
		if vErr := remote.ValidateRemoteReady(st, pacBody); vErr != nil {
			rollback()
			return vErr
		}
		pacURL = st.PACURL
		mitmProxy = st.MITMProxy
	} else {
		// Public PAC/CA path when setup/status requires auth
		if strings.Contains(pacBody, "PROXY 127.0.0.1:") {
			rollback()
			return fmt.Errorf("PAC still points to 127.0.0.1; admin must enable LAN egress (allow_lan_clients + advertise_host)")
		}
		fmt.Printf("warning: setup/status unavailable (%v); using public PAC URL\n", err)
	}

	caPEM, err := client.DownloadCA()
	if err != nil {
		rollback()
		return err
	}
	fp, err := e.OS.InstallCA(caPEM)
	if err != nil {
		rollback()
		return err
	}
	snap.CA = snapshot.CAState{FingerprintSHA256: fp, InstalledByUs: true}
	snap.Centag.MITMProxy = mitmProxy
	_ = snapshot.Save(snap)

	if err := e.OS.WritePAC(pacURL); err != nil {
		_ = e.OS.UninstallCA(fp)
		rollback()
		return err
	}
	fmt.Printf("enabled remote mode: server=%s pac=%s ca_fp=%s\n", server, pacURL, short(fp))
	fmt.Println("note: disable will only restore this machine; remote MITM stays up")
	return nil
}

func (e *Engine) Disable() error {
	snap, err := snapshot.Load()
	if err != nil {
		return fmt.Errorf("no snapshot to restore: %w", err)
	}
	if err := e.OS.RestoreProxy(snap.Proxy); err != nil {
		return fmt.Errorf("restore proxy: %w", err)
	}
	if snap.CA.InstalledByUs && snap.CA.FingerprintSHA256 != "" {
		if err := e.OS.UninstallCA(snap.CA.FingerprintSHA256); err != nil {
			fmt.Fprintf(os.Stderr, "warning: uninstall CA: %v\n", err)
		}
	}
	// remote mode: never call server to disable MITM
	if snap.ClientMode == "local" {
		fmt.Println("local mode: OS proxy restored; stop MITM from Centag Web if desired")
	} else {
		fmt.Println("remote mode: OS proxy restored; team server MITM left running")
	}
	if err := snapshot.Remove(); err != nil {
		return err
	}
	fmt.Println("disabled and snapshot removed")
	return nil
}

func (e *Engine) Status() error {
	ok, detail := e.OS.Supported()
	fmt.Printf("os_backend: supported=%v (%s)\n", ok, detail)
	fmt.Printf("snapshot: %v\n", snapshot.Exists())
	if snapshot.Exists() {
		s, err := snapshot.Load()
		if err == nil {
			fmt.Printf("client_mode: %s\n", s.ClientMode)
			fmt.Printf("api_base: %s\n", s.Centag.APIBase)
			fmt.Printf("ca_fp: %s\n", short(s.CA.FingerprintSHA256))
		}
	}
	cur, err := e.OS.ReadProxy()
	if err != nil {
		return err
	}
	fmt.Printf("os_proxy_mode: %s pac_url=%s\n", cur.Mode, cur.PACURL)
	return nil
}

func (e *Engine) Doctor(server, token string) error {
	api := strings.TrimSpace(server)
	if api == "" {
		if snapshot.Exists() {
			if s, err := snapshot.Load(); err == nil && s.Centag.APIBase != "" {
				api = s.Centag.APIBase
			}
		}
		if api == "" {
			api = envOr("CENTAG_API_BASE", defaultLocalAPI)
		}
	}
	client, err := remote.New(api)
	if err != nil {
		return err
	}
	client.Token = resolveWrapToken(token)

	fmt.Printf("doctor api=%s\n", api)
	pac, err := client.FetchPAC()
	if err != nil {
		return fmt.Errorf("PAC fetch failed: %w", err)
	}
	fmt.Printf("PAC ok (%d bytes)\n", len(pac))

	if st, err := client.SetupStatus(); err != nil {
		fmt.Printf("setup/status: %v\n", err)
	} else {
		fmt.Printf("mode=%s lan=%v advertise=%s loopback=%v proxy_auth=%v\n",
			st.Mode, st.AllowLANClients, st.AdvertiseHost, st.ListenIsLoopback, st.ProxyAuthRequired || st.AllowLANClients)
		if st.AllowLANClients {
			if err := remote.ValidateRemoteReady(st, pac); err != nil {
				return err
			}
			fmt.Println("remote readiness: ok")
			if client.Token == "" {
				return fmt.Errorf("LAN requires --token KEY or CENTAG_WRAP_TOKEN (llmproxy_* from WebUI → API Keys) for MITM Proxy-Authorization")
			}
			fmt.Println("wrap token: set (will be embedded in HTTPS_PROXY by wrap run)")
		}
	}
	if _, err := client.DownloadCA(); err != nil {
		return fmt.Errorf("CA download failed: %w", err)
	}
	fmt.Println("CA download: ok")
	ok, detail := e.OS.Supported()
	fmt.Printf("os_backend: supported=%v (%s)\n", ok, detail)
	fmt.Println("doctor: PASS")
	return nil
}

func ensureLocalMITM(c *remote.Client) error {
	// Optional: PUT /api/v1/config with system_proxy.enabled=true when token set.
	// Kept minimal in M1 — Web/admin enables MITM; wrap focuses on OS side.
	_ = c
	return nil
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func short(fp string) string {
	if len(fp) <= 12 {
		return fp
	}
	return fp[:12] + "…"
}

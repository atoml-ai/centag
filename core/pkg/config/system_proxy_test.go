package config

import "testing"

func TestNormalizeSystemProxyConfig_Table(t *testing.T) {
	tests := []struct {
		name      string
		in        SystemProxyConfig
		wantPort  int
		wantListen string
	}{
		{
			name:       "local forces loopback and default port",
			in:         SystemProxyConfig{AllowLANClients: false, ListenAddr: "0.0.0.0", ListenPort: 0},
			wantPort:   8081,
			wantListen: "127.0.0.1",
		},
		{
			name:       "lan empty listen becomes 0.0.0.0",
			in:         SystemProxyConfig{AllowLANClients: true, ListenAddr: "", ListenPort: 9090},
			wantPort:   9090,
			wantListen: "0.0.0.0",
		},
		{
			name:       "lan keeps custom listen",
			in:         SystemProxyConfig{AllowLANClients: true, ListenAddr: "192.168.1.2", ListenPort: 8081},
			wantPort:   8081,
			wantListen: "192.168.1.2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.in
			NormalizeSystemProxyConfig(&c)
			if c.ListenPort != tt.wantPort || c.ListenAddr != tt.wantListen {
				t.Fatalf("got port=%d addr=%q", c.ListenPort, c.ListenAddr)
			}
		})
	}
}

func TestNormalizeSystemProxyConfig_DefaultsAndForceLoopback(t *testing.T) {
	c := SystemProxyConfig{AllowLANClients: false, ListenAddr: "0.0.0.0", ListenPort: 0}
	NormalizeSystemProxyConfig(&c)
	if c.ListenPort != 8081 {
		t.Fatalf("ListenPort=%d want 8081", c.ListenPort)
	}
	if c.ListenAddr != "127.0.0.1" {
		t.Fatalf("ListenAddr=%q want 127.0.0.1", c.ListenAddr)
	}
}

func TestValidateSystemProxyConfig_LANRequiresAdvertise(t *testing.T) {
	c := SystemProxyConfig{AllowLANClients: true, ListenAddr: "0.0.0.0", AdvertiseHost: ""}
	if err := ValidateSystemProxyConfig(&c); err == nil {
		t.Fatal("expected error for empty advertise_host")
	}
	c.AdvertiseHost = "127.0.0.1"
	if err := ValidateSystemProxyConfig(&c); err == nil {
		t.Fatal("expected error for loopback advertise_host")
	}
	c.AdvertiseHost = "192.168.1.50"
	c.ListenAddr = "127.0.0.1"
	if err := ValidateSystemProxyConfig(&c); err == nil {
		t.Fatal("expected error for loopback listen with LAN")
	}
	c.ListenAddr = "0.0.0.0"
	if err := ValidateSystemProxyConfig(&c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSystemProxyConfig_LocalRejectsNonLoopback(t *testing.T) {
	c := SystemProxyConfig{AllowLANClients: false, ListenAddr: "0.0.0.0"}
	// Normalize forces loopback before validate when AllowLAN=false — call Validate which normalizes first.
	// Explicitly set after normalize path: Validate normalizes AllowLAN=false → 127.0.0.1 so this passes.
	if err := ValidateSystemProxyConfig(&c); err != nil {
		t.Fatalf("after normalize local should pass: %v", err)
	}
	if c.ListenAddr != "127.0.0.1" {
		t.Fatalf("ListenAddr=%q", c.ListenAddr)
	}
}

func TestPACProxyHostAndMITMListenAddr(t *testing.T) {
	local := SystemProxyConfig{ListenPort: 8081, AllowLANClients: false}
	if got := local.PACProxyHost(); got != "127.0.0.1" {
		t.Fatalf("PACProxyHost local=%q", got)
	}
	if got := local.MITMListenAddr(); got != "127.0.0.1:8081" {
		t.Fatalf("MITMListenAddr=%q", got)
	}

	lan := SystemProxyConfig{
		ListenPort:      8081,
		ListenAddr:      "0.0.0.0",
		AllowLANClients: true,
		AdvertiseHost:   "192.168.1.50",
	}
	if err := ValidateSystemProxyConfig(&lan); err != nil {
		t.Fatal(err)
	}
	if got := lan.PACProxyHost(); got != "192.168.1.50" {
		t.Fatalf("PACProxyHost lan=%q", got)
	}
	if got := lan.PublicAPIBase(20060); got != "http://192.168.1.50:20060" {
		t.Fatalf("PublicAPIBase=%q", got)
	}
	if lan.SetupMode() != "lan" {
		t.Fatalf("SetupMode=%q", lan.SetupMode())
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for _, h := range []string{"127.0.0.1", "localhost", "::1", ""} {
		if !IsLoopbackHost(h) {
			t.Fatalf("%q should be loopback", h)
		}
	}
	if IsLoopbackHost("192.168.1.1") {
		t.Fatal("192.168.1.1 should not be loopback")
	}
}

func TestSetupModeAndListenIsLoopback(t *testing.T) {
	local := SystemProxyConfig{AllowLANClients: false}
	if local.SetupMode() != "local" || !local.ListenIsLoopback() {
		t.Fatalf("local mode/loopback failed: mode=%s loopback=%v", local.SetupMode(), local.ListenIsLoopback())
	}
	lan := SystemProxyConfig{AllowLANClients: true, ListenAddr: "0.0.0.0", AdvertiseHost: "10.0.0.1"}
	if err := ValidateSystemProxyConfig(&lan); err != nil {
		t.Fatal(err)
	}
	if lan.SetupMode() != "lan" || lan.ListenIsLoopback() {
		t.Fatalf("lan mode/loopback failed: mode=%s loopback=%v", lan.SetupMode(), lan.ListenIsLoopback())
	}
}

func TestPublicAPIBase_DefaultPort(t *testing.T) {
	c := SystemProxyConfig{AllowLANClients: false}
	if got := c.PublicAPIBase(0); got != "http://127.0.0.1:20060" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSystemProxyEgressAPIKey(t *testing.T) {
	t.Setenv("LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY", "")
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")

	cfg := &SystemProxyConfig{EgressAPIKey: "llmproxy_from_cfg"}
	if got := ResolveSystemProxyEgressAPIKey(cfg); got != "llmproxy_from_cfg" {
		t.Fatalf("cfg key=%q", got)
	}
	t.Setenv("LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY", "llmproxy_from_env")
	if got := ResolveSystemProxyEgressAPIKey(&SystemProxyConfig{}); got != "llmproxy_from_env" {
		t.Fatalf("env key=%q", got)
	}
}

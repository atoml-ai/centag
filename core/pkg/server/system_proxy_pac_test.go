package server

import (
	"testing"

	"centag/core/pkg/config"
)

func TestBuildSystemProxyPACConfig_LocalAndLAN(t *testing.T) {
	local := &config.Config{SystemProxy: config.SystemProxyConfig{
		ListenPort: 8081, AllowLANClients: false, Domains: []string{"a.com"},
	}}
	pc := buildSystemProxyPACConfig(local)
	if pc.ProxyHost != "127.0.0.1" || pc.ProxyPort != 8081 {
		t.Fatalf("local pac=%+v", pc)
	}

	lan := &config.Config{SystemProxy: config.SystemProxyConfig{
		ListenPort: 8081, ListenAddr: "0.0.0.0", AllowLANClients: true, AdvertiseHost: "192.168.0.2",
		Domains: []string{"b.com"}, PathPatterns: []string{"/v1"},
	}}
	if err := config.ValidateSystemProxyConfig(&lan.SystemProxy); err != nil {
		t.Fatal(err)
	}
	pc = buildSystemProxyPACConfig(lan)
	if pc.ProxyHost != "192.168.0.2" {
		t.Fatalf("lan ProxyHost=%q", pc.ProxyHost)
	}
	if len(pc.Domains) != 1 || pc.Domains[0] != "b.com" {
		t.Fatalf("domains=%v", pc.Domains)
	}
}

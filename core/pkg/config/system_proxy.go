package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// NormalizeSystemProxyConfig applies safe defaults.
// When AllowLANClients is false, ListenAddr is forced to loopback.
func NormalizeSystemProxyConfig(c *SystemProxyConfig) {
	if c == nil {
		return
	}
	if c.ListenPort <= 0 {
		c.ListenPort = 8081
	}
	if !c.AllowLANClients {
		c.ListenAddr = "127.0.0.1"
		return
	}
	if strings.TrimSpace(c.ListenAddr) == "" {
		c.ListenAddr = "0.0.0.0"
	}
}

// ValidateSystemProxyConfig validates LAN / advertise rules after normalize.
func ValidateSystemProxyConfig(c *SystemProxyConfig) error {
	if c == nil {
		return fmt.Errorf("system_proxy config is nil")
	}
	NormalizeSystemProxyConfig(c)

	if !c.AllowLANClients {
		return nil
	}
	host := strings.TrimSpace(c.AdvertiseHost)
	if host == "" {
		return fmt.Errorf("advertise_host is required when allow_lan_clients is true")
	}
	if IsLoopbackHost(host) {
		return fmt.Errorf("advertise_host must not be loopback when allow_lan_clients is true")
	}
	if IsLoopbackHost(strings.TrimSpace(c.ListenAddr)) {
		return fmt.Errorf("listen_addr must not be loopback when allow_lan_clients is true (use 0.0.0.0 or a LAN IP)")
	}
	return nil
}

// IsLoopbackHost reports whether host is a loopback name/IP.
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "" || h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// MITMListenAddr returns host:port for the MITM listener.
func (c SystemProxyConfig) MITMListenAddr() string {
	NormalizeSystemProxyConfig(&c)
	return net.JoinHostPort(c.ListenAddr, fmt.Sprintf("%d", c.ListenPort))
}

// PACProxyHost returns the host written into PAC PROXY lines.
func (c SystemProxyConfig) PACProxyHost() string {
	NormalizeSystemProxyConfig(&c)
	if c.AllowLANClients {
		return strings.TrimSpace(c.AdvertiseHost)
	}
	return "127.0.0.1"
}

// SetupMode returns "lan" or "local".
func (c SystemProxyConfig) SetupMode() string {
	if c.AllowLANClients {
		return "lan"
	}
	return "local"
}

// ListenIsLoopback reports whether MITM listens on loopback only.
func (c SystemProxyConfig) ListenIsLoopback() bool {
	NormalizeSystemProxyConfig(&c)
	return IsLoopbackHost(c.ListenAddr)
}

// PublicAPIBase builds http://host:apiPort for client-facing PAC/CA URLs.
// When LAN is enabled, host is AdvertiseHost; otherwise 127.0.0.1.
func (c SystemProxyConfig) PublicAPIBase(apiPort int) string {
	NormalizeSystemProxyConfig(&c)
	host := "127.0.0.1"
	if c.AllowLANClients {
		host = strings.TrimSpace(c.AdvertiseHost)
	}
	if apiPort <= 0 {
		apiPort = 20060
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%d", apiPort)))
}

// ResolveSystemProxyEgressAPIKey returns the Centag API key MITM should inject
// when forwarding LLM traffic to the local gateway. Priority:
//  1. system_proxy.egress_api_key
//  2. LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY
//  3. LLM_PROXY_DEFAULT_ADMIN_API_KEY / LLM_PROXY_ADMIN_API_KEY
func ResolveSystemProxyEgressAPIKey(c *SystemProxyConfig) string {
	if c != nil {
		if k := strings.TrimSpace(c.EgressAPIKey); k != "" {
			return k
		}
	}
	for _, envKey := range []string{
		"LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY",
		"LLM_PROXY_DEFAULT_ADMIN_API_KEY",
		"LLM_PROXY_ADMIN_API_KEY",
	} {
		if k := strings.TrimSpace(os.Getenv(envKey)); k != "" {
			return k
		}
	}
	return ""
}

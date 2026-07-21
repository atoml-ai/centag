package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// RunningInContainer reports whether the process appears to run inside Docker/Podman.
// Used so MITM can bind 0.0.0.0 (host port publish) while PAC still points at 127.0.0.1.
// CENTAG_IN_DOCKER=0|false forces false (tests); =1|true|yes forces true.
func RunningInContainer() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CENTAG_IN_DOCKER"))) {
	case "0", "false", "no":
		return false
	case "1", "true", "yes":
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Podman / some runtimes
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	return false
}

// NormalizeSystemProxyConfig applies safe defaults.
// When AllowLANClients is false:
//   - bare metal: ListenAddr forced to loopback
//   - container: ListenAddr becomes 0.0.0.0 so published host ports can reach MITM;
//     PACProxyHost stays 127.0.0.1 for same-host wrap.
func NormalizeSystemProxyConfig(c *SystemProxyConfig) {
	if c == nil {
		return
	}
	if c.ListenPort <= 0 {
		c.ListenPort = 8081
	}
	if !c.AllowLANClients {
		if RunningInContainer() {
			if strings.TrimSpace(c.ListenAddr) == "" || IsLoopbackHost(c.ListenAddr) {
				c.ListenAddr = "0.0.0.0"
			}
			return
		}
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

// SuggestLANHosts returns non-loopback IPv4 addresses from local network interfaces.
// Used to prefill advertise_host when enabling LAN clients.
// Inside a container the addresses are usually bridge IPs (useless to host agents), so return nil.
func SuggestLANHosts() []string {
	if RunningInContainer() {
		return nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			s := ip4.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
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

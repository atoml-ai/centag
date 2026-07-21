package remote

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var proxyRe = regexp.MustCompile(`(?i)PROXY\s+([^\s"';]+)`)

// ParsePACProxyHostPort extracts the first PROXY host:port from a PAC body.
func ParsePACProxyHostPort(pacBody string) (string, error) {
	m := proxyRe.FindStringSubmatch(pacBody)
	if len(m) < 2 {
		return "", fmt.Errorf("no PROXY host:port found in PAC")
	}
	hostPort := strings.TrimSpace(m[1])
	if hostPort == "" {
		return "", fmt.Errorf("empty PROXY host:port in PAC")
	}
	return hostPort, nil
}

// IsLoopbackHost reports whether host (no port) is loopback.
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	h = strings.Trim(h, "[]")
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

// IsLoopbackMITM reports whether host:port points at loopback.
func IsLoopbackMITM(hostPort string) bool {
	h := strings.TrimSpace(strings.ToLower(hostPort))
	if h == "" {
		return true
	}
	if i := strings.LastIndex(h, ":"); i > 0 {
		h = h[:i]
	}
	return IsLoopbackHost(h)
}

// IsLoopbackAPIBase reports whether an API base URL hosts on loopback
// (local personal use: --server http://127.0.0.1:20060 is OK with MITM 127.0.0.1:8081).
func IsLoopbackAPIBase(apiBase string) bool {
	u, err := url.Parse(strings.TrimSpace(apiBase))
	if err != nil || u.Host == "" {
		return IsLoopbackHost(apiBase)
	}
	host := u.Hostname()
	if host == "" {
		host = u.Host
		if i := strings.LastIndex(host, ":"); i > 0 {
			host = host[:i]
		}
	}
	return IsLoopbackHost(host)
}

// RejectLoopbackMITMForRemote is true when the API is a remote (non-loopback) host
// but MITM still advertises 127.0.0.1 — employee machine cannot reach the server's loopback.
func RejectLoopbackMITMForRemote(apiBase, mitm string) bool {
	return !IsLoopbackAPIBase(apiBase) && IsLoopbackMITM(mitm)
}

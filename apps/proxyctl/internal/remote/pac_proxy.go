package remote

import (
	"fmt"
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

// IsLoopbackMITM reports whether host:port points at loopback.
func IsLoopbackMITM(hostPort string) bool {
	h := strings.TrimSpace(strings.ToLower(hostPort))
	if h == "" {
		return true
	}
	if i := strings.LastIndex(h, ":"); i > 0 {
		h = h[:i]
	}
	h = strings.Trim(h, "[]")
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

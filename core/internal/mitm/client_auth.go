package mitm

import (
	"encoding/base64"
	"net"
	"net/http"
	"strings"
)

// ClientTokenValidator validates a proxy client credential (llmproxy_* or JWT).
// Returns nil if the token is accepted.
type ClientTokenValidator func(token string) error

// extractProxyAuthToken reads Proxy-Authorization (Basic or Bearer) or legacy
// Authorization Bearer when used by some clients on CONNECT.
func extractProxyAuthToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if h := strings.TrimSpace(r.Header.Get("Proxy-Authorization")); h != "" {
		if tok := parseAuthHeader(h); tok != "" {
			return tok
		}
	}
	// Some stacks put Bearer on Authorization even for CONNECT.
	if h := strings.TrimSpace(r.Header.Get("Authorization")); h != "" {
		if tok := parseAuthHeader(h); tok != "" {
			return tok
		}
	}
	return ""
}

func parseAuthHeader(h string) string {
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(parts[0]))
	cred := strings.TrimSpace(parts[1])
	switch scheme {
	case "bearer":
		return cred
	case "basic":
		raw, err := base64.StdEncoding.DecodeString(cred)
		if err != nil {
			return ""
		}
		// user:password — accept either side if it looks like a token
		user, pass, ok := strings.Cut(string(raw), ":")
		if !ok {
			return strings.TrimSpace(string(raw))
		}
		user = strings.TrimSpace(user)
		pass = strings.TrimSpace(pass)
		if pass != "" {
			return pass
		}
		return user
	default:
		return ""
	}
}

func remoteIPIsLoopback(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) requireClientProxyAuth(w http.ResponseWriter, r *http.Request) bool {
	if s == nil || r == nil {
		return true
	}
	s.mu.RLock()
	required := s.requireClientProxyAuthFlag
	validate := s.clientTokenValidator
	s.mu.RUnlock()

	if !required {
		return true
	}
	// Same-machine clients (loopback) skip proxy auth — LAN peers must authenticate.
	if remoteIPIsLoopback(r.RemoteAddr) {
		return true
	}
	if validate == nil {
		w.Header().Set("Proxy-Authenticate", `Basic realm="centag-mitm"`)
		http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
		return false
	}
	token := extractProxyAuthToken(r)
	if token == "" || validate(token) != nil {
		w.Header().Set("Proxy-Authenticate", `Basic realm="centag-mitm"`)
		http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
		return false
	}
	return true
}

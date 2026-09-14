package mitm

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"centag/core/internal/sysproxy"
	"centag/core/pkg/logger"

	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

// egressUpstream decides, per target, whether MITM should relay a connection
// through an upstream proxy (the user's original VPN/accelerator) or dial it
// directly. It is safe for concurrent use and hot-updatable at runtime.
type egressUpstream struct {
	mu      sync.RWMutex
	mode    string // auto | manual | direct
	manual  string // raw manual URL / host:port
	noProxy []string
	self    []string // host:port values that must never be proxied (self loop guard)

	cached    *url.URL
	cachedSet bool
	cachedAt  time.Time
}

const egressAutoCacheTTL = 30 * time.Second

func newEgressUpstream(mode, manual string, noProxy, self []string) *egressUpstream {
	e := &egressUpstream{}
	e.configure(mode, manual, noProxy, self)
	return e
}

func (e *egressUpstream) configure(mode, manual string, noProxy, self []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mode = normalizeEgressMode(mode)
	e.manual = strings.TrimSpace(manual)
	e.noProxy = append([]string(nil), noProxy...)
	e.self = append([]string(nil), self...)
	e.cached = nil
	e.cachedSet = false
	e.cachedAt = time.Time{}
}

func normalizeEgressMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "manual", "direct":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "auto"
	}
}

// proxyFor returns the upstream proxy for target ("host:port"), or nil for a
// direct dial.
func (e *egressUpstream) proxyFor(target string) *url.URL {
	e.mu.RLock()
	mode, manual := e.mode, e.manual
	e.mu.RUnlock()

	if e.bypass(target) {
		return nil
	}

	switch mode {
	case "direct":
		return nil
	case "manual":
		u := normalizeProxyURL(manual)
		if u == nil {
			return nil
		}
		if e.isSelfURL(u) {
			logger.Warn("upstream egress manual URL points at Centag itself; using direct dial",
				zap.String("url", manual))
			return nil
		}
		return u
	default: // auto
		return e.autoUpstream()
	}
}

func (e *egressUpstream) autoUpstream() *url.URL {
	e.mu.RLock()
	u, set, at := e.cached, e.cachedSet, e.cachedAt
	e.mu.RUnlock()
	if set && time.Since(at) < egressAutoCacheTTL {
		return u
	}

	u = e.detectUpstream()

	e.mu.Lock()
	e.cached, e.cachedSet, e.cachedAt = u, true, time.Now()
	e.mu.Unlock()
	return u
}

// detectUpstream finds a non-self upstream proxy, preferring the value captured
// before Centag took over (wrap snapshot) over the live system proxy.
func (e *egressUpstream) detectUpstream() *url.URL {
	if v := strings.TrimSpace(os.Getenv("CENTAG_UPSTREAM_URL")); v != "" {
		if u := normalizeProxyURL(v); u != nil && !e.isSelfURL(u) {
			return u
		}
	}
	if st, ok := sysproxy.ReadSnapshot(); ok {
		if u := e.stateUpstream(st); u != nil {
			logger.Info("upstream egress: using pre-takeover proxy from snapshot",
				zap.String("proxy", u.String()))
			return u
		}
	}
	if st, err := sysproxy.Read(); err == nil {
		if u := e.stateUpstream(st); u != nil {
			logger.Info("upstream egress: using live system proxy",
				zap.String("proxy", u.String()))
			return u
		}
	}
	return nil
}

func (e *egressUpstream) stateUpstream(st sysproxy.State) *url.URL {
	candidate := ""
	switch st.Mode {
	case "manual":
		candidate = firstNonEmpty(st.HTTP, st.HTTPS, st.Manual)
	case "pac":
		// A Centag PAC is self: recover the residual manual proxy it displaced.
		if sysproxy.IsPACURL(st.PACURL) {
			candidate = st.Manual
		}
	}
	if candidate == "" {
		return nil
	}
	u := normalizeProxyURL(candidate)
	if u == nil || e.isSelfURL(u) {
		return nil
	}
	return u
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// bypass reports whether target must be dialed directly (never proxied).
func (e *egressUpstream) bypass(target string) bool {
	host, port := splitHostPortLoose(target)
	if host == "" {
		return false
	}
	if isLoopbackName(host) {
		return true
	}
	e.mu.RLock()
	self := append([]string(nil), e.self...)
	noProxy := append([]string(nil), e.noProxy...)
	e.mu.RUnlock()

	for _, s := range self {
		if hostPortMatches(target, host, port, s) {
			return true
		}
	}
	for _, np := range noProxy {
		if noProxyMatches(host, port, np) {
			return true
		}
	}
	return false
}

func (e *egressUpstream) isSelfURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := u.Hostname()
	port := u.Port()
	if isLoopbackName(host) {
		e.mu.RLock()
		defer e.mu.RUnlock()
		for _, s := range e.self {
			sh, sp := splitHostPortLoose(s)
			if (sp == "" || sp == port) && isLoopbackName(sh) {
				return true
			}
		}
		// Any loopback proxy on a Centag-known port is self-referential.
		switch port {
		case "8081", "8080", "20060", "20061", "20062", "20063", "20064":
			return true
		}
	}
	return false
}

func isLoopbackName(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func splitHostPortLoose(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		return strings.Trim(h, "[]"), p
	}
	return strings.Trim(s, "[]"), ""
}

func hostPortMatches(full, host, port, self string) bool {
	sh, sp := splitHostPortLoose(self)
	if !strings.EqualFold(sh, host) {
		return false
	}
	return sp == "" || sp == port
}

func noProxyMatches(host, port, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.Contains(pattern, "/") {
		if _, ipnet, err := net.ParseCIDR(pattern); err == nil {
			if ip := net.ParseIP(host); ip != nil {
				return ipnet.Contains(ip)
			}
		}
	}
	ph, pp := splitHostPortLoose(pattern)
	if pp != "" && pp != port {
		return false
	}
	if ph == "*" {
		return true
	}
	if strings.HasPrefix(ph, "*.") {
		suffix := strings.TrimPrefix(ph, "*")
		return strings.HasSuffix(strings.ToLower(host), suffix)
	}
	return strings.EqualFold(ph, host)
}

// normalizeProxyURL accepts "host:port", "http://host:port",
// "socks5://host:port" or WinINET's "http=host:port;https=host:port".
func normalizeProxyURL(raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, "=") {
		raw = pickWinINETProxy(raw)
		if raw == "" {
			return nil
		}
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks", "socks5", "socks5h":
		return u
	default:
		return nil
	}
}

func pickWinINETProxy(spec string) string {
	// e.g. "http=127.0.0.1:12000;https=127.0.0.1:12000" or "ftp=..;http=.."
	var http, https string
	for _, part := range strings.Split(spec, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "https":
			https = strings.TrimSpace(v)
		case "http":
			http = strings.TrimSpace(v)
		}
	}
	if https != "" {
		return https
	}
	return http
}

// dialContext dials addr, optionally through the configured upstream proxy.
func (e *egressUpstream) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	u := e.proxyFor(addr)
	if u == nil {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	switch strings.ToLower(u.Scheme) {
	case "socks", "socks5", "socks5h":
		return dialSOCKS5(ctx, u, network, addr)
	default:
		return dialHTTPConnect(ctx, u, addr)
	}
}

func dialHTTPConnect(ctx context.Context, u *url.URL, addr string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyDialHost(u))
	if err != nil {
		return nil, fmt.Errorf("dial upstream proxy %s: %w", u.Host, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	var req strings.Builder
	fmt.Fprintf(&req, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if u.User != nil {
		pw, _ := u.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(u.User.Username() + ":" + pw))
		fmt.Fprintf(&req, "Proxy-Authorization: Basic %s\r\n", token)
	}
	req.WriteString("Proxy-Connection: Keep-Alive\r\n\r\n")
	if _, err := conn.Write([]byte(req.String())); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write CONNECT to upstream: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read CONNECT response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		conn.Close()
		return nil, fmt.Errorf("upstream proxy CONNECT %s: %s", addr, resp.Status)
	}
	// 200: resp.Body is tied to the tunnel; do not close it. Any bytes the
	// proxy has already buffered are preserved via bufferedConn.
	_ = conn.SetDeadline(time.Time{})
	return &bufferedConn{Conn: conn, r: br}, nil
}

// bufferedConn serves any bytes buffered while reading the CONNECT response
// before reading further from the underlying connection.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func dialSOCKS5(ctx context.Context, u *url.URL, network, addr string) (net.Conn, error) {
	var auth *proxy.Auth
	if u.User != nil {
		pw, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: pw}
	}
	d, err := proxy.SOCKS5("tcp", proxyDialHost(u), auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("socks5 upstream %s: %w", u.Host, err)
	}
	if cd, ok := d.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, addr)
	}
	return d.Dial(network, addr)
}

func proxyDialHost(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return net.JoinHostPort(u.Hostname(), "443")
	case "socks", "socks5", "socks5h":
		return net.JoinHostPort(u.Hostname(), "1080")
	default:
		return net.JoinHostPort(u.Hostname(), "80")
	}
}

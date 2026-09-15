package mitm

import (
	"net"
	"testing"

	"centag/core/pkg/logger"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	prev := logger.Logger
	logger.Logger = zap.NewNop()
	defer func() { logger.Logger = prev }()
	m.Run()
}

func TestEgressUpstream_DirectMode(t *testing.T) {
	e := newEgressUpstream("direct", "", nil, []string{"127.0.0.1:8081"})
	if u := e.proxyFor("api.openai.com:443"); u != nil {
		t.Fatalf("direct mode should return nil, got %v", u)
	}
}

func TestEgressUpstream_ManualMode(t *testing.T) {
	e := newEgressUpstream("manual", "192.168.1.50:8080", nil, nil)
	u := e.proxyFor("api.openai.com:443")
	if u == nil {
		t.Fatal("manual mode should return proxy URL")
	}
	if u.Host != "192.168.1.50:8080" {
		t.Fatalf("expected 192.168.1.50:8080, got %s", u.Host)
	}
}

func TestEgressUpstream_SelfLoopGuard(t *testing.T) {
	e := newEgressUpstream("manual", "127.0.0.1:8081", nil, []string{"127.0.0.1:8081"})
	u := e.proxyFor("api.openai.com:443")
	if u != nil {
		t.Fatalf("self loop should return nil, got %v", u)
	}
}

func TestEgressUpstream_BypassLoopback(t *testing.T) {
	e := newEgressUpstream("manual", "192.168.1.50:8080", nil, nil)
	if u := e.proxyFor("127.0.0.1:443"); u != nil {
		t.Fatalf("loopback should be bypassed, got %v", u)
	}
	if u := e.proxyFor("localhost:443"); u != nil {
		t.Fatalf("localhost should be bypassed, got %v", u)
	}
}

func TestEgressUpstream_NoProxy(t *testing.T) {
	e := newEgressUpstream("manual", "192.168.1.50:8080", []string{"*.internal.com"}, nil)
	if u := e.proxyFor("api.internal.com:443"); u != nil {
		t.Fatalf("noProxy should bypass, got %v", u)
	}
	u := e.proxyFor("api.openai.com:443")
	if u == nil {
		t.Fatal("non-noProxy should use upstream")
	}
}

func TestEgressUpstream_NoProxyCIDR(t *testing.T) {
	e := newEgressUpstream("manual", "192.168.1.50:8080", []string{"10.0.0.0/8"}, nil)
	if u := e.proxyFor("10.1.2.3:443"); u != nil {
		t.Fatalf("CIDR noProxy should bypass, got %v", u)
	}
	u := e.proxyFor("192.168.1.1:443")
	if u == nil {
		t.Fatal("non-matching CIDR should use upstream")
	}
}

func TestNormalizeProxyURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"192.168.1.50:8080", "http://192.168.1.50:8080"},
		{"http://192.168.1.50:8080", "http://192.168.1.50:8080"},
		{"socks5://192.168.1.50:1080", "socks5://192.168.1.50:1080"},
		{"http=127.0.0.1:12000;https=127.0.0.1:12000", "http://127.0.0.1:12000"},
		{"", ""},
	}
	for _, tt := range tests {
		u := normalizeProxyURL(tt.input)
		if tt.want == "" {
			if u != nil {
				t.Errorf("normalizeProxyURL(%q) = %v, want nil", tt.input, u)
			}
			continue
		}
		if u == nil {
			t.Errorf("normalizeProxyURL(%q) = nil, want %s", tt.input, tt.want)
			continue
		}
		got := u.String()
		if got != tt.want {
			t.Errorf("normalizeProxyURL(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestIsLoopbackName(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"localhost", true},
		{"::1", true},
		{"api.openai.com", false},
		{"192.168.1.1", false},
	}
	for _, tt := range tests {
		if got := isLoopbackName(tt.host); got != tt.want {
			t.Errorf("isLoopbackName(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestNoProxyMatches(t *testing.T) {
	tests := []struct {
		host, port, pattern string
		want                bool
	}{
		{"api.openai.com", "443", "*.openai.com", true},
		{"api.openai.com", "443", "api.openai.com", true},
		{"api.openai.com", "443", "other.com", false},
		{"10.1.2.3", "443", "10.0.0.0/8", true},
		{"192.168.1.1", "443", "10.0.0.0/8", false},
		{"api.openai.com", "443", "*", true},
	}
	for _, tt := range tests {
		if got := noProxyMatches(tt.host, tt.port, tt.pattern); got != tt.want {
			t.Errorf("noProxyMatches(%q, %q, %q) = %v, want %v", tt.host, tt.port, tt.pattern, got, tt.want)
		}
	}
}

func TestEgressUpstream_HttpsScheme(t *testing.T) {
	e := newEgressUpstream("manual", "https://proxy.example.com:443", nil, nil)
	u := e.proxyFor("api.openai.com:443")
	if u == nil {
		t.Fatal("https proxy should return URL")
	}
	if u.Scheme != "https" {
		t.Fatalf("expected https scheme, got %s", u.Scheme)
	}
}

func TestEgressUpstream_Socks5Scheme(t *testing.T) {
	e := newEgressUpstream("manual", "socks5://proxy.example.com:1080", nil, nil)
	u := e.proxyFor("api.openai.com:443")
	if u == nil {
		t.Fatal("socks5 proxy should return URL")
	}
	if u.Scheme != "socks5" {
		t.Fatalf("expected socks5 scheme, got %s", u.Scheme)
	}
}

func TestSplitHostPortLoose(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPort string
	}{
		{"192.168.1.50:8080", "192.168.1.50", "8080"},
		{"192.168.1.50", "192.168.1.50", ""},
		{"[::1]:8080", "::1", "8080"},
		{"", "", ""},
	}
	for _, tt := range tests {
		host, port := splitHostPortLoose(tt.input)
		if host != tt.wantHost || port != tt.wantPort {
			t.Errorf("splitHostPortLoose(%q) = (%q, %q), want (%q, %q)", tt.input, host, port, tt.wantHost, tt.wantPort)
		}
	}
}

func TestPickWinINETProxy(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http=127.0.0.1:12000;https=127.0.0.1:12000", "127.0.0.1:12000"},
		{"http=127.0.0.1:12000", "127.0.0.1:12000"},
		{"https=127.0.0.1:12000", "127.0.0.1:12000"},
		{"ftp=127.0.0.1:21", ""},
	}
	for _, tt := range tests {
		got := pickWinINETProxy(tt.input)
		if got != tt.want {
			t.Errorf("pickWinINETProxy(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEgressUpstream_CacheExpiry(t *testing.T) {
	e := newEgressUpstream("auto", "", nil, nil)
	// First call should detect upstream
	_ = e.autoUpstream()
	// Cache should be set
	e.mu.RLock()
	set := e.cachedSet
	e.mu.RUnlock()
	if !set {
		t.Fatal("cache should be set after first call")
	}
}

func TestHostPortMatches(t *testing.T) {
	tests := []struct {
		full, host, port, self string
		want                   bool
	}{
		{"127.0.0.1:8081", "127.0.0.1", "8081", "127.0.0.1:8081", true},
		{"127.0.0.1:8081", "127.0.0.1", "8081", "127.0.0.1", true},
		{"127.0.0.1:8081", "127.0.0.1", "8081", "192.168.1.1:8081", false},
	}
	for _, tt := range tests {
		if got := hostPortMatches(tt.full, tt.host, tt.port, tt.self); got != tt.want {
			t.Errorf("hostPortMatches(%q, %q, %q, %q) = %v, want %v", tt.full, tt.host, tt.port, tt.self, got, tt.want)
		}
	}
}

func TestEgressUpstream_AutoModeNoUpstream(t *testing.T) {
	e := newEgressUpstream("auto", "", nil, nil)
	u := e.proxyFor("api.openai.com:443")
	// auto mode may detect a system proxy or return nil for direct dial
	// both outcomes are valid in this environment
	_ = u // no assertion needed - auto mode behavior depends on system state
}

func TestEgressUpstream_ConfigureResetsCache(t *testing.T) {
	e := newEgressUpstream("auto", "", nil, nil)
	_ = e.autoUpstream()
	e.mu.RLock()
	if !e.cachedSet {
		t.Fatal("cache should be set")
	}
	e.mu.RUnlock()

	// Reconfigure should reset cache
	e.configure("manual", "192.168.1.50:8080", nil, nil)
	e.mu.RLock()
	if e.cachedSet {
		t.Fatal("cache should be reset after configure")
	}
	e.mu.RUnlock()
}

func TestEgressUpstream_ManualWithPort443(t *testing.T) {
	e := newEgressUpstream("manual", "proxy.example.com:443", nil, nil)
	u := e.proxyFor("api.openai.com:443")
	if u == nil {
		t.Fatal("manual proxy should return URL")
	}
	if u.Hostname() != "proxy.example.com" {
		t.Fatalf("expected proxy.example.com, got %s", u.Hostname())
	}
	if u.Port() != "443" {
		t.Fatalf("expected port 443, got %s", u.Port())
	}
}

func TestNoProxyMatches_Wildcard(t *testing.T) {
	if !noProxyMatches("anyhost.example.com", "443", "*") {
		t.Fatal("wildcard should match any host")
	}
}

func TestNoProxyMatches_PortMismatch(t *testing.T) {
	if noProxyMatches("api.openai.com", "443", "api.openai.com:80") {
		t.Fatal("port mismatch should not match")
	}
}

func TestNoProxyMatches_PortMatch(t *testing.T) {
	if !noProxyMatches("api.openai.com", "443", "api.openai.com:443") {
		t.Fatal("port match should match")
	}
}

func TestIsLoopbackIP(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"::1", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.host)
		if ip == nil {
			if tt.want {
				t.Errorf("net.ParseIP(%q) returned nil", tt.host)
			}
			continue
		}
		if got := ip.IsLoopback(); got != tt.want {
			t.Errorf("IsLoopback(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

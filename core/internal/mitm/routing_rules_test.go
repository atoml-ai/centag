package mitm

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestApplyBackendAuth_InjectsCentagKey(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:20060/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-opencode-upstream")
	applyBackendAuth(req, "llmproxy_centag_egress")
	if got := req.Header.Get("Authorization"); got != "Bearer llmproxy_centag_egress" {
		t.Fatalf("Authorization=%q", got)
	}
	if got := req.Header.Get("X-Original-Authorization"); got != "Bearer sk-opencode-upstream" {
		t.Fatalf("X-Original-Authorization=%q", got)
	}
	// empty token → leave agent auth untouched
	req2, _ := http.NewRequest(http.MethodPost, "http://x/v1", nil)
	req2.Header.Set("Authorization", "Bearer keep-me")
	applyBackendAuth(req2, "")
	if got := req2.Header.Get("Authorization"); got != "Bearer keep-me" {
		t.Fatalf("empty egress must not rewrite, got %q", got)
	}
}

func TestConvertBackendPath_VendorAgnostic(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Already Centag
		{"/v1/chat/completions", "/v1/chat/completions"},
		{"/v1/responses", "/v1/responses"},
		// Strip any prefix before /v1/
		{"/zen/v1/chat/completions", "/v1/chat/completions"},
		{"/zen/v1/responses", "/v1/responses"},
		{"/openai/v1/chat/completions", "/v1/chat/completions"},
		{"/gateway/v1/embeddings", "/v1/embeddings"},
		{"/foo/bar/v1/models", "/v1/models"},
		// Suffix without /v1/
		{"/openai/chat/completions", "/v1/chat/completions"},
		{"/api/responses", "/v1/responses"},
		{"/v1/messages", "/v1/messages"},
	}
	for _, tc := range cases {
		if got := convertBackendPath(tc.in); got != tc.want {
			t.Fatalf("convertBackendPath(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsWhitelistedHost(t *testing.T) {
	s := &Server{}
	s.SetRoutingRules([]string{"opencode.ai", "api.openai.com", "openai.azure.com"}, nil)
	if !s.isWhitelistedHost("opencode.ai:443") {
		t.Fatal("expected opencode.ai:443 whitelisted")
	}
	if !s.isWhitelistedHost("api.openai.com") {
		t.Fatal("expected api.openai.com whitelisted")
	}
	if !s.isWhitelistedHost("my-res.openai.azure.com:443") {
		t.Fatal("expected Azure subdomain whitelisted via dnsDomainIs semantics")
	}
	if s.isWhitelistedHost("github.com:443") {
		t.Fatal("github.com must not be whitelisted → CONNECT tunnel, no MITM")
	}
	if s.isWhitelistedHost("evil.example") {
		t.Fatal("non-list host must not be whitelisted")
	}
	if s.isWhitelistedHost("openai.com") {
		t.Fatal("parent of listed api.openai.com must not match")
	}
}

func TestConvertBackendPath_PreservesV1Beta(t *testing.T) {
	cases := []string{
		"/v1beta/models/gemini-3.1-flash-lite:generateContent",
		"/v1beta/models/gemini-3.1-flash-lite:streamGenerateContent",
		"/v1beta/models",
	}
	for _, in := range cases {
		if got := convertBackendPath(in); got != in {
			t.Fatalf("convertBackendPath(%q)=%q, want %q (v1beta must be preserved)", in, got, in)
		}
	}
}

func TestResponseWriter_WritesValidHeaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	gotCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		rw := &responseWriter{conn: conn}
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Content-Length", "13")
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(`{"ok":true}`))
	}()

	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return
		}
		defer conn.Close()
		var b strings.Builder
		buf := make([]byte, 256)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
			if b.Len() >= 64 {
				break
			}
		}
		gotCh <- b.String()
	}()

	var got string
	select {
	case got = <-gotCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for responseWriter output")
	}

	if !strings.Contains(got, "Content-Type: application/json") {
		t.Fatalf("missing/wrong Content-Type header in raw response: %q", got)
	}
	if !strings.Contains(got, "Content-Length: 13") {
		t.Fatalf("missing/wrong Content-Length header in raw response: %q", got)
	}
	if strings.Contains(got, "[application/json]") {
		t.Fatalf("header value was serialized as slice: %q", got)
	}
}

func TestStripHostPort(t *testing.T) {
	if got := stripHostPort("opencode.ai:443"); got != "opencode.ai" {
		t.Fatalf("got %q", got)
	}
	if got := stripHostPort("API.OpenAI.COM"); got != "api.openai.com" {
		t.Fatalf("got %q", got)
	}
}

func TestShouldRouteToBackend_LLMShapeWithoutExactPathPattern(t *testing.T) {
	s := &Server{}
	s.SetRoutingRules(
		[]string{"opencode.ai"},
		[]string{"/v1/chat/completions"}, // no /zen/v1 entry
	)
	if !s.ShouldRouteToBackend("opencode.ai:443", "/zen/v1/responses") {
		t.Fatal("whitelisted LLM domain + /zen/v1/responses should route without per-agent path pattern")
	}
	if !s.ShouldRouteToBackend("opencode.ai", "/zen/v1/chat/completions") {
		t.Fatal("expected chat completions under vendor prefix to route")
	}
	if s.ShouldRouteToBackend("opencode.ai", "/pricing") {
		t.Fatal("non-LLM path on whitelisted domain must not route to Centag")
	}
	if s.ShouldRouteToBackend("evil.example", "/v1/chat/completions") {
		t.Fatal("non-whitelisted domain must not route")
	}
}

func TestSetRoutingRulesHotUpdate(t *testing.T) {
	s := &Server{}
	s.SetRoutingRules(
		[]string{"api.openai.com", "api.deepseek.com"},
		[]string{"/v1/chat/completions", "/v1/models"},
	)

	if !s.ShouldRouteToBackend("api.openai.com:443", "/v1/chat/completions") {
		t.Fatal("expected openai chat path to route to backend")
	}
	if !s.ShouldRouteToBackend("API.DeepSeek.COM", "/v1/models") {
		t.Fatal("expected case-insensitive domain match")
	}
	if s.ShouldRouteToBackend("opencode.ai:443", "/zen/v1/chat/completions") {
		t.Fatal("opencode.ai should not match before domain is added")
	}

	s.SetRoutingRules(
		[]string{"api.openai.com", "opencode.ai"},
		[]string{"/v1/chat/completions"},
	)

	if !s.ShouldRouteToBackend("opencode.ai:443", "/zen/v1/chat/completions") {
		t.Fatal("expected opencode zen path to route after domain hot update")
	}
	if s.ShouldRouteToBackend("api.deepseek.com", "/v1/chat/completions") {
		t.Fatal("removed domain must stop routing immediately")
	}
}

package mitm

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractProxyAuthToken_BasicPassword(t *testing.T) {
	cred := base64.StdEncoding.EncodeToString([]byte(":llmproxy_abc"))
	r := httptest.NewRequest(http.MethodConnect, "api.openai.com:443", nil)
	r.Header.Set("Proxy-Authorization", "Basic "+cred)
	if got := extractProxyAuthToken(r); got != "llmproxy_abc" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractProxyAuthToken_BasicUser(t *testing.T) {
	cred := base64.StdEncoding.EncodeToString([]byte("llmproxy_user:"))
	r := httptest.NewRequest(http.MethodConnect, "api.openai.com:443", nil)
	r.Header.Set("Proxy-Authorization", "Basic "+cred)
	if got := extractProxyAuthToken(r); got != "llmproxy_user" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractProxyAuthToken_Bearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodConnect, "api.openai.com:443", nil)
	r.Header.Set("Proxy-Authorization", "Bearer llmproxy_xyz")
	if got := extractProxyAuthToken(r); got != "llmproxy_xyz" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteIPIsLoopback(t *testing.T) {
	if !remoteIPIsLoopback("127.0.0.1:12345") {
		t.Fatal("expected loopback")
	}
	if !remoteIPIsLoopback("[::1]:9") {
		t.Fatal("expected ::1 loopback")
	}
	if remoteIPIsLoopback("192.168.1.10:9999") {
		t.Fatal("LAN must not be loopback")
	}
}

func TestRequireClientProxyAuth_LAN(t *testing.T) {
	s := &Server{}
	s.SetClientProxyAuth(true, func(token string) error {
		if token == "good" {
			return nil
		}
		return errors.New("bad")
	})

	// loopback skips
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodConnect, "h:443", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	if !s.requireClientProxyAuth(w, r) {
		t.Fatal("loopback should skip auth")
	}

	// LAN without token → 407
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodConnect, "h:443", nil)
	r.RemoteAddr = "192.168.1.20:5555"
	if s.requireClientProxyAuth(w, r) {
		t.Fatal("LAN without token must fail")
	}
	if w.Code != http.StatusProxyAuthRequired {
		t.Fatalf("status=%d", w.Code)
	}

	// LAN with good token
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodConnect, "h:443", nil)
	r.RemoteAddr = "192.168.1.20:5555"
	cred := base64.StdEncoding.EncodeToString([]byte(":good"))
	r.Header.Set("Proxy-Authorization", "Basic "+cred)
	if !s.requireClientProxyAuth(w, r) {
		t.Fatal("expected success")
	}
}

func TestRequireClientProxyAuth_Disabled(t *testing.T) {
	s := &Server{}
	s.SetClientProxyAuth(false, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodConnect, "h:443", nil)
	r.RemoteAddr = "192.168.1.20:5555"
	if !s.requireClientProxyAuth(w, r) {
		t.Fatal("when disabled, LAN should pass")
	}
}

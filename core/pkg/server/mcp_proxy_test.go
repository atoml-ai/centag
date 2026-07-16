package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleMCPProxy_DenyTargetOutsideAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewMCPProxyHandler()
	h.loadConfig = func() *MCPProxyConfig {
		return &MCPProxyConfig{
			Enabled:        true,
			TimeoutSeconds: 10,
			Targets: map[string]string{
				"allowed": "http://127.0.0.1:9999",
			},
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-MCP-Target", "https://example.com")

	h.HandleMCPProxy(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "allowlist") {
		t.Fatalf("expected allowlist error, got body=%s", w.Body.String())
	}
}

func TestHandleMCPProxy_StreamPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"result\":{\"ok\":true}}\n\n"))
	}))
	defer upstream.Close()

	h := NewMCPProxyHandler()
	h.httpClient = upstream.Client()
	h.loadConfig = func() *MCPProxyConfig {
		return &MCPProxyConfig{
			Enabled:        true,
			TimeoutSeconds: 10,
			Targets: map[string]string{
				"main": upstream.URL,
			},
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-MCP-Target", "main")

	h.HandleMCPProxy(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type=%q want text/event-stream", ct)
	}
	if !strings.Contains(w.Body.String(), "\"ok\":true") {
		t.Fatalf("expected streamed body, got %s", w.Body.String())
	}
}

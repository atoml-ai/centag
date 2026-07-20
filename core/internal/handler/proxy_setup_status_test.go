package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/internal/pac"
	"centag/core/pkg/config"

	"github.com/gin-gonic/gin"
)

func TestServePAC_UsesAdvertiseHostInLANMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 20060},
		SystemProxy: config.SystemProxyConfig{
			ListenPort:      8081,
			ListenAddr:      "0.0.0.0",
			AllowLANClients: true,
			AdvertiseHost:   "192.168.1.50",
			PACEnabled:      true,
			Domains:         []string{"api.openai.com"},
			PathPatterns:    []string{"/v1/models"},
		},
	}
	if err := config.ValidateSystemProxyConfig(&cfg.SystemProxy); err != nil {
		t.Fatal(err)
	}
	pacCfg := &pac.Config{
		ProxyHost:    cfg.SystemProxy.PACProxyHost(),
		ProxyPort:    cfg.SystemProxy.ListenPort,
		Domains:      cfg.SystemProxy.Domains,
		PathPatterns: cfg.SystemProxy.PathPatterns,
	}
	h := NewProxyHandler(nil, pacCfg, cfg)
	body := h.pacGen.Generate()
	if !strings.Contains(body, `PROXY 192.168.1.50:8081`) {
		t.Fatalf("PAC missing advertise PROXY, body=%s", body)
	}
	if strings.Contains(body, `PROXY 127.0.0.1:8081`) {
		t.Fatalf("LAN PAC must not use loopback PROXY, body=%s", body)
	}
}

func TestRefreshPACConfig_UpdatesProxyHost(t *testing.T) {
	h := NewProxyHandler(nil, &pac.Config{
		ProxyHost: "127.0.0.1",
		ProxyPort: 8081,
		Domains:   []string{"api.openai.com"},
	}, nil)
	h.RefreshPACConfig(&pac.Config{ProxyHost: "10.0.0.9", ProxyPort: 8081})
	body := h.pacGen.Generate()
	if !strings.Contains(body, `PROXY 10.0.0.9:8081`) {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "api.openai.com") {
		t.Fatal("domains should be preserved when new config omits them")
	}
}

func TestGetSetupStatus_LocalMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Isolate from developer .env / process env (ResolveSystemProxyEgressAPIKey falls back to these).
	t.Setenv("LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY", "")
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 20060},
		SystemProxy: config.SystemProxyConfig{
			ListenPort:      8081,
			AllowLANClients: false,
			PACEnabled:      true,
			EgressAPIKey:    "", // explicit empty; do not inherit ambient keys
		},
	}
	_ = config.ValidateSystemProxyConfig(&cfg.SystemProxy)
	h := NewProxyHandler(nil, &pac.Config{ProxyHost: "127.0.0.1", ProxyPort: 8081}, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/proxy/setup/status", nil)
	h.GetSetupStatus(c)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["mode"] != "local" {
		t.Fatalf("mode=%v", got["mode"])
	}
	if got["listen_is_loopback"] != true {
		t.Fatalf("loopback=%v", got["listen_is_loopback"])
	}
	if got["pac_url"] != "http://127.0.0.1:20060/api/v1/proxy/pac" {
		t.Fatalf("pac_url=%v", got["pac_url"])
	}
	if got["egress_api_key_configured"] != false {
		t.Fatalf("egress_api_key_configured=%v", got["egress_api_key_configured"])
	}
}

func TestGetSetupStatus_EgressKeyFromConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("LLM_PROXY_SYSTEM_PROXY_EGRESS_API_KEY", "")
	t.Setenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY", "")
	t.Setenv("LLM_PROXY_ADMIN_API_KEY", "")

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 20060},
		SystemProxy: config.SystemProxyConfig{
			ListenPort:   8081,
			PACEnabled:   true,
			EgressAPIKey: "llmproxy_test_egress",
		},
	}
	_ = config.ValidateSystemProxyConfig(&cfg.SystemProxy)
	h := NewProxyHandler(nil, &pac.Config{ProxyHost: "127.0.0.1", ProxyPort: 8081}, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/proxy/setup/status", nil)
	h.GetSetupStatus(c)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["egress_api_key_configured"] != true {
		t.Fatalf("egress_api_key_configured=%v, want true", got["egress_api_key_configured"])
	}
}

func TestGetSetupStatus_LANFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 20060},
		SystemProxy: config.SystemProxyConfig{
			ListenPort:      8081,
			ListenAddr:      "0.0.0.0",
			AllowLANClients: true,
			AdvertiseHost:   "10.0.0.2",
			PACEnabled:      true,
		},
	}
	_ = config.ValidateSystemProxyConfig(&cfg.SystemProxy)
	h := NewProxyHandler(nil, &pac.Config{ProxyHost: "10.0.0.2", ProxyPort: 8081}, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/proxy/setup/status", nil)
	h.GetSetupStatus(c)

	if w.Code != 200 {
		t.Fatalf("status=%d", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["mode"] != "lan" {
		t.Fatalf("mode=%v", got["mode"])
	}
	if got["pac_url"] != "http://10.0.0.2:20060/api/v1/proxy/pac" {
		t.Fatalf("pac_url=%v", got["pac_url"])
	}
	if got["listen_is_loopback"] != false {
		t.Fatalf("listen_is_loopback=%v", got["listen_is_loopback"])
	}
}

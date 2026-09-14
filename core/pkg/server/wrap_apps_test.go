package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"centag/core/pkg/config"

	"github.com/gin-gonic/gin"
)

func doPrepare(t *testing.T, s *Server, id, body string) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/wrap/apps/"+id+"/prepare", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}
	s.PrepareWrapApp(c)
	var m map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &m)
	}
	return w.Code, m
}

// TestPrepareWrapApp covers TC-API-003…009 (T3b).
func TestPrepareWrapApp(t *testing.T) {
	s := &Server{cfg: &config.Config{}}

	t.Run("TC-API-003 pipeline model", func(t *testing.T) {
		code, body := doPrepare(t, s, "opencode", `{"pipeline_id":"transparent"}`)
		if code != http.StatusOK || body["model"] != "centag/transparent" {
			t.Fatalf("code=%d body=%v", code, body)
		}
	})

	t.Run("TC-API-005 unknown id", func(t *testing.T) {
		code, _ := doPrepare(t, s, "nope", `{}`)
		if code != http.StatusNotFound {
			t.Fatalf("code=%d want 404", code)
		}
	})

	t.Run("TC-API-006 no user context default", func(t *testing.T) {
		code, body := doPrepare(t, s, "opencode", ``)
		if code != http.StatusOK || body["model"] != "centag/"+config.DefaultSystemPipelineID {
			t.Fatalf("code=%d body=%v", code, body)
		}
	})

	t.Run("TC-API-008 env has no proxy vars", func(t *testing.T) {
		_, body := doPrepare(t, s, "opencode", `{}`)
		env, _ := body["env"].(map[string]any)
		for _, k := range []string{"HTTPS_PROXY", "HTTP_PROXY", "NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE"} {
			if _, ok := env[k]; ok {
				t.Fatalf("prepare.env must not carry proxy var %s", k)
			}
		}
	})

	t.Run("TC-API-009 client argv ignored", func(t *testing.T) {
		_, body := doPrepare(t, s, "opencode", `{"argv":["rm","-rf","/"]}`)
		argv, _ := body["argv"].([]any)
		if len(argv) == 0 || argv[0] != "opencode" {
			t.Fatalf("argv should come from catalog, got %v", argv)
		}
	})

	t.Run("TC-API-004 backend pinned model", func(t *testing.T) {
		code, body := doPrepare(t, s, "opencode", `{"backend_id":"b1","model":"gpt-4o"}`)
		if code != http.StatusOK || body["model"] != "b1/gpt-4o" {
			t.Fatalf("code=%d body=%v", code, body)
		}
	})

	t.Run("TC-API-011 empty body", func(t *testing.T) {
		code, body := doPrepare(t, s, "opencode", "")
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("code=%d body=%v", code, body)
		}
	})

	t.Run("TC-API-012 malformed json", func(t *testing.T) {
		code, _ := doPrepare(t, s, "opencode", "{not json")
		if code != http.StatusBadRequest {
			t.Fatalf("code=%d want 400", code)
		}
	})

	t.Run("TC-API-007 write_config without key downgraded", func(t *testing.T) {
		code, body := doPrepare(t, s, "opencode", `{"write_config":true}`)
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("code=%d body=%v", code, body)
		}
		warnings, _ := body["warnings"].([]any)
		if len(warnings) == 0 {
			t.Fatal("expected warning when egress key missing")
		}
	})
}

// TestWrapDoctor covers TC-DOC-001…003.
func TestWrapDoctor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(s *Server) (int, map[string]any) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/wrap/doctor", nil)
		s.WrapDoctor(c)
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}

	t.Run("TC-DOC-002 MITM disabled", func(t *testing.T) {
		code, body := run(&Server{cfg: &config.Config{}})
		if code != http.StatusOK || body["ok"] != false {
			t.Fatalf("code=%d body=%v", code, body)
		}
	})

	t.Run("TC-DOC-001 all ready", func(t *testing.T) {
		ca := filepath.Join(t.TempDir(), "ca.crt")
		if err := os.WriteFile(ca, []byte("dummy"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{}
		cfg.SystemProxy.Enabled = true
		cfg.SystemProxy.EgressAPIKey = "llmproxy_test"
		cfg.SystemProxy.CACertPath = ca
		_, body := run(&Server{cfg: cfg})
		if body["ok"] != true {
			t.Fatalf("expected ok, got %v", body)
		}
	})

	t.Run("TC-DOC-003 LAN without egress key", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.SystemProxy.AllowLANClients = true
		cfg.SystemProxy.AdvertiseHost = "10.0.0.1"
		_, body := run(&Server{cfg: cfg})
		if body["ok"] != false {
			t.Fatalf("expected not ok, got %v", body)
		}
	})
}

// TestListWrapApps covers TC-API-001/002.
func TestListWrapApps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("TC-API-001", func(t *testing.T) {
		s := &Server{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/wrap/apps", nil)

		s.ListWrapApps(c)

		if w.Code != http.StatusOK {
			t.Fatalf("status=%d", w.Code)
		}
		var body struct {
			Apps []struct {
				ID         string `json:"id"`
				LaunchMode string `json:"launch_mode"`
			} `json:"apps"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Apps) == 0 {
			t.Fatal("expected non-empty catalog")
		}
		for _, a := range body.Apps {
			if a.ID == "" || a.LaunchMode == "" {
				t.Fatalf("bad entry %#v", a)
			}
		}
	})

	t.Run("TC-API-002", func(t *testing.T) {
		if wrapAppsPayload() == nil {
			t.Fatal("catalog must be non-nil so JSON encodes [] not null")
		}
	})
}

// TestWrapRegression covers TC-REG-001/002: existing presets/run behavior
// must not change with the app-catalog additions.
func TestWrapRegression(t *testing.T) {
	t.Run("TC-REG-001 presets unchanged", func(t *testing.T) {
		ps := wrapPresets()
		if len(ps) == 0 {
			t.Fatal("presets empty")
		}
		found := false
		for _, p := range ps {
			if p.ID == "opencode" && len(p.Argv) > 0 && p.Argv[0] == "opencode" {
				found = true
			}
		}
		if !found {
			t.Fatalf("opencode preset missing/changed: %#v", ps)
		}
	})

	t.Run("TC-REG-002 run command unchanged", func(t *testing.T) {
		got, err := buildWrapRunUserCommand("http://127.0.0.1:20060", "", []string{"opencode"})
		if err != nil {
			t.Fatal(err)
		}
		want := "centag wrap run --server http://127.0.0.1:20060 -- opencode"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

// TestWrapLocalGuard covers TC-AUTH-001…005: loopback bypass, non-loopback
// delegation to proxyAuth (valid/invalid token).
func TestWrapLocalGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	proxyAuthCalls := 0
	proxyAuth := func(c *gin.Context) {
		proxyAuthCalls++
		if c.GetHeader("Authorization") == "Bearer good" {
			c.Next()
			return
		}
		c.AbortWithStatus(http.StatusUnauthorized)
	}
	guard := newWrapLocalGuard(proxyAuth)

	r := gin.New()
	r.GET("/api/v1/wrap/apps", guard, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	cases := []struct {
		name       string
		id         string
		remoteAddr string
		auth       string
		wantStatus int
		wantProxy  bool
	}{
		{"TC-AUTH-001", "loopback no token", "127.0.0.1:1234", "", http.StatusOK, false},
		{"TC-AUTH-002", "non-loopback no token", "192.168.1.9:1234", "", http.StatusUnauthorized, true},
		{"TC-AUTH-003", "non-loopback valid JWT", "192.168.1.9:1234", "Bearer good", http.StatusOK, true},
		{"TC-AUTH-004", "non-loopback invalid token", "192.168.1.9:1234", "Bearer bad", http.StatusUnauthorized, true},
		{"TC-AUTH-005", "A2 admin-password JWT", "192.168.1.9:1234", "Bearer good", http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			before := proxyAuthCalls
			req := httptest.NewRequest(http.MethodGet, "/api/v1/wrap/apps", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("%s: status=%d want %d", tc.name, w.Code, tc.wantStatus)
			}
			if called := proxyAuthCalls > before; called != tc.wantProxy {
				t.Fatalf("%s: proxyAuth called=%v want %v", tc.name, called, tc.wantProxy)
			}
		})
	}
}

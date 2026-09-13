package server

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
	_ "centag/plugins/database/sqlite"

	"github.com/gin-gonic/gin"
)

// mcpTokenTestMu serializes tests that reinitialize the global database
// manager (database.Get() is a process-wide singleton).
var mcpTokenTestMu = make(chan struct{}, 1)

// resetDatabaseForTest reinitializes the global database singleton.  Used
// because database.Init is guarded by sync.Once and prior tests may Close it.
func resetDatabaseForTest(t *testing.T, dbPath string) {
	t.Helper()
	database.ResetForTest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if err := database.Init(ctx, "sqlite", map[string]interface{}{"path": dbPath}); err != nil {
		t.Fatalf("database init: %v", err)
	}
	t.Cleanup(func() { _ = database.Get().Close() })
}

func setupMCPTokenTest(t *testing.T) *ConfigHandler {
	t.Helper()
	mcpTokenTestMu <- struct{}{}
	t.Cleanup(func() { <-mcpTokenTestMu })
	dir := t.TempDir()
	resetDatabaseForTest(t, filepath.Join(dir, "test.db"))
	if err := auth.LoadSecret(context.Background()); err != nil {
		t.Fatalf("jwt secret load: %v", err)
	}
	return &ConfigHandler{}
}

func TestMCPTokenIssueValidateRevoke(t *testing.T) {
	h := setupMCPTokenTest(t)

	if h.MCPObservationTokenActive(context.Background()) {
		t.Fatal("no token should be active initially")
	}
	if h.ValidateMCPObservationToken("anything") {
		t.Fatal("validation must fail before any token is issued")
	}

	tok, err := h.IssueMCPObservationToken(context.Background(), "admin")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
	if !h.MCPObservationTokenActive(context.Background()) {
		t.Fatal("token should be active after issue")
	}
	if !h.ValidateMCPObservationToken(tok) {
		t.Fatal("issued token must validate")
	}

	// 普通 access token（无 purpose=mcp）不得通过
	accessTok, err := auth.IssueAccessToken(1, "admin", "admin", "")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	if h.ValidateMCPObservationToken(accessTok) {
		t.Fatal("ordinary access token must not be accepted as MCP token")
	}

	// 吊销后立即失效
	if err := h.RevokeMCPObservationToken(context.Background()); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if h.ValidateMCPObservationToken(tok) {
		t.Fatal("revoked token must not validate")
	}
	if h.MCPObservationTokenActive(context.Background()) {
		t.Fatal("no token should be active after revoke")
	}

	// 重复吊销（幂等）
	if err := h.RevokeMCPObservationToken(context.Background()); err != nil {
		t.Fatalf("revoke twice should be idempotent: %v", err)
	}
}

func TestMCPTokenReissueRevokesOld(t *testing.T) {
	h := setupMCPTokenTest(t)

	tok1, err := h.IssueMCPObservationToken(context.Background(), "admin")
	if err != nil {
		t.Fatalf("issue 1: %v", err)
	}
	tok2, err := h.IssueMCPObservationToken(context.Background(), "admin")
	if err != nil {
		t.Fatalf("issue 2: %v", err)
	}
	if tok1 == tok2 {
		t.Fatal("tokens should differ")
	}
	if !h.ValidateMCPObservationToken(tok2) {
		t.Fatal("latest token must validate")
	}
	if h.ValidateMCPObservationToken(tok1) {
		t.Fatal("previous token must be revoked on reissue")
	}
}

func TestProxyAuthMCPTokenPathScoped(t *testing.T) {
	h := setupMCPTokenTest(t)
	gin.SetMode(gin.TestMode)

	tok, err := h.IssueMCPObservationToken(context.Background(), "admin")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	allowCfg := &auth.AuthConfig{
		AllowMCPToken: func(path, token string) bool {
			return len(path) >= len("/api/v1/mcp") && path[:len("/api/v1/mcp")] == "/api/v1/mcp" && h.ValidateMCPObservationToken(token)
		},
	}

	cases := []struct {
		name  string
		path  string
		allow bool
	}{
		{"mcp endpoint", "/api/v1/mcp", true},
		{"mcp sse", "/api/v1/mcp/sse", true},
		{"config endpoint", "/api/v1/config", false},
		{"chat completions", "/v1/chat/completions", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", tc.path, nil)
			c.Request.Header.Set("Authorization", "Bearer "+tok)

			mw := auth.ProxyAuthMiddleware(allowCfg)
			mw(c)

			if tc.allow && w.Code != 200 {
				t.Fatalf("expected pass on %s, got %d: %s", tc.path, w.Code, w.Body.String())
			}
			if !tc.allow && w.Code == 200 {
				t.Fatalf("expected reject on %s, got pass", tc.path)
			}
		})
	}
}

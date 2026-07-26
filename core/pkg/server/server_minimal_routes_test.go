package server

import (
	"testing"

	"centag/core/internal/edition"
	"centag/core/pkg/config"

	"github.com/gin-gonic/gin"
)

// TestMinimalRoutes_FetchModelsRegistered ensures FNOS/minimal WebUI can call
// POST /api/v1/backends/fetch-models (was 404 before route parity fix).
func TestMinimalRoutes_FetchModelsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test", Edition: "minimal"},
	}
	config.Set(cfg)
	t.Cleanup(func() { config.Set(nil) })

	srv := NewMinimal(cfg)
	if srv == nil || srv.router == nil {
		t.Fatal("NewMinimal returned nil")
	}
	if srv.edition != edition.Minimal {
		t.Fatalf("edition=%v", srv.edition)
	}

	want := map[string]bool{
		"POST /api/v1/backends/fetch-models": false,
		"POST /v1/responses":                 false,
	}
	for _, r := range srv.router.Routes() {
		key := r.Method + " " + r.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, ok := range want {
		if !ok {
			t.Fatalf("missing route %s", key)
		}
	}
}

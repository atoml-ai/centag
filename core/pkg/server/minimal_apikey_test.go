package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMinimalAPIKeyStore_CreateValidateDelete(t *testing.T) {
	dir := t.TempDir()
	store := newMinimalAPIKeyStore(dir)

	if store.authRequired() {
		t.Fatal("expected auth not required with empty store")
	}

	full, rec, err := store.create("test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if full == "" || rec.ID == "" || rec.Hash == "" || rec.Key == "" {
		t.Fatalf("unexpected record: %+v full=%q", rec, full)
	}
	keys, err := store.list()
	if err != nil || len(keys) != 1 || keys[0].Key != full {
		t.Fatalf("list should return stored full key: keys=%v err=%v", keys, err)
	}
	if !store.authRequired() {
		t.Fatal("expected auth required after create")
	}
	if !store.validateRawKey(full) {
		t.Fatal("expected key to validate")
	}
	if store.validateRawKey("llmproxy_invalid") {
		t.Fatal("invalid key should not validate")
	}

	if err := store.delete(rec.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if store.authRequired() {
		t.Fatal("expected auth not required after delete")
	}
}

func TestMinimalAPIKeyStore_EnvKey(t *testing.T) {
	dir := t.TempDir()
	store := newMinimalAPIKeyStore(dir)
	t.Setenv("CENTAG_PROXY_API_KEY", "my-secret-env-key")
	if !store.authRequired() {
		t.Fatal("env key should require auth")
	}
	if !store.validateRawKey("my-secret-env-key") {
		t.Fatal("env key should validate")
	}
}

func TestMinimalProxyAuthOptionalMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	h := NewMinimalAuthHandler(dir)

	r := gin.New()
	r.GET("/v1/ping", h.ProxyAuthOptionalMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// open when no keys
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("open access: got %d", w.Code)
	}

	full, _, err := h.apiKeys.create("cli")
	if err != nil {
		t.Fatal(err)
	}

	// reject without key
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// accept with key
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	req.Header.Set("Authorization", "Bearer "+full)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("with key: got %d body=%s", w.Code, w.Body.String())
	}

	_ = os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o600)
}

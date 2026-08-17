package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	_ = logger.Init(logger.Config{Level: "error", Output: "stdout"})
	os.Exit(m.Run())
}

func setupModelConfigRouter(h *ModelConfigHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/config/model-variables", h.GetModelVariables)
	router.PUT("/config/model-variables", h.UpdateModelVariables)
	router.DELETE("/config/model-variables/:name", h.DeleteUserVariable)
	return router
}

func setAuthUser(c *gin.Context, uid int64, role string) {
	c.Set("auth_user_id", uid)
	c.Set("auth_role", role)
}

// ─── GetModelVariables ────────────────────────────────────────────────────

func TestModelConfig_Get_SystemVars(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "personal"},
		Proxy:  config.ProxyConfig{DefaultBackendID: "openai", DefaultModel: "gpt-4o"},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{"system.rerank_backend": "cohere"},
			UserVariables:   map[string]string{"my_var": "1"},
		},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/config/model-variables", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			SystemVariables []config.ModelVariableItem `json:"system_variables"`
			UserVariables   []config.ModelVariableItem `json:"user_variables"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Errorf("success = false, want true")
	}
	if len(resp.Data.SystemVariables) == 0 {
		t.Fatal("system_variables empty")
	}
	if len(resp.Data.UserVariables) != 1 {
		t.Errorf("user_variables len = %d, want 1", len(resp.Data.UserVariables))
	}
}

func TestModelConfig_Get_TeamAdmin(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "team"},
		Proxy:  config.ProxyConfig{DefaultBackendID: "openai"},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	// Team admin.
	router.GET("/config/model-variables-admin", func(c *gin.Context) {
		setAuthUser(c, 1, "admin")
		h.GetModelVariables(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/config/model-variables-admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestModelConfig_Get_NilConfig(t *testing.T) {
	config.Set(nil)
	h := NewModelConfigHandler(nil)
	router := setupModelConfigRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/config/model-variables", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

// ─── UpdateModelVariables ─────────────────────────────────────────────────

func TestModelConfig_Update_Personal(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "personal"},
		Proxy:  config.ProxyConfig{DefaultBackendID: "old"},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{},
			UserVariables:   map[string]string{},
		},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	body := `{"variables":{"system.default_backend":"openai","custom_user":"v"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/config/model-variables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if cfg.Proxy.DefaultBackendID != "openai" {
		t.Errorf("Proxy.DefaultBackendID = %q, want openai", cfg.Proxy.DefaultBackendID)
	}
	if cfg.ModelVariables.UserVariables["custom_user"] != "v" {
		t.Errorf("UserVariables[custom_user] = %q, want v", cfg.ModelVariables.UserVariables["custom_user"])
	}
}

func TestModelConfig_Update_RerankStoredInSystemVars(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "personal"},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{},
			UserVariables:   map[string]string{},
		},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	body := `{"variables":{"system.rerank_backend":"cohere","system.rerank_model":"rerank-v3","system.classify_backend":"groq","system.classify_model":"llama-3.1-8b"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/config/model-variables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := cfg.ModelVariables.SystemVariables["system.rerank_backend"]; got != "cohere" {
		t.Errorf("SystemVariables[system.rerank_backend] = %q, want cohere", got)
	}
	if got := cfg.ModelVariables.SystemVariables["system.classify_backend"]; got != "groq" {
		t.Errorf("SystemVariables[system.classify_backend] = %q, want groq", got)
	}
	if got := cfg.ModelVariables.SystemVariables["system.classify_model"]; got != "llama-3.1-8b" {
		t.Errorf("SystemVariables[system.classify_model] = %q, want llama-3.1-8b", got)
	}
	if _, ok := cfg.ModelVariables.UserVariables["system.rerank_backend"]; ok {
		t.Error("rerank vars should not be stored in UserVariables")
	}
	if _, ok := cfg.ModelVariables.UserVariables["system.classify_backend"]; ok {
		t.Error("classify vars should not be stored in UserVariables")
	}
}

func TestModelConfig_Update_InvalidJSON(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "personal"},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/config/model-variables", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ─── DeleteUserVariable ───────────────────────────────────────────────────

func TestModelConfig_Delete_SystemVariableRejected(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "personal"},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{"system.rerank_backend": "cohere"},
		},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/config/model-variables/system.rerank_backend", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestModelConfig_Delete_UserVariable(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Edition: "personal"},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{},
			UserVariables:   map[string]string{"my_var": "v"},
		},
	}
	config.Set(cfg)
	defer config.Set(nil)

	h := NewModelConfigHandler(cfg)
	router := setupModelConfigRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/config/model-variables/my_var", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if _, ok := cfg.ModelVariables.UserVariables["my_var"]; ok {
		t.Error("my_var should have been deleted")
	}
}

// ─── mergeVars ────────────────────────────────────────────────────────────

func TestMergeVars(t *testing.T) {
	system := []config.ModelVariableItem{
		{Name: "system.default_backend", Value: "openai"},
		{Name: "system.default_model", Value: "gpt-4o"},
	}
	user := map[string]string{
		"system.default_backend": "ollama", // override
		"extra":                  "1",
	}

	got := mergeVars(system, user)

	byName := map[string]config.ModelVariableItem{}
	for _, it := range got {
		byName[it.Name] = it
	}

	if byName["system.default_backend"].Value != "ollama" {
		t.Errorf("merged default_backend = %q, want ollama (user override)", byName["system.default_backend"].Value)
	}
	if byName["system.default_model"].Value != "gpt-4o" {
		t.Errorf("merged default_model = %q, want gpt-4o", byName["system.default_model"].Value)
	}
	if byName["extra"].Value != "1" {
		t.Errorf("merged extra = %q, want 1", byName["extra"].Value)
	}
	// System vars keep their position first, extras appended.
	if len(got) != 3 {
		t.Errorf("merged len = %d, want 3", len(got))
	}
}

func TestMergeVars_EmptyUser(t *testing.T) {
	system := []config.ModelVariableItem{
		{Name: "system.default_backend", Value: "openai"},
	}
	got := mergeVars(system, nil)
	if len(got) != 1 || got[0].Value != "openai" {
		t.Errorf("mergeVars(system, nil) = %+v, want single item openai", got)
	}
}

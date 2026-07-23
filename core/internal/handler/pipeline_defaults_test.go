package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/pkg/config"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPipelineDefaultsHandler_GetDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			DefaultMode: "smart-scheduling",
		},
	}

	router := gin.New()
	handler := NewPipelineDefaultsHandler(cfg, nil)
	router.GET("/pipeline/defaults", handler.GetDefaults)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pipeline/defaults", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "smart-scheduling", resp["default_pipeline_id"])
	assert.Equal(t, true, resp["allow_user_override"])
}

func TestPipelineDefaultsHandler_GetDefaults_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler := NewPipelineDefaultsHandler(nil, nil)
	router.GET("/pipeline/defaults", handler.GetDefaults)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pipeline/defaults", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, config.DefaultSystemPipelineID, resp["default_pipeline_id"])
}

func TestPipelineDefaultsHandler_UpdateDefaults_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler := NewPipelineDefaultsHandler(&config.Config{}, nil)
	router.PUT("/pipeline/defaults", handler.UpdateDefaults)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/pipeline/defaults", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineDefaultsHandler_UpdateDefaults_InvalidPipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			PipelineConfig: &config.PipelineConfig{},
		},
	}
	registry := pipeline.NewPipelineRegistry()

	router := gin.New()
	handler := NewPipelineDefaultsHandler(cfg, registry)
	router.PUT("/pipeline/defaults", handler.UpdateDefaults)

	reqBody := map[string]string{"default_pipeline_id": "nonexistent-pipeline"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/pipeline/defaults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid pipeline_id")
}

func TestPipelineDefaultsHandler_UpdateDefaults_DisallowedPipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			PipelineConfig: &config.PipelineConfig{},
		},
	}
	registry := pipeline.NewPipelineRegistry()
	// Register a pipeline that is disallowed as default (cache-only)
	registry.Register(&pipeline.AgentPatternPipeline{
		ID:   "cache-hit",
		Name: "Cache Hit",
	})

	router := gin.New()
	handler := NewPipelineDefaultsHandler(cfg, registry)
	router.PUT("/pipeline/defaults", handler.UpdateDefaults)

	reqBody := map[string]string{"default_pipeline_id": "cache-hit"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/pipeline/defaults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "cannot be used as system default")
}

func TestPipelineDefaultsHandler_isAllowedAsDefault(t *testing.T) {
	handler := &PipelineDefaultsHandler{}

	tests := []struct {
		pipelineID string
		expected   bool
	}{
		{"smart-scheduling", true},
		{"direct-backend", true},
		{"audit-mode", true},
		{"router-mode", true},
		{"translate-mode", true},
		{"aggregator-mode", true},
		{"transparent-proxy", true},
		{"transparent-fast", true},
		{"fixed-egress", true},
		{"cache-hit", false},
		{"cache-mode", false},
	}

	for _, tt := range tests {
		t.Run(tt.pipelineID, func(t *testing.T) {
			result := handler.isAllowedAsDefault(tt.pipelineID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipelineDefaultsHandler_isValidPipelineID(t *testing.T) {
	registry := pipeline.NewPipelineRegistry()
	registry.Register(&pipeline.AgentPatternPipeline{
		ID:   "smart-scheduling",
		Name: "Smart Scheduling",
	})
	registry.Register(&pipeline.AgentPatternPipeline{
		ID:   "audit-mode",
		Name: "Audit Mode",
	})

	handler := &PipelineDefaultsHandler{pipelineRegistry: registry}

	tests := []struct {
		pipelineID string
		expected   bool
	}{
		{"smart-scheduling", true},
		{"audit-mode", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.pipelineID, func(t *testing.T) {
			result := handler.isValidPipelineID(tt.pipelineID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipelineDefaultsHandler_isValidPipelineID_NilRegistry(t *testing.T) {
	handler := &PipelineDefaultsHandler{pipelineRegistry: nil}
	assert.False(t, handler.isValidPipelineID("smart-scheduling"))
}

func TestPipelineDefaultsHandler_getDefaultPipelineID(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: config.DefaultSystemPipelineID,
		},
		{
			name: "config with pipeline",
			cfg: &config.Config{
				Proxy: config.ProxyConfig{
					PipelineConfig: &config.PipelineConfig{
						DefaultPipeline: "audit-mode",
					},
				},
			},
			expected: "audit-mode",
		},
		{
			name: "config with default_mode",
			cfg: &config.Config{
				Proxy: config.ProxyConfig{
					DefaultMode: "direct-backend",
				},
			},
			expected: "direct-backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &PipelineDefaultsHandler{cfg: tt.cfg}
			result := handler.getDefaultPipelineID()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipelineDefaultsHandler_getPipelineConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		handler := &PipelineDefaultsHandler{cfg: nil}
		assert.Nil(t, handler.getPipelineConfig())
	})

	t.Run("nil PipelineConfig", func(t *testing.T) {
		handler := &PipelineDefaultsHandler{cfg: &config.Config{}}
		assert.Nil(t, handler.getPipelineConfig())
	})

	t.Run("with PipelineConfig", func(t *testing.T) {
		pc := &config.PipelineConfig{DefaultPipeline: "test"}
		handler := &PipelineDefaultsHandler{cfg: &config.Config{
			Proxy: config.ProxyConfig{PipelineConfig: pc},
		}}
		result := handler.getPipelineConfig()
		assert.Equal(t, pc, result)
	})
}

func TestPipelineDefaultsHandler_isAllowUserOverride(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		handler := &PipelineDefaultsHandler{cfg: nil}
		assert.True(t, handler.isAllowUserOverride())
	})

	t.Run("override enabled", func(t *testing.T) {
		handler := &PipelineDefaultsHandler{cfg: &config.Config{
			Proxy: config.ProxyConfig{
				PipelineConfig: &config.PipelineConfig{AllowUserOverride: true},
			},
		}}
		assert.True(t, handler.isAllowUserOverride())
	})

	t.Run("override disabled", func(t *testing.T) {
		handler := &PipelineDefaultsHandler{cfg: &config.Config{
			Proxy: config.ProxyConfig{
				PipelineConfig: &config.PipelineConfig{AllowUserOverride: false},
			},
		}}
		assert.False(t, handler.isAllowUserOverride())
	})
}

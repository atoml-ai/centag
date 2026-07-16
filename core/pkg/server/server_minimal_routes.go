package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"centag/core/internal/auth"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	pluginregistry "centag/core/pkg/plugin/registry"

	"github.com/gin-gonic/gin"
)

// setupMinimalRoutes registers routes for the minimal edition (file config + lite WebUI).
func (s *Server) setupMinimalRoutes(configHandler *MinimalConfigHandler, pluginRegistryAPI *pluginregistry.Handler, authHandler *MinimalAuthHandler) {
	_ = configHandler // file-based defaults still loaded at startup; WebUI uses /api/v1 pipelines

	staticDir := os.Getenv("STATIC_PATH")
	if staticDir == "" {
		staticDir = "./static"
	}
	if abs, err := filepath.Abs(staticDir); err == nil {
		staticDir = abs
	}

	serveIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		indexPath := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			c.String(http.StatusNotFound, "WebUI not found. Build frontend into %s", staticDir)
			return
		}
		c.File(indexPath)
	}

	// Custom static + SPA fallback (Gin Static returns 404 without hitting NoRoute).
	// Do NOT also register GET /static/ — it conflicts with /*filepath in gin's tree.
	s.router.GET("/static/*filepath", func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		if rel == "" || rel == "." {
			serveIndex(c)
			return
		}
		full := filepath.Join(staticDir, rel)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			c.File(full)
			return
		}
		serveIndex(c)
	})

	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/static/")
	})

	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") || path == "/health" || path == "/ping" {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found"})
			return
		}
		if strings.HasPrefix(path, "/static/") {
			rel := strings.TrimPrefix(path, "/static/")
			if rel == "" {
				serveIndex(c)
				return
			}
			full := filepath.Join(staticDir, rel)
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				c.File(full)
				return
			}
			serveIndex(c)
			return
		}
		serveIndex(c)
	})

	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "edition": "minimal", "service": "centag"})
	})
	s.router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Public status (edition for WebUI bootstrap)
	s.router.GET("/api/v1/status", s.handleStatus)

	// Auth (password setup / login)
	if authHandler != nil {
		authHandler.RegisterRoutes(s.router)
	}

	if pluginRegistryAPI != nil {
		pluginRegistryAPI.RegisterRoutes(s.router.Group("/api/v1"))
	}

	// LLM proxy — open when no API keys configured; otherwise require Bearer key/JWT
	v1 := s.router.Group("/v1")
	if authHandler != nil {
		v1.Use(authHandler.ProxyAuthOptionalMiddleware())
	}
	{
		v1.POST("/chat/completions", s.proxyHandler.HandleChatCompletions)
		v1.GET("/models", s.proxyHandler.ListModels)
		v1.GET("/backends", s.proxyHandler.ListBackends)
		v1.POST("/messages", s.proxyHandler.HandleChatCompletions)
		v1.POST("/completions", s.proxyHandler.HandleChatCompletions)
		v1.POST("/embeddings", s.proxyHandler.HandleChatCompletions)
	}

	// Protected management APIs (WebUI)
	protected := s.router.Group("/api/v1")
	protected.Use(auth.JWTMiddleware())
	{
		backends := protected.Group("/backends")
		{
			backends.GET("", s.backendHandler.ListBackends)
			backends.GET("/types", s.backendHandler.ListBackendTypes)
			backends.GET("/export", s.backendHandler.ExportBackends)
			backends.GET("/:id", s.backendHandler.GetBackend)
			backends.GET("/:id/models", s.backendHandler.GetModels)
			backends.POST("", s.backendHandler.CreateBackend)
			backends.PUT("/:id", s.backendHandler.UpdateBackend)
			backends.DELETE("/:id", s.backendHandler.DeleteBackend)
			backends.POST("/test", s.backendHandler.TestConnection)
			backends.POST("/:id/probe", s.backendHandler.ProbeBackend)
			backends.POST("/import", s.backendHandler.ImportBackends)
		}

		if s.pipelineHandler != nil {
			s.pipelineHandler.RegisterPipelineRoutes(protected)
		}

		if s.pipelineDefaultsHandler != nil {
			defaults := protected.Group("/pipeline/defaults")
			{
				defaults.GET("", s.pipelineDefaultsHandler.GetDefaults)
				defaults.PUT("", s.pipelineDefaultsHandler.UpdateDefaults)
			}
		}

		// Proxy config (default backend / model)
		protected.GET("/config/proxy", s.handleGetProxyConfig)
		protected.PUT("/config/proxy", s.handleSaveProxyConfig)

		// Lightweight usage stub for overview (P3 can enrich)
		if s.tokenUsageHandler != nil {
			userAPI := protected.Group("/user")
			userAPI.GET("/token-usage", s.tokenUsageHandler.GetUserUsage)
			userAPI.GET("/token-usage/daily", s.tokenUsageHandler.GetDailyUsage)
			userAPI.GET("/token-usage/models", s.tokenUsageHandler.GetModelStats)
			userAPI.GET("/token-usage/backends", s.tokenUsageHandler.GetBackendStats)
		} else {
			protected.GET("/user/token-usage", s.handleMinimalTokenUsage)
			protected.GET("/user/token-usage/daily", s.handleMinimalTokenUsageDaily)
		}
	}

	logger.Info("Minimal edition routes registered (lite WebUI + password auth)")
}

func (s *Server) handleMinimalTokenUsage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_tokens":  0,
			"total_cost":    0,
			"request_count": 0,
			"note":          "minimal in-memory metering not yet enabled",
		},
	})
}

func (s *Server) handleMinimalTokenUsageDaily(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
}

// handleGetProxyConfig returns the current proxy config (default backend/model).
func (s *Server) handleGetProxyConfig(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "config not initialized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"default_backend_id": cfg.Proxy.DefaultBackendID,
			"default_model":      cfg.Proxy.DefaultModel,
		},
	})
}

// handleSaveProxyConfig updates the proxy config (default backend/model).
func (s *Server) handleSaveProxyConfig(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "config not initialized"})
		return
	}

	var req struct {
		DefaultBackendID *string `json:"default_backend_id"`
		DefaultModel     *string `json:"default_model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.DefaultBackendID != nil {
		cfg.Proxy.DefaultBackendID = *req.DefaultBackendID
	}
	if req.DefaultModel != nil {
		cfg.Proxy.DefaultModel = *req.DefaultModel
	}

	logger.Infof("[ProxyConfig] Updated default_backend_id=%q default_model=%q", cfg.Proxy.DefaultBackendID, cfg.Proxy.DefaultModel)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "proxy config saved",
		"data": gin.H{
			"default_backend_id": cfg.Proxy.DefaultBackendID,
			"default_model":      cfg.Proxy.DefaultModel,
		},
	})
}

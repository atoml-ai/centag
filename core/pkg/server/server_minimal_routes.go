package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"centag/core/internal/auth"
	"centag/core/internal/middleware"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	pluginregistry "centag/core/pkg/plugin/registry"
	"centag/core/pkg/useraccess"

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
		data, err := os.ReadFile(indexPath)
		if err != nil {
			c.String(http.StatusNotFound, "WebUI not found. Build frontend into %s", staticDir)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, strings.Replace(string(data), "<html", `<html data-edition="minimal"`, 1))
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
	// 与完整版对齐：解析 pipeline.<id> / 快捷码，并在仅流水线 ID 时回写 default_model
	if s.modeManager != nil {
		v1.Use(middleware.ProxyModeMiddlewareGin(s.modeManager, s.sessionStore))
	}
	{
		v1.POST("/chat/completions", s.proxyHandler.HandleChatCompletions)
		v1.GET("/models", s.proxyHandler.ListModels)
		v1.GET("/backends", s.proxyHandler.ListBackends)
		v1.POST("/messages", s.proxyHandler.HandleChatCompletions)
		// OpenCode / Codex wire_api=responses（需编译进 protocol_openairesponses）
		v1.POST("/responses", s.proxyHandler.HandleChatCompletions)
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
			// 静态路径必须在 /:id 之前，否则会被当成 id（与完整版 setupRoutes 对齐）
			backends.POST("/fetch-models", s.backendHandler.FetchModels)
			backends.POST("/test", s.backendHandler.TestConnection)
			backends.POST("/probe-all", s.backendHandler.ProbeAllBackends)
			backends.POST("/probe-all-sse", s.backendHandler.ProbeAllBackendsSSE)
			backends.POST("/import", s.backendHandler.ImportBackends)
			backends.GET("/:id", s.backendHandler.GetBackend)
			backends.GET("/:id/models", s.backendHandler.GetModels)
			backends.POST("", s.backendHandler.CreateBackend)
			backends.PUT("/:id", s.backendHandler.UpdateBackend)
			backends.DELETE("/:id", s.backendHandler.DeleteBackend)
			backends.POST("/:id/probe", s.backendHandler.ProbeBackend)

			// 账户池 CRUD（与完整版 setupRoutes 对齐）
			backends.GET("/:id/accounts", s.backendHandler.ListBackendAccounts)
			backends.PUT("/:id/account-pool", s.backendHandler.UpdateAccountPool)
			backends.GET("/:id/accounts/stats", s.backendHandler.GetAccountPoolStats)
			backends.GET("/:id/accounts/:accountId", s.backendHandler.GetBackendAccount)
			backends.POST("/:id/accounts", s.backendHandler.CreateBackendAccount)
			backends.PUT("/:id/accounts/:accountId", s.backendHandler.UpdateBackendAccount)
			backends.DELETE("/:id/accounts/:accountId", s.backendHandler.DeleteBackendAccount)
			backends.POST("/:id/accounts/:accountId/reset-breaker", s.backendHandler.ResetAccountBreaker)
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

		// 模型变量配置
		if s.modelConfigHandler != nil {
			configGroup := protected.Group("/config")
			{
				configGroup.GET("/model-variables", s.modelConfigHandler.GetModelVariables)
				configGroup.PUT("/model-variables", s.modelConfigHandler.UpdateModelVariables)
				configGroup.DELETE("/model-variables/:name", s.modelConfigHandler.DeleteUserVariable)
			}
		}

		userAPI := protected.Group("/user")
		if s.tokenUsageHandler != nil {
			// Process-wide totals so proxy traffic (often user_id=0) still appears in WebUI.
			userAPI.GET("/token-usage", s.tokenUsageHandler.GetAggregateUsage)
			userAPI.GET("/token-usage/daily", s.tokenUsageHandler.GetDailyUsage)
			userAPI.GET("/token-usage/models", s.tokenUsageHandler.GetModelStats)
			userAPI.GET("/token-usage/backends", s.tokenUsageHandler.GetBackendStats)
			// 计量计价明细（前端 useUsageTotals / UsageMetricsSummary 等共用）
			userAPI.GET("/usage", s.tokenUsageHandler.GetUsageBreakdown)
			userAPI.GET("/usage/sessions", s.tokenUsageHandler.GetSessionsUsage)
			userAPI.GET("/usage/self-limit", s.tokenUsageHandler.GetSelfLimit)
		} else {
			userAPI.GET("/token-usage", s.handleMinimalTokenUsage)
			userAPI.GET("/token-usage/daily", s.handleMinimalTokenUsageDaily)
		}

		// Personal 计费配置只读 API
		if s.personalBillingHandler != nil {
			s.personalBillingHandler.RegisterPersonalBillingRoutes(userAPI)
		}

		if s.billingRulesHandler != nil {
			billingRules := protected.Group("/admin/billing/rules")
			{
				billingRules.GET("", s.billingRulesHandler.ListRules)
				billingRules.POST("", s.billingRulesHandler.CreateRule)
				billingRules.PUT("/:id", s.billingRulesHandler.UpdateRule)
				billingRules.DELETE("/:id", s.billingRulesHandler.DeleteRule)
				billingRules.POST("/import", s.billingRulesHandler.ImportRules)
				billingRules.GET("/export", s.billingRulesHandler.ExportRules)
			}
		}
		if s.costHandler != nil {
			protected.GET("/admin/cost/summary", s.costHandler.GetSummary)
		}
		if s.conversationHandler != nil {
			convs := userAPI.Group("/conversations")
			{
				convs.GET("/sessions", s.conversationHandler.ListSessions)
				convs.GET("/sessions/:id", s.conversationHandler.GetSession)
				convs.GET("/sessions/:id/messages", s.conversationHandler.ListMessages)
				convs.GET("/categories", s.conversationHandler.ListCategories)
				convs.DELETE("/sessions/:id", s.conversationHandler.DeleteSession)
				convs.POST("/sessions/delete", s.conversationHandler.DeleteSessions)
				convs.POST("/sessions/:id/messages/delete", s.conversationHandler.DeleteMessages)
			}
			convsTop := protected.Group("/conversations")
			{
				convsTop.GET("/sessions", s.conversationHandler.ListSessions)
				convsTop.GET("/sessions/:id", s.conversationHandler.GetSession)
				convsTop.GET("/sessions/:id/messages", s.conversationHandler.ListMessages)
				convsTop.GET("/categories", s.conversationHandler.ListCategories)
				convsTop.DELETE("/sessions/:id", s.conversationHandler.DeleteSession)
				convsTop.POST("/sessions/delete", s.conversationHandler.DeleteSessions)
				convsTop.POST("/sessions/:id/messages/delete", s.conversationHandler.DeleteMessages)
			}
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
// Team 普通用户：返回个人默认（user_config.proxy_settings），缺省时回落系统默认。
func (s *Server) handleGetProxyConfig(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "config not initialized"})
		return
	}
	backendID := cfg.Proxy.DefaultBackendID
	model := cfg.Proxy.DefaultModel
	scope := "system"
	if user := s.loadAccessUser(c); user != nil {
		if ub, um, ok := loadUserProxyDefaults(c.Request.Context(), user.ID); ok {
			if ub != "" {
				backendID = ub
			}
			if um != "" {
				model = um
			}
			scope = "user"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"default_backend_id":    backendID,
			"default_model":         model,
			"fallback_backend_id":   cfg.Proxy.FallbackBackendID,
			"fallback_model":        cfg.Proxy.FallbackModel,
			"response_trace_banner": cfg.Proxy.ResponseTraceBanner,
			"scope":                 scope,
		},
	})
}

// handleSaveProxyConfig updates the proxy config (default backend/model).
// Team 普通用户：写入个人 user_config（不改系统默认）；管理员/Personal：写系统配置。
func (s *Server) handleSaveProxyConfig(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "config not initialized"})
		return
	}

	var req struct {
		DefaultBackendID    *string `json:"default_backend_id"`
		DefaultModel        *string `json:"default_model"`
		FallbackBackendID   *string `json:"fallback_backend_id"`
		FallbackModel       *string `json:"fallback_model"`
		ResponseTraceBanner *bool   `json:"response_trace_banner"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if user := s.loadAccessUser(c); user != nil {
		s.saveUserProxyConfig(c, user, req.DefaultBackendID, req.DefaultModel)
		return
	}

	if req.DefaultBackendID != nil {
		cfg.Proxy.DefaultBackendID = strings.TrimSpace(*req.DefaultBackendID)
	}
	if req.DefaultModel != nil {
		cfg.Proxy.DefaultModel = strings.TrimSpace(*req.DefaultModel)
	}
	if req.FallbackBackendID != nil {
		cfg.Proxy.FallbackBackendID = strings.TrimSpace(*req.FallbackBackendID)
	}
	if req.FallbackModel != nil {
		cfg.Proxy.FallbackModel = strings.TrimSpace(*req.FallbackModel)
	}
	if req.ResponseTraceBanner != nil {
		cfg.Proxy.ResponseTraceBanner = *req.ResponseTraceBanner
	}
	if strings.TrimSpace(cfg.Proxy.DefaultModel) == "" && strings.TrimSpace(cfg.Proxy.DefaultBackendID) != "" {
		if filled := s.preferredModelForBackend(cfg.Proxy.DefaultBackendID); filled != "" {
			cfg.Proxy.DefaultModel = filled
			logger.Infof("[ProxyConfig] Auto-filled default_model=%q from backend %q", filled, cfg.Proxy.DefaultBackendID)
		}
	}

	if err := config.PersistProxyConfig(c.Request.Context(), cfg.Proxy); err != nil {
		logger.Errorf("Failed to persist proxy config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to persist proxy config: " + err.Error()})
		return
	}

	// 联动依赖系统默认后端的运行时组件：刷新 broker 默认 LLM 目标并
	// 幂等重建 centag-ops-router（节点 backend/model 为注册期快照）。
	if s.onProxyConfigChanged != nil {
		s.onProxyConfigChanged(cfg.Proxy.DefaultBackendID, cfg.Proxy.DefaultModel)
	}

	logger.Infof("[ProxyConfig] Updated default_backend_id=%q default_model=%q fallback_backend_id=%q fallback_model=%q response_trace_banner=%v",
		cfg.Proxy.DefaultBackendID, cfg.Proxy.DefaultModel, cfg.Proxy.FallbackBackendID, cfg.Proxy.FallbackModel, cfg.Proxy.ResponseTraceBanner)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "proxy config saved",
		"data": gin.H{
			"default_backend_id":    cfg.Proxy.DefaultBackendID,
			"default_model":         cfg.Proxy.DefaultModel,
			"fallback_backend_id":   cfg.Proxy.FallbackBackendID,
			"fallback_model":        cfg.Proxy.FallbackModel,
			"response_trace_banner": cfg.Proxy.ResponseTraceBanner,
			"scope":                 "system",
		},
	})
}

func loadUserProxyDefaults(ctx context.Context, userID int64) (backendID, model string, ok bool) {
	if !database.IsInitialized() || userID <= 0 {
		return "", "", false
	}
	uc, err := database.Get().UserConfigStore().Get(ctx, userID)
	if err != nil || uc == nil || strings.TrimSpace(uc.ProxySettings) == "" {
		return "", "", false
	}
	var ps struct {
		DefaultBackendID string `json:"default_backend_id"`
		DefaultModel     string `json:"default_model"`
	}
	if json.Unmarshal([]byte(uc.ProxySettings), &ps) != nil {
		return "", "", false
	}
	backendID = strings.TrimSpace(ps.DefaultBackendID)
	model = strings.TrimSpace(ps.DefaultModel)
	if backendID == "" && model == "" {
		return "", "", false
	}
	return backendID, model, true
}

func (s *Server) saveUserProxyConfig(c *gin.Context, user *database.User, backendID, model *string) {
	if user == nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "user required"})
		return
	}
	ctx := c.Request.Context()
	uc, err := database.Get().UserConfigStore().Get(ctx, user.ID)
	if err != nil || uc == nil {
		uc = database.DefaultUserConfig(user.ID)
	}
	var ps struct {
		DefaultBackendID string `json:"default_backend_id"`
		DefaultModel     string `json:"default_model"`
	}
	_ = json.Unmarshal([]byte(uc.ProxySettings), &ps)
	if backendID != nil {
		ps.DefaultBackendID = strings.TrimSpace(*backendID)
	}
	if model != nil {
		ps.DefaultModel = strings.TrimSpace(*model)
	}
	if ps.DefaultBackendID != "" && s.backendHandler != nil && s.backendHandler.backendManager != nil {
		list := s.backendHandler.backendManager.List()
		filtered := useraccess.FilterBackendsFor(user, list, policyForUser(ctx, user))
		allowed := false
		for _, b := range filtered {
			if b != nil && b.ID == ps.DefaultBackendID {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "backend not allowed for this user: " + ps.DefaultBackendID})
			return
		}
	}
	raw, _ := json.Marshal(ps)
	uc.ProxySettings = string(raw)
	if err := database.Get().UserConfigStore().Upsert(ctx, uc); err != nil {
		logger.Errorf("save user proxy config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to save user proxy config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "user proxy config saved",
		"data": gin.H{
			"default_backend_id": ps.DefaultBackendID,
			"default_model":      ps.DefaultModel,
			"scope":              "user",
		},
	})
}

// preferredModelForBackend resolves ProbeModel / first supported model for a backend id.
func (s *Server) preferredModelForBackend(backendID string) string {
	if s == nil || s.backendManager == nil || strings.TrimSpace(backendID) == "" {
		return ""
	}
	b, err := s.backendManager.Get(backendID)
	if err != nil || b == nil {
		return ""
	}
	return backend.PreferredDefaultModel(b)
}

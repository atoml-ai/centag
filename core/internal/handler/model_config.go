package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"centag/core/internal/auth"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ModelConfigHandler 模型变量配置处理器
type ModelConfigHandler struct {
	cfg *config.Config
}

// NewModelConfigHandler 创建模型变量配置处理器
func NewModelConfigHandler(cfg *config.Config) *ModelConfigHandler {
	return &ModelConfigHandler{cfg: cfg}
}

// GetModelVariables 获取所有模型变量
// GET /api/v1/config/model-variables
func (h *ModelConfigHandler) GetModelVariables(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	isAdmin := auth.IsAdmin(c)
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "config not initialized"})
		return
	}
	edition := cfg.Server.Edition

	// Team admin reads system-wide defaults from config.
	if edition == "team" && isAdmin {
		systemVars := config.ListSystemVariables(cfg)
		userVars := config.ListUserVariables(cfg)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"system_variables": systemVars,
				"user_variables":   userVars,
			},
		})
		return
	}

	// Team user: merge system vars (read-only from config) with per-user overrides.
	if edition == "team" && uid > 0 {
		systemVars := config.ListSystemVariables(cfg)
		userVars := h.getUserModelVars(uid)

		// User overrides take precedence over system for display.
		merged := mergeVars(systemVars, userVars)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"system_variables": merged,
				"user_variables":   []config.ModelVariableItem{},
			},
		})
		return
	}

	// Personal / minimal: read system-wide config.
	systemVars := config.ListSystemVariables(cfg)
	userVars := config.ListUserVariables(cfg)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"system_variables": systemVars,
			"user_variables":   userVars,
		},
	})
}

// UpdateModelVariables 更新模型变量
// PUT /api/v1/config/model-variables
func (h *ModelConfigHandler) UpdateModelVariables(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	isAdmin := auth.IsAdmin(c)
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "config not initialized"})
		return
	}
	edition := cfg.Server.Edition

	var req struct {
		Variables map[string]string `json:"variables"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Variables == nil {
		req.Variables = map[string]string{}
	}

	// Team admin writes to system-wide config.
	if edition == "team" && isAdmin {
		h.updateSystemVariables(req.Variables)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "model variables updated"})
		return
	}

	// Team user writes to per-user overrides.
	if edition == "team" && uid > 0 {
		h.setUserModelVars(uid, req.Variables)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "model variables updated"})
		return
	}

	// Personal / minimal writes to system-wide config.
	h.updateSystemVariables(req.Variables)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "model variables updated"})
}

// DeleteUserVariable 删除用户自定义变量
// DELETE /api/v1/config/model-variables/:name
func (h *ModelConfigHandler) DeleteUserVariable(c *gin.Context) {
	uid, _ := auth.GetUserID(c)
	isAdmin := auth.IsAdmin(c)
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "config not initialized"})
		return
	}
	edition := cfg.Server.Edition

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "variable name is required"})
		return
	}

	// Only allow deleting user/custom variables.
	if strings.HasPrefix(name, "system.") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete system variables"})
		return
	}

	// Team admin deletes from system-wide config.
	if edition == "team" && isAdmin {
		h.deleteSystemUserVariable(name)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "variable deleted"})
		return
	}

	// Team user deletes from per-user overrides.
	if edition == "team" && uid > 0 {
		h.deleteUserModelVar(uid, name)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "variable deleted"})
		return
	}

	// Personal / minimal deletes from system-wide config.
	h.deleteSystemUserVariable(name)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "variable deleted"})
}

// ─── System-wide helpers ────────────────────────────────────────────────

func (h *ModelConfigHandler) updateSystemVariables(vars map[string]string) {
	cfg := config.Get()
	if cfg == nil {
		return
	}

	for k, v := range vars {
		switch k {
		case "system.default_backend":
			cfg.Proxy.DefaultBackendID = v
		case "system.default_model":
			cfg.Proxy.DefaultModel = v
		case "system.fallback_backend":
			cfg.Proxy.FallbackBackendID = v
		case "system.fallback_model":
			cfg.Proxy.FallbackModel = v
		case "system.embedding_backend":
			cfg.Embedding.BackendID = v
		case "system.embedding_model":
			cfg.Embedding.Model = v
		case "system.rerank_backend", "system.rerank_model",
			"system.classify_backend", "system.classify_model":
			if cfg.ModelVariables.SystemVariables == nil {
				cfg.ModelVariables.SystemVariables = map[string]string{}
			}
			cfg.ModelVariables.SystemVariables[k] = v
		default:
			if cfg.ModelVariables.UserVariables == nil {
				cfg.ModelVariables.UserVariables = map[string]string{}
			}
			cfg.ModelVariables.UserVariables[k] = v
		}
	}

	if err := config.SaveConfig(cfg); err != nil {
		logger.Errorf("Failed to save model variables config: %v", err)
	}
}

func (h *ModelConfigHandler) deleteSystemUserVariable(name string) {
	cfg := config.Get()
	if cfg == nil {
		return
	}
	if cfg.ModelVariables.UserVariables != nil {
		delete(cfg.ModelVariables.UserVariables, name)
	}
	if err := config.SaveConfig(cfg); err != nil {
		logger.Errorf("Failed to save config after deleting variable %s: %v", name, err)
	}
}

// ─── Per-user helpers ───────────────────────────────────────────────────

func (h *ModelConfigHandler) getUserModelVars(userID int64) map[string]string {
	uc, err := database.Get().UserConfigStore().Get(context.Background(), userID)
	if err != nil || uc == nil || uc.ModelVars == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(uc.ModelVars), &m); err != nil {
		return map[string]string{}
	}
	return m
}

func (h *ModelConfigHandler) setUserModelVars(userID int64, vars map[string]string) {
	ctx := context.Background()
	uc, err := database.Get().UserConfigStore().Get(ctx, userID)
	if err != nil || uc == nil {
		uc = database.DefaultUserConfig(userID)
	}

	// Merge with existing per-user vars.
	existing := h.getUserModelVars(userID)
	for k, v := range vars {
		existing[k] = v
	}

	b, err := json.Marshal(existing)
	if err != nil {
		logger.Errorf("Failed to marshal user model vars: %v", err)
		return
	}
	uc.ModelVars = string(b)

	if err := database.Get().UserConfigStore().Upsert(ctx, uc); err != nil {
		logger.Errorf("Failed to save user model vars: %v", err)
	}
}

func (h *ModelConfigHandler) deleteUserModelVar(userID int64, name string) {
	existing := h.getUserModelVars(userID)
	delete(existing, name)

	b, err := json.Marshal(existing)
	if err != nil {
		logger.Errorf("Failed to marshal user model vars: %v", err)
		return
	}

	ctx := context.Background()
	uc, err := database.Get().UserConfigStore().Get(ctx, userID)
	if err != nil || uc == nil {
		uc = database.DefaultUserConfig(userID)
	}
	uc.ModelVars = string(b)

	if err := database.Get().UserConfigStore().Upsert(ctx, uc); err != nil {
		logger.Errorf("Failed to save user model vars: %v", err)
	}
}

// ─── Merge ──────────────────────────────────────────────────────────────

func mergeVars(system []config.ModelVariableItem, user map[string]string) []config.ModelVariableItem {
	// Start with system vars, let user overrides take precedence.
	merged := map[string]string{}
	order := []string{}

	// Collect all keys (system first, then user extras).
	for _, v := range system {
		merged[v.Name] = v.Value
		order = append(order, v.Name)
	}
	for k, v := range user {
		if _, exists := merged[k]; !exists {
			order = append(order, k)
		}
		merged[k] = v
	}

	result := make([]config.ModelVariableItem, 0, len(order))
	for _, name := range order {
		result = append(result, config.ModelVariableItem{
			Name:  name,
			Value: merged[name],
		})
	}
	return result
}

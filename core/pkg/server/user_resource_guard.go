package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"centag/core/pkg/database"
	"centag/core/pkg/groupmodel"
	"centag/core/pkg/useraccess"

	"github.com/gin-gonic/gin"
)

// teamResourceModelGuard rejects proxy requests whose model is not allowed
// for the current Team normal user (dual backend+model whitelist).
func (s *Server) teamResourceModelGuard() gin.HandlerFunc {
	// Under the group model (036) the allowlists live in the user's effective
	// policy (group_plans / user_plans), not the legacy users.allowed_* columns.
	// A 30s TTL cache is acceptable; admin mutations also invalidate their own
	// resolver instance in centag-pro.
	var resolver *groupmodel.Resolver
	if dbm := database.Get(); dbm != nil && dbm.GetDB() != nil {
		resolver = groupmodel.NewResolver(dbm.GetDB(), dbm.DriverName())
	}

	return func(c *gin.Context) {
		user := s.loadAccessUser(c)
		if user == nil || s.backendHandler == nil || s.backendHandler.backendManager == nil {
			c.Next()
			return
		}

		// 036: when the user has an active plan, enforce the effective policy
		// allowlists (group or custom mode) instead of the legacy columns.
		if resolver != nil {
			if pol, err := resolver.Resolve(c.Request.Context(), user.ID); err == nil && pol != nil && pol.HasPlan {
				s.enforcePolicyAllowLists(c, pol)
				return
			}
		}

		model := peekRequestModel(c)
		// Pipeline-as-model: centag/<id>、pipeline.<id>，或 #shortcut / #u... 快捷码
		if pid, ok := pipelineIDFromModel(model); ok {
			if pid != "" && !useraccess.CanUseSharedPipeline(user, pid) {
				// Allow if it is a tenant-owned pipeline (not in shared whitelist).
				tenantID := ownTenantID(user)
				if tenantID == "" || s.pipelineHandler == nil || s.pipelineHandler.pipelineRegistry == nil {
					RespondError(c, http.StatusForbidden, "pipeline not allowed for this user")
					c.Abort()
					return
				}
				p := s.pipelineHandler.pipelineRegistry.GetByTenant(tenantID, pid)
				if p == nil || p.TenantID == "" {
					RespondError(c, http.StatusForbidden, "pipeline not allowed for this user")
					c.Abort()
					return
				}
			}
			c.Next()
			return
		}
		if shortcut, ok := shortcutCodeFromModel(model); ok {
			if !s.canUsePipelineShortcut(user, shortcut) {
				RespondError(c, http.StatusForbidden, "pipeline not allowed for this user")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		tenantID := ownTenantID(user)
		backends := s.backendHandler.backendManager.ListByTenant(tenantID)
		if !useraccess.CanServeModel(user, backends, model) {
			RespondError(c, http.StatusForbidden, "model or backend not allowed for this user")
			c.Abort()
			return
		}
		c.Next()
	}
}

// enforcePolicyAllowLists enforces the effective policy's pipeline/model
// allowlists (empty = all allowed). Backend allowlists are already enforced by
// the centag-pro UserPlanEnforcer on the same request chain.
func (s *Server) enforcePolicyAllowLists(c *gin.Context, pol *groupmodel.EffectivePolicy) {
	model := peekRequestModel(c)
	if pid, ok := pipelineIDFromModel(model); ok {
		if pid != "" && !pol.IsAllowedPipeline(pid) {
			RespondError(c, http.StatusForbidden, "pipeline not allowed for this user")
			c.Abort()
			return
		}
		c.Next()
		return
	}
	if shortcut, ok := shortcutCodeFromModel(model); ok {
		user := s.loadAccessUser(c)
		if !s.canUsePipelineShortcut(user, shortcut) {
			RespondError(c, http.StatusForbidden, "pipeline not allowed for this user")
			c.Abort()
			return
		}
		c.Next()
		return
	}
	norm := strings.TrimSpace(model)
	if norm != "" && norm != "auto" && !pol.IsAllowedModel(norm) {
		RespondError(c, http.StatusForbidden, "model not allowed for this user")
		c.Abort()
		return
	}
	c.Next()
}

func peekRequestModel(c *gin.Context) string {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	var req struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &req)
	return strings.TrimSpace(req.Model)
}

// pipelineIDFromModel 从 model 字段解析流水线 ID。
// 支持 centag/<id> 与兼容写法 pipeline.<id>。
func pipelineIDFromModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	switch {
	case strings.HasPrefix(model, "centag/"):
		pid := strings.TrimPrefix(model, "centag/")
		pid = strings.TrimSpace(strings.SplitN(pid, " ", 2)[0])
		pid = strings.TrimSuffix(pid, ".auto")
		return pid, pid != ""
	case strings.HasPrefix(model, "pipeline."):
		pid := strings.TrimPrefix(model, "pipeline.")
		pid = strings.TrimSpace(strings.SplitN(pid, " ", 2)[0])
		pid = strings.TrimSuffix(pid, ".auto")
		return pid, pid != ""
	default:
		return "", false
	}
}

// shortcutCodeFromModel 识别 #t / #u801600974 等流水线快捷码（可带后续参数）。
func shortcutCodeFromModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if !strings.HasPrefix(model, "#") {
		return "", false
	}
	code := strings.TrimSpace(strings.SplitN(model, " ", 2)[0])
	if code == "#" {
		return "", false
	}
	return code, true
}

func (s *Server) canUsePipelineShortcut(user *database.User, shortcut string) bool {
	if user == nil || s == nil || s.pipelineHandler == nil || s.pipelineHandler.pipelineRegistry == nil {
		return false
	}
	own := ownTenantID(user)
	for _, p := range s.pipelineHandler.pipelineRegistry.ListByTenant(own) {
		if p == nil || strings.TrimSpace(p.ShortcutCode) != shortcut {
			continue
		}
		if p.TenantID != "" && p.TenantID == own {
			return true
		}
		return useraccess.CanUseSharedPipeline(user, p.ID)
	}
	return false
}

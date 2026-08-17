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
// for the current Team normal user (EffectivePlan allowlists only).
func (s *Server) teamResourceModelGuard() gin.HandlerFunc {
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

		if resolver == nil {
			RespondError(c, http.StatusForbidden, "no active plan; contact administrator")
			c.Abort()
			return
		}
		pol, err := groupmodel.ResolveForEdition(resolver, c.Request.Context(), user.ID, s.edition.IsTeam())
		if err != nil || pol == nil || !pol.HasPlan {
			RespondError(c, http.StatusForbidden, "no active plan; contact administrator")
			c.Abort()
			return
		}
		s.enforcePolicyAllowLists(c, pol)
	}
}

// enforcePolicyAllowLists enforces the effective policy's pipeline/model
// allowlists (empty = all allowed). Backend allowlists are already enforced by
// the centag-pro UserPlanEnforcer on the same request chain.
func (s *Server) enforcePolicyAllowLists(c *gin.Context, pol *groupmodel.EffectivePolicy) {
	model := peekRequestModel(c)
	// X-Pipeline-ID 请求头显式指定流水线（内置 agent 等客户端通过该头
	// 选中流水线，body model 仅为占位符）。按 core 的解析优先级
	// （X-Pipeline-ID 请求头 > 模式字符串）优先按 pipeline 白名单检查。
	if headerPID := peekPipelineIDHeader(c); headerPID != "" {
		if !pol.IsAllowedPipeline(headerPID) {
			RespondError(c, http.StatusForbidden, "pipeline not allowed for this user")
			c.Abort()
			return
		}
		c.Next()
		return
	}
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
		if !s.canUsePipelineShortcutWithPolicy(user, shortcut, pol) {
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

// peekPipelineIDHeader 解析 X-Pipeline-ID 请求头中的 base pipeline id，
// 剥离强制路由后缀（如 centag-ops-router:status-check → centag-ops-router）。
func peekPipelineIDHeader(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("X-Pipeline-ID"))
	if header == "" {
		return ""
	}
	if idx := strings.Index(header, ":"); idx > 0 {
		base := strings.TrimSpace(header[:idx])
		if base == "" {
			return ""
		}
		return base
	}
	return header
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

func (s *Server) canUsePipelineShortcutWithPolicy(user *database.User, shortcut string, pol *groupmodel.EffectivePolicy) bool {
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
		if pol != nil {
			return pol.IsAllowedPipeline(p.ID)
		}
		return false
	}
	return false
}

// Deprecated path kept for tests that still call the legacy helper name.
func (s *Server) canUsePipelineShortcut(user *database.User, shortcut string) bool {
	return s.canUsePipelineShortcutWithPolicy(user, shortcut, nil)
}

var _ = useraccess.Applies

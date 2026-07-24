package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"centag/core/pkg/useraccess"

	"github.com/gin-gonic/gin"
)

// teamResourceModelGuard rejects proxy requests whose model is not allowed
// for the current Team normal user (dual backend+model whitelist).
func (s *Server) teamResourceModelGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := s.loadAccessUser(c)
		if user == nil || s.backendHandler == nil || s.backendHandler.backendManager == nil {
			c.Next()
			return
		}

		model := peekRequestModel(c)
		// Pipeline-as-model: centag/<id> 或兼容 pipeline.<id>
		if pid, ok := pipelineIDFromModel(model); ok {
			if pid != "" && !useraccess.CanUseSharedPipeline(user, pid) {
				// Allow if it is a tenant-owned pipeline (not in shared whitelist).
				tenantID := ""
				if user.TenantID != nil {
					tenantID = *user.TenantID
				}
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

		tenantID := ""
		if user.TenantID != nil {
			tenantID = *user.TenantID
		}
		backends := s.backendHandler.backendManager.ListByTenant(tenantID)
		if !useraccess.CanServeModel(user, backends, model) {
			RespondError(c, http.StatusForbidden, "model or backend not allowed for this user")
			c.Abort()
			return
		}
		c.Next()
	}
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

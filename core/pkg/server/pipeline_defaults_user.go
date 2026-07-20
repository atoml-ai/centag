package server

import (
	"net/http"
	"strings"

	"centag/core/internal/auth"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/useraccess"

	"github.com/gin-gonic/gin"
)

// getPipelineDefaultsForUser returns system or per-user default pipeline.
func (s *Server) getPipelineDefaultsForUser(c *gin.Context) {
	sysDefault := config.DefaultSystemPipelineID
	if s.cfg != nil {
		sysDefault = s.cfg.Proxy.EffectiveDefaultPipeline()
	}

	user := s.loadAccessUser(c)
	defaultID := sysDefault
	canChange := true
	if user != nil {
		canChange = user.CanChangeDefaultPipeline
		if strings.TrimSpace(user.DefaultPipelineID) != "" {
			defaultID = user.DefaultPipelineID
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"default_pipeline_id":         defaultID,
		"allow_user_override":         canChange,
		"can_change_default_pipeline": canChange,
	})
}

// updatePipelineDefaultsForUser updates user.default_pipeline_id for Team normal users;
// admins still update system defaults via PipelineDefaultsHandler.
func (s *Server) updatePipelineDefaultsForUser(c *gin.Context) {
	if s.pipelineDefaultsHandler == nil {
		RespondInternalError(c, "pipeline defaults not available")
		return
	}

	if auth.GetScopedAccess(c) == auth.AccessGlobal {
		s.pipelineDefaultsHandler.UpdateDefaults(c)
		return
	}

	user := s.loadAccessUser(c)
	if user == nil {
		s.pipelineDefaultsHandler.UpdateDefaults(c)
		return
	}
	if !user.CanChangeDefaultPipeline {
		RespondError(c, http.StatusForbidden, "changing default pipeline is disabled for this user")
		return
	}

	var req struct {
		DefaultPipelineID string `json:"default_pipeline_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	pid := strings.TrimSpace(req.DefaultPipelineID)
	if pid == "" {
		RespondBadRequest(c, "default_pipeline_id is required")
		return
	}

	if s.pipelineHandler != nil && s.pipelineHandler.pipelineRegistry != nil {
		tenantID := ""
		if user.TenantID != nil {
			tenantID = *user.TenantID
		}
		pipelines := useraccess.FilterPipelines(user, s.pipelineHandler.pipelineRegistry.ListByTenant(tenantID))
		ok := false
		for _, p := range pipelines {
			if p != nil && p.ID == pid {
				ok = true
				break
			}
		}
		if !ok {
			RespondBadRequest(c, "pipeline not allowed for this user: "+pid)
			return
		}
	}

	user.DefaultPipelineID = pid
	if err := database.Get().UserStore().Update(c.Request.Context(), user); err != nil {
		logger.Errorf("update user default pipeline: %v", err)
		RespondInternalError(c, "failed to update default pipeline")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"default_pipeline_id": pid,
	})
}

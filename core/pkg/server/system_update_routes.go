package server

import (
	"net/http"

	"centag/core/pkg/systemupdateapi"

	"github.com/gin-gonic/gin"
)

// registerSystemUpdateRoutes mounts /system/update* OTA routes for editions
// that do not load the centag-pro team plugin (personal/minimal). The team
// plugin registers the same surface via extension.Host.ApplySystemAPI (E2.5).
func (s *Server) registerSystemUpdateRoutes(rg *gin.RouterGroup) {
	if s == nil || rg == nil || s.systemUpdate == nil {
		return
	}
	h := systemupdateapi.Wrap(s.systemUpdate)
	rg.POST("/update", wrapSystemUpdate(h.HandleUpdate))
	rg.GET("/update/check", wrapSystemUpdate(h.HandleCheckUpdate))
	rg.POST("/update/apply-remote", wrapSystemUpdate(h.HandleApplyRemote))
	rg.GET("/update/history", wrapSystemUpdate(h.HandleUpdateHistory))
	rg.POST("/rollback", wrapSystemUpdate(h.HandleRollback))
	rg.POST("/delete-update", wrapSystemUpdate(h.HandleDelete))
}

func wrapSystemUpdate(fn func(w http.ResponseWriter, r *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) {
		fn(c.Writer, c.Request)
	}
}

package server

import (
	"centag/core/pkg/plugin"

	"github.com/gin-gonic/gin"
)

// ListBackendTypes 返回所有已注册的后端类型元数据。
// 用于管理端发现「当前二进制支持哪些 backend type」，
// 并提供 DefaultBaseURL / KeyHelp / ConfigSchema 供 WebUI 动态表单。
func (h *BackendHandler) ListBackendTypes(c *gin.Context) {
	metas := plugin.ListBackendMetas()
	if metas == nil {
		metas = []plugin.BackendMeta{}
	}
	RespondSuccess(c, metas)
}

package server

import (
	"github.com/gin-gonic/gin"
)

// BindJSON 统一的 JSON 绑定处理
func BindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		RespondBadRequest(c, "Invalid request body: "+err.Error())
		return false
	}
	return true
}

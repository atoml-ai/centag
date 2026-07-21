// Package quotaapi exposes tenant quota middleware for commercial plugins.
package quotaapi

import (
	"centag/core/internal/middleware"
	"centag/core/pkg/database"

	"github.com/gin-gonic/gin"
)

// NewTenantQuotaMiddleware returns the tenant-level quota gin middleware.
func NewTenantQuotaMiddleware(db *database.Manager) gin.HandlerFunc {
	if db == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return middleware.NewQuotaMiddleware(db).Middleware()
}

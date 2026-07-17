package server

import (
	"net/http"

	"centag/core/internal/auth"

	"github.com/gin-gonic/gin"
)

func (s *Server) teamEditionOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.edition.IsPersonal() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "feature_unavailable",
				"message": "not available in personal edition",
				"edition": s.edition.String(),
			})
			return
		}
		c.Next()
	}
}

// teamAdminWriteOnly restricts mutating / sensitive backend ops to admins in team edition.
// personal / minimal keep existing authenticated write access.
func (s *Server) teamAdminWriteOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.edition.IsTeam() && !auth.IsAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "administrator access required",
				"message": "team edition: only administrators can modify backends",
			})
			return
		}
		c.Next()
	}
}
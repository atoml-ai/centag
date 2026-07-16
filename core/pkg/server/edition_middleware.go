package server

import (
	"net/http"

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
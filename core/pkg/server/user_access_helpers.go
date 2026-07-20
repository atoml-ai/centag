package server

import (
	"centag/core/internal/auth"
	"centag/core/internal/edition"
	"centag/core/pkg/database"
	"centag/core/pkg/useraccess"

	"github.com/gin-gonic/gin"
)

func (s *Server) currentUser(c *gin.Context) (*database.User, error) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return nil, err
	}
	return database.Get().UserStore().GetByID(c.Request.Context(), userID)
}

// loadTeamNormalUser returns the user when Team whitelist rules apply.
func loadTeamNormalUser(c *gin.Context, ed edition.Edition) *database.User {
	if !ed.IsTeam() {
		return nil
	}
	userID, err := auth.GetUserID(c)
	if err != nil {
		return nil
	}
	user, err := database.Get().UserStore().GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		return nil
	}
	if !useraccess.Applies(ed, user) {
		return nil
	}
	return user
}

// loadAccessUser returns the current user when Team whitelist rules apply; otherwise nil.
func (s *Server) loadAccessUser(c *gin.Context) *database.User {
	if s == nil {
		return nil
	}
	return loadTeamNormalUser(c, s.edition)
}

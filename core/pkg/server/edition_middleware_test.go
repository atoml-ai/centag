package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/internal/edition"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTeamEditionOnly_PersonalBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{edition: edition.Personal}
	r := gin.New()
	r.GET("/blocked", s.teamEditionOnly(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/blocked", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "personal edition")
}

func TestTeamEditionOnly_TeamAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{edition: edition.Team}
	r := gin.New()
	r.GET("/ok", s.teamEditionOnly(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
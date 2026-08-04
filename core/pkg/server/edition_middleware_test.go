package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/internal/auth"
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

func TestTeamAdminWriteOnly_TeamNormalForbidden(t *testing.T) {
	// Sensitive ops (export/import 等) still require admin in team.
	// 注：PUT /config/proxy 已改为普通用户写个人默认，不再挂此中间件。
	gin.SetMode(gin.TestMode)
	s := &Server{edition: edition.Team}
	r := gin.New()
	r.POST("/write", func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, string(auth.RoleNormal))
		c.Next()
	}, s.teamAdminWriteOnly(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "administrator access required")
}

func TestTeamAdminWriteOnly_TeamAdminAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{edition: edition.Team}
	r := gin.New()
	r.POST("/write", func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, string(auth.RoleAdmin))
		c.Next()
	}, s.teamAdminWriteOnly(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTeamAdminWriteOnly_PersonalNormalAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{edition: edition.Personal}
	r := gin.New()
	r.POST("/write", func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, string(auth.RoleNormal))
		c.Next()
	}, s.teamAdminWriteOnly(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTeamAdminWriteOnly_MinimalNormalAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{edition: edition.Minimal}
	r := gin.New()
	r.POST("/write", func(c *gin.Context) {
		c.Set(auth.CtxKeyRole, string(auth.RoleNormal))
		c.Next()
	}, s.teamAdminWriteOnly(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{startTime: time.Now().Add(-8 * time.Second)}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/status", nil)

	srv.handleStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"status"`)
	assert.Contains(t, body, `"uptime":"8s"`)
}

func TestFormatUptime(t *testing.T) {
	assert.Equal(t, "8s", formatUptime(8*time.Second+644*time.Millisecond))
	assert.Equal(t, "1m30s", formatUptime(90*time.Second))
	assert.Equal(t, "1h2m3s", formatUptime(time.Hour+2*time.Minute+3*time.Second))
	assert.Equal(t, "1d2h3m", formatUptime(26*time.Hour+3*time.Minute))
}

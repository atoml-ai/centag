package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewUserQuotaMiddleware(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)
	if m == nil {
		t.Fatal("expected non-nil middleware")
	}
}

func TestUserQuotaMiddleware_NoOp(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)
	m.RecordTokens(1, 100)
	m.RecordRequest(1)
	m.ResetWindow(1)

	tokens, requests, resetAt := m.GetWindowStats(1)
	if tokens != 0 || requests != 0 || !resetAt.IsZero() {
		t.Errorf("expected zero stats for no-op middleware, got tokens=%d requests=%d", tokens, requests)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

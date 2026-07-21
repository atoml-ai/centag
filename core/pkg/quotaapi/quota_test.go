package quotaapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewTenantQuotaMiddleware_NilDBPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := NewTenantQuotaMiddleware(nil)
	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}

	hit := false
	r := gin.New()
	r.Use(mw)
	r.GET("/ok", func(c *gin.Context) {
		hit = true
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if !hit || w.Code != http.StatusNoContent {
		t.Fatalf("passthrough failed: hit=%v status=%d", hit, w.Code)
	}
}

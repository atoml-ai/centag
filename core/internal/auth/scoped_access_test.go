package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetScopedAccess_NormalUserWithoutTenantIsScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(CtxKeyRole, RoleNormal)

	if got := GetScopedAccess(c); got != AccessTenant {
		t.Fatalf("GetScopedAccess = %v, want AccessTenant for normal user without tenant", got)
	}
}

func TestGetScopedAccess_AdminIsGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(CtxKeyRole, RoleAdmin)

	if got := GetScopedAccess(c); got != AccessGlobal {
		t.Fatalf("GetScopedAccess = %v, want AccessGlobal for admin", got)
	}
}

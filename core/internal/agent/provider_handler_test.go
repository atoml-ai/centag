package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestAgentProviderHandler_GetByAgentType_TenantScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := NewAgentProviderManager()
	if err := mgr.Add(&AgentProviderConfig{
		ID:        "system-claude",
		AgentType: "claude-code",
		Enabled:   true,
	}); err != nil {
		t.Fatalf("add system provider: %v", err)
	}
	if err := mgr.Add(&AgentProviderConfig{
		ID:        "tenant-claude",
		AgentType: "claude-code",
		TenantID:  "tenant-a",
		Enabled:   true,
	}); err != nil {
		t.Fatalf("add tenant provider: %v", err)
	}

	h := NewAgentProviderHandler(mgr)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent-providers/by-type?agent_type=claude-code", nil)
	c.Set(auth.CtxKeyRole, auth.RoleNormal)
	c.Set(auth.CtxKeyTenantID, "tenant-a")

	h.GetByAgentType(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var got AgentProviderConfig
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.ID != "tenant-claude" {
		t.Fatalf("provider id = %q, want tenant-claude", got.ID)
	}
}

func TestAgentProviderHandler_List_DenyTenantMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := NewAgentProviderManager()
	h := NewAgentProviderHandler(mgr)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent-providers?tenant_id=tenant-b", nil)
	c.Set(auth.CtxKeyRole, auth.RoleNormal)
	c.Set(auth.CtxKeyTenantID, "tenant-a")

	h.List(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

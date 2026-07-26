package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/internal/agent"

	"github.com/gin-gonic/gin"
)

func TestListAgentTypes_HermesOpenClawVerified(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAgentHandler(agent.NewTemplateRegistry(), nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent/types", nil)

	h.ListAgentTypes(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		AgentTypes []struct {
			Type          string `json:"type"`
			VerifiedWrite bool   `json:"verified_write"`
			VerifiedWrap  bool   `json:"verified_wrap"`
			Verified      bool   `json:"verified"`
		} `json:"agent_types"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v body=%s", err, w.Body.String())
	}
	got := map[string]struct{ write, wrap, any bool }{}
	for _, a := range resp.AgentTypes {
		got[a.Type] = struct{ write, wrap, any bool }{a.VerifiedWrite, a.VerifiedWrap, a.Verified}
	}
	for _, id := range []string{"hermes", "openclaw"} {
		v, ok := got[id]
		if !ok {
			t.Fatalf("%s missing from agent_types", id)
		}
		if !v.write || !v.wrap || !v.any {
			t.Fatalf("%s verified flags: write=%v wrap=%v verified=%v", id, v.write, v.wrap, v.any)
		}
	}
}

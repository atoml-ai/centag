package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/internal/agent"

	"github.com/gin-gonic/gin"
)

func TestApplyAgentProviderConfig_TenantOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := agent.GetProviderManager()
	createdIDs := []string{"test-system-claude", "test-tenant-claude"}
	t.Cleanup(func() {
		for _, id := range createdIDs {
			_ = mgr.Delete(id)
		}
	})

	_ = mgr.Add(&agent.AgentProviderConfig{
		ID:         "test-system-claude",
		AgentType:  "claude-code",
		BackendID:  "system-be",
		PipelineID: "system-pipe",
		Enabled:    true,
	})
	_ = mgr.Add(&agent.AgentProviderConfig{
		ID:         "test-tenant-claude",
		AgentType:  "claude-code",
		BackendID:  "tenant-be",
		PipelineID: "tenant-pipe",
		TenantID:   "tenant-a",
		Enabled:    true,
	})

	h := &Handler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set("tenant_id", "tenant-a")

	h.applyAgentProviderConfig(c, "anthropic-protocol")

	if got := c.Request.Header.Get("X-Backend-ID"); got != "tenant-be" {
		t.Fatalf("X-Backend-ID=%q want tenant-be", got)
	}
	if got := c.Request.Header.Get("X-Pipeline-ID"); got != "tenant-pipe" {
		t.Fatalf("X-Pipeline-ID=%q want tenant-pipe", got)
	}
}

func TestApplyAgentProviderConfig_ContextAgentTypeForOpenAIProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := agent.GetProviderManager()
	createdIDs := []string{"test-codex-provider"}
	t.Cleanup(func() {
		for _, id := range createdIDs {
			_ = mgr.Delete(id)
		}
	})

	_ = mgr.Add(&agent.AgentProviderConfig{
		ID:         "test-codex-provider",
		AgentType:  "codex",
		BackendID:  "codex-be",
		PipelineID: "codex-pipe",
		Enabled:    true,
	})

	h := &Handler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("agent_type", "codex")

	h.applyAgentProviderConfig(c, "openai-protocol")

	if got := c.Request.Header.Get("X-Backend-ID"); got != "codex-be" {
		t.Fatalf("X-Backend-ID=%q want codex-be", got)
	}
	if got := c.Request.Header.Get("X-Pipeline-ID"); got != "codex-pipe" {
		t.Fatalf("X-Pipeline-ID=%q want codex-pipe", got)
	}
}

func TestApplyAgentProviderConfig_ContextGeminiForOpenAIProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := agent.GetProviderManager()
	createdIDs := []string{"test-gemini-provider"}
	t.Cleanup(func() {
		for _, id := range createdIDs {
			_ = mgr.Delete(id)
		}
	})

	_ = mgr.Add(&agent.AgentProviderConfig{
		ID:         "test-gemini-provider",
		AgentType:  "gemini-cli",
		BackendID:  "gemini-be",
		PipelineID: "gemini-pipe",
		Enabled:    true,
	})

	h := &Handler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("agent_type", "gemini-cli")

	h.applyAgentProviderConfig(c, "openai-protocol")

	if got := c.Request.Header.Get("X-Backend-ID"); got != "gemini-be" {
		t.Fatalf("X-Backend-ID=%q want gemini-be", got)
	}
	if got := c.Request.Header.Get("X-Pipeline-ID"); got != "gemini-pipe" {
		t.Fatalf("X-Pipeline-ID=%q want gemini-pipe", got)
	}
}

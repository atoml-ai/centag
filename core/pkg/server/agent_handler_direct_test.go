package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/backend"

	"github.com/gin-gonic/gin"
)

// newAnonymousContext 返回无登录态的 gin 上下文，用于触发代理密钥解析失败路径。
func newAnonymousContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

// newDirectTestHandler 构造一个带内存后端管理器的 AgentHandler，用于直连模式单测。
func newDirectTestHandler(t *testing.T) *AgentHandler {
	t.Helper()
	mgr := backend.NewManager()
	backends := []*backend.BackendConfig{
		{
			ID:      "be-ok",
			Name:    "OKBackend",
			Type:    "openai",
			BaseURL: "https://api.direct.example.com/v1",
			APIKey:  "sk-real-123",
			Enabled: true,
		},
		{
			ID:      "be-disabled",
			Name:    "DisabledBackend",
			Type:    "openai",
			BaseURL: "https://api.disabled.example.com/v1",
			APIKey:  "sk-disabled",
			Enabled: false,
		},
		{
			ID:      "be-nokey",
			Name:    "NoKeyBackend",
			Type:    "openai",
			BaseURL: "https://api.nokey.example.com/v1",
			APIKey:  "",
			Enabled: true,
		},
	}
	for _, be := range backends {
		if err := mgr.Add(be); err != nil {
			t.Fatalf("add backend %s: %v", be.ID, err)
		}
	}
	return &AgentHandler{backendMgr: mgr}
}

// TestBuildPinnedProxyModelName 验证钉死模式模型写法 <后端ID>/<模型ID>。
func TestBuildPinnedProxyModelName(t *testing.T) {
	cases := []struct {
		backendID, model, want string
	}{
		{"opencode-zen", "hy3-free", "opencode-zen/hy3-free"},
		{" e2e-mock ", "centag-e2e-model", "e2e-mock/centag-e2e-model"},
		{"", "gpt-4o", "/gpt-4o"},
	}
	for _, c := range cases {
		if got := buildPinnedProxyModelName(c.backendID, c.model); got != c.want {
			t.Fatalf("buildPinnedProxyModelName(%q,%q) = %q, want %q", c.backendID, c.model, got, c.want)
		}
	}
}

// TestBuildAgentBackendInfo_DirectMode 验证直连模式使用后端真实密钥与真实 BaseURL，且不走代理密钥。
func TestBuildAgentBackendInfo_DirectMode(t *testing.T) {
	h := newDirectTestHandler(t)

	info, route, err := h.buildAgentBackendInfo(nil, "be-ok", "", "gpt-4o", false, "localhost", 20060)
	if err != nil {
		t.Fatalf("direct mode unexpected error: %v", err)
	}
	if route != "OKBackend" {
		t.Fatalf("route name = %s", route)
	}
	if info.BaseURL != "https://api.direct.example.com/v1" {
		t.Fatalf("BaseURL = %s", info.BaseURL)
	}
	if info.APIKey != "sk-real-123" {
		t.Fatalf("APIKey should be real backend key, got %s", info.APIKey)
	}
	if info.Model != "gpt-4o" {
		t.Fatalf("Model = %s", info.Model)
	}
}

// TestBuildAgentBackendInfo_DirectModeDisabled 验证禁用后端无法直连。
func TestBuildAgentBackendInfo_DirectModeDisabled(t *testing.T) {
	h := newDirectTestHandler(t)
	_, _, err := h.buildAgentBackendInfo(nil, "be-disabled", "", "gpt-4o", false, "localhost", 20060)
	if err == nil {
		t.Fatal("disabled backend should error")
	}
	if !strings.Contains(err.Error(), "已禁用") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBuildAgentBackendInfo_DirectModeNoKey 验证无可用密钥的后端无法直连。
func TestBuildAgentBackendInfo_DirectModeNoKey(t *testing.T) {
	h := newDirectTestHandler(t)
	_, _, err := h.buildAgentBackendInfo(nil, "be-nokey", "", "gpt-4o", false, "localhost", 20060)
	if err == nil {
		t.Fatal("backend without API key should error")
	}
	if !strings.Contains(err.Error(), "未配置可用 API Key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBuildAgentBackendInfo_PinnedProxySkipsDirectEgress 验证 backend_id+via_proxy 时
// 走代理路径（不直连后端真实地址），且所选后端校验通过后才进入代理密钥解析。
func TestBuildAgentBackendInfo_PinnedProxySkipsDirectEgress(t *testing.T) {
	h := newDirectTestHandler(t)

	// 直连分支会成功返回真实 BaseURL/密钥；此处报「代理密钥无法解析」说明走了代理路径。
	c := newAnonymousContext()
	_, _, err := h.buildAgentBackendInfo(c, "be-ok", "", "gpt-4o", true, "localhost", 20060)
	if err == nil {
		t.Fatal("expected proxy api key resolution error without auth context")
	}
	if strings.Contains(err.Error(), "api.direct.example.com") || strings.Contains(err.Error(), "sk-real") {
		t.Fatalf("pinned mode must not use direct egress, got: %v", err)
	}
	if !strings.Contains(err.Error(), "proxy api key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBuildAgentBackendInfo_PinnedProxyDisabledBackend 验证钉死模式拒绝禁用后端。
func TestBuildAgentBackendInfo_PinnedProxyDisabledBackend(t *testing.T) {
	h := newDirectTestHandler(t)
	_, _, err := h.buildAgentBackendInfo(newAnonymousContext(), "be-disabled", "", "gpt-4o", true, "localhost", 20060)
	if err == nil {
		t.Fatal("disabled backend should error in pinned proxy mode")
	}
	if !strings.Contains(err.Error(), "已禁用") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBuildAgentBackendInfo_PinnedProxyNoKeyBackend 验证钉死模式拒绝无密钥后端（优先于代理密钥解析）。
func TestBuildAgentBackendInfo_PinnedProxyNoKeyBackend(t *testing.T) {
	h := newDirectTestHandler(t)
	_, _, err := h.buildAgentBackendInfo(newAnonymousContext(), "be-nokey", "", "gpt-4o", true, "localhost", 20060)
	if err == nil {
		t.Fatal("backend without API key should error in pinned proxy mode")
	}
	if !strings.Contains(err.Error(), "默认出站") {
		t.Fatalf("unexpected error: %v", err)
	}
}

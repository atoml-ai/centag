package server

import (
	"strings"
	"testing"

	"centag/core/pkg/backend"
)

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

// TestBuildAgentBackendInfo_DirectMode 验证直连模式使用后端真实密钥与真实 BaseURL，且不走代理密钥。
func TestBuildAgentBackendInfo_DirectMode(t *testing.T) {
	h := newDirectTestHandler(t)

	info, route, err := h.buildAgentBackendInfo(nil, "be-ok", "", "gpt-4o", "localhost", 20060)
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
	_, _, err := h.buildAgentBackendInfo(nil, "be-disabled", "", "gpt-4o", "localhost", 20060)
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
	_, _, err := h.buildAgentBackendInfo(nil, "be-nokey", "", "gpt-4o", "localhost", 20060)
	if err == nil {
		t.Fatal("backend without API key should error")
	}
	if !strings.Contains(err.Error(), "未配置可用 API Key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

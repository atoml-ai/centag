package agent

import (
	"strings"
	"testing"
	"time"

	"edgeag/pkg/agentcore"
)

func TestRuntimeEngine_effectiveTimeout(t *testing.T) {
	tests := []struct {
		name   string
		config *AgentConfig
		want   time.Duration
	}{
		{name: "nil config", config: nil, want: 10 * time.Minute},
		{name: "zero timeout", config: &AgentConfig{}, want: 10 * time.Minute},
		{name: "negative timeout", config: &AgentConfig{Timeout: -1}, want: 10 * time.Minute},
		{name: "custom timeout", config: &AgentConfig{Timeout: 2 * time.Minute}, want: 2 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewRuntimeEngine(tt.config, "", nil)
			if got := e.effectiveTimeout(); got != tt.want {
				t.Errorf("effectiveTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuntimeEngine_effectiveMaxTurns(t *testing.T) {
	tests := []struct {
		name   string
		config *AgentConfig
		want   int
	}{
		{name: "nil config", config: nil, want: 20},
		{name: "zero", config: &AgentConfig{}, want: 20},
		{name: "negative", config: &AgentConfig{MaxTurns: -5}, want: 20},
		{name: "custom", config: &AgentConfig{MaxTurns: 8}, want: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewRuntimeEngine(tt.config, "", nil)
			if got := e.effectiveMaxTurns(); got != tt.want {
				t.Errorf("effectiveMaxTurns() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRuntimeEngine_effectiveMaxTokens(t *testing.T) {
	tests := []struct {
		name   string
		config *AgentConfig
		want   int
	}{
		{name: "nil config", config: nil, want: 8192},
		{name: "zero", config: &AgentConfig{}, want: 8192},
		{name: "custom", config: &AgentConfig{MaxTokens: 2048}, want: 2048},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewRuntimeEngine(tt.config, "", nil)
			if got := e.effectiveMaxTokens(); got != tt.want {
				t.Errorf("effectiveMaxTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRuntimeEngine_EnsureBackend(t *testing.T) {
	cfg := &AgentConfig{Tools: ToolsConfig{Allowed: []string{}}}
	e := NewRuntimeEngine(cfg, "", nil)

	// 首次构造 backend，并透传 pipelineID。
	e.EnsureBackend(AgentEngineOptions{
		BaseURL:    "http://127.0.0.1:20060",
		BackendID:  "openai",
		Model:      "gpt-4o",
		PipelineID: "agent-skill-router:status-check",
	})
	if e.backendHTTP == nil {
		t.Fatal("EnsureBackend() did not construct backend")
	}
	if e.backendID != "openai" {
		t.Errorf("backendID = %q, want openai", e.backendID)
	}
	proxyMode, pipelineID, _ := e.backendHTTP.Strategy()
	if pipelineID != "agent-skill-router:status-check" {
		t.Errorf("pipelineID = %q, want agent-skill-router:status-check", pipelineID)
	}
	if proxyMode != "transparent" {
		t.Errorf("proxyMode = %q, want transparent", proxyMode)
	}

	// 相同 backend 不变更 pipelineID 时，不重建 backend。
	prevBackend := e.backendHTTP
	e.EnsureBackend(AgentEngineOptions{
		BaseURL:   "http://127.0.0.1:20060",
		BackendID: "openai",
		Model:     "gpt-4o",
	})
	if e.backendHTTP != prevBackend {
		t.Error("EnsureBackend() should reuse backend when backendID unchanged")
	}
}

func TestRuntimeEngine_EnsureBackend_PipelineChange(t *testing.T) {
	cfg := &AgentConfig{Tools: ToolsConfig{Allowed: []string{}}}
	e := NewRuntimeEngine(cfg, "", nil)

	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", PipelineID: "p1"})
	_, pid1, _ := e.backendHTTP.Strategy()
	if pid1 != "p1" {
		t.Fatalf("first pipelineID = %q, want p1", pid1)
	}

	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", PipelineID: "p2"})
	_, pid2, _ := e.backendHTTP.Strategy()
	if pid2 != "p2" {
		t.Errorf("changed pipelineID = %q, want p2", pid2)
	}
}

func TestRuntimeEngine_EnsureBackend_SessionChange(t *testing.T) {
	cfg := &AgentConfig{Tools: ToolsConfig{Allowed: []string{}}}
	e := NewRuntimeEngine(cfg, "", nil)

	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", SessionID: "sess-1"})
	_, _, sid1 := e.backendHTTP.Strategy()
	if sid1 != "sess-1" {
		t.Fatalf("first sessionID = %q, want sess-1", sid1)
	}

	// 同一后端、不同 session → 不重建，仅更新 X-Session-ID。
	prevBackend := e.backendHTTP
	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", SessionID: "sess-2"})
	if e.backendHTTP != prevBackend {
		t.Fatal("EnsureBackend() should reuse backend when backendID unchanged")
	}
	_, _, sid2 := e.backendHTTP.Strategy()
	if sid2 != "sess-2" {
		t.Errorf("changed sessionID = %q, want sess-2", sid2)
	}

	// 相同 session → 不重复更新。
	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", SessionID: "sess-2"})
	_, _, sid3 := e.backendHTTP.Strategy()
	if sid3 != "sess-2" {
		t.Errorf("unchanged sessionID = %q, want sess-2", sid3)
	}
}

func TestRuntimeEngine_EnsureBackend_TokenRefresh(t *testing.T) {
	cfg := &AgentConfig{Tools: ToolsConfig{Allowed: []string{}}}
	e := NewRuntimeEngine(cfg, "", nil)

	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", Token: ""})
	e.RefreshToken("")
	e.EnsureBackend(AgentEngineOptions{BackendID: "b1", Token: "jwt-new"})
	if e.backendHTTP == nil {
		t.Fatal("backend nil after EnsureBackend")
	}
}

func TestRuntimeEngine_NewSession_NoBackend(t *testing.T) {
	cfg := &AgentConfig{Tools: ToolsConfig{Allowed: []string{"read_config"}}}
	e := NewRuntimeEngine(cfg, "", nil)
	if _, err := e.NewSession("prompt"); err == nil {
		t.Error("NewSession() should error when backend not initialized")
	}
}

func TestRuntimeEngine_RegisterTools_Intersection(t *testing.T) {
	// 全局白名单包含 read_config/analyze，但 skill 工具集仅 read_config。
	cfg := &AgentConfig{
		Tools: ToolsConfig{Allowed: []string{"read_config", "read_log", "analyze", "centag_info", "system_info"}},
	}
	e := NewRuntimeEngine(cfg, "", nil)

	registry := agentcore.NewToolRegistry()
	if err := e.registerTools(registry, []string{"read_config", "read_log", "analyze", "read_database"}); err != nil {
		t.Fatalf("registerTools() error = %v", err)
	}

	// read_database 不在白名单 → 不注册。
	if registry.GetTool("read_database") != nil {
		t.Error("read_database should not be registered (not in whitelist)")
	}
	if registry.GetTool("read_config") == nil {
		t.Error("read_config should be registered (in both skill set and whitelist)")
	}
	if registry.GetTool("analyze") == nil {
		t.Error("analyze should be registered (in both skill set and whitelist)")
	}
}

func TestRuntimeEngine_RegisterTools_NoSkillTools(t *testing.T) {
	// 未传 skill 工具集时仅受全局白名单约束。
	cfg := &AgentConfig{
		Tools: ToolsConfig{Allowed: []string{"read_config"}},
	}
	e := NewRuntimeEngine(cfg, "", nil)

	registry := agentcore.NewToolRegistry()
	if err := e.registerTools(registry); err != nil {
		t.Fatalf("registerTools() error = %v", err)
	}
	if registry.GetTool("read_config") == nil {
		t.Error("read_config should be registered under global whitelist only")
	}
	if registry.GetTool("analyze") != nil {
		t.Error("analyze should NOT be registered (not in whitelist)")
	}
}

func TestRuntimeEngine_Truncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q, want short", got)
	}
	long := strings.Repeat("x", 300)
	got := truncate(long, 200)
	if !strings.HasSuffix(got, "...") || len(got) != 203 {
		t.Errorf("truncate(long) = %q (len %d), want suffix ... len 203", got, len(got))
	}
}

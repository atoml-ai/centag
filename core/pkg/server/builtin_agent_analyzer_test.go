package server

import (
	"context"
	"strings"
	"testing"

	"centag/core/internal/agent"
	"centag/core/internal/agent/skills"
	"centag/core/internal/agent/tools"
)

// fakeProvider 实现 AgentDataProvider，用于 replyForSkill / statusCheckReport 测试。
type fakeProvider struct {
	backends   []AgentBackendInfo
	pipelines  []AgentPipelineInfo
	defaultPID string
	defaultBID string
}

func (p *fakeProvider) ListBackends(ctx context.Context) []AgentBackendInfo   { return p.backends }
func (p *fakeProvider) ListPipelines(ctx context.Context) []AgentPipelineInfo { return p.pipelines }
func (p *fakeProvider) DefaultPipelineID() string                             { return p.defaultPID }
func (p *fakeProvider) DefaultBackendID() string                              { return p.defaultBID }

func newTestBuiltinHandler() *BuiltinAgentHandler {
	skillReg := skills.NewSkillRegistry()
	skills.LoadBuiltinSkills(skillReg)
	return &BuiltinAgentHandler{
		config:        agent.DefaultAgentConfig(),
		skillRegistry: skillReg,
		toolRegistry:  tools.NewToolRegistry("", nil, nil),
		engine:        agent.NewRuntimeEngine(agent.DefaultAgentConfig(), "", nil),
	}
}

func TestReplyForSkill_NilProvider(t *testing.T) {
	h := newTestBuiltinHandler()
	h.provider = nil
	got := h.replyForSkill("status-check", "help me")
	if !strings.Contains(got, "Agent 数据源未初始化") {
		t.Errorf("replyForSkill(nil provider) = %q, want init message", got)
	}
}

func TestReplyForSkill_DefaultFallback(t *testing.T) {
	h := newTestBuiltinHandler()
	h.provider = &fakeProvider{}
	got := h.replyForSkill("", "hello")
	if !strings.Contains(got, "hello") {
		t.Errorf("replyForSkill(default) = %q, want echo input", got)
	}
}

func TestReplyForSkill_StatusCheck(t *testing.T) {
	h := newTestBuiltinHandler()
	h.provider = &fakeProvider{
		backends: []AgentBackendInfo{
			{ID: "b1", Name: "openai", Enabled: true, HealthOK: true, BaseURL: "http://x", Default: true},
		},
		pipelines: []AgentPipelineInfo{
			{ID: "p1", Name: "透明", Default: true},
		},
		defaultPID: "p1",
		defaultBID: "b1",
	}
	got := h.replyForSkill("status-check", "")
	for _, want := range []string{"openai", "p1", "默认后端: b1"} {
		if !strings.Contains(got, want) {
			t.Errorf("status-check report missing %q:\n%s", want, got)
		}
	}
}

func TestDefaultAgentSystemPrompt(t *testing.T) {
	p := defaultAgentSystemPrompt()
	for _, want := range []string{"read_config", "read_log", "write_config", "analyze", "system_info", "centag_info"} {
		if !strings.Contains(p, want) {
			t.Errorf("default prompt missing tool %q", want)
		}
	}
}

func TestOrDash(t *testing.T) {
	if got := orDash("x"); got != "x" {
		t.Errorf("orDash(x) = %q, want x", got)
	}
	if got := orDash(""); got != "-" {
		t.Errorf("orDash(empty) = %q, want -", got)
	}
}

func TestStatusCheckReport_EmptyProvider(t *testing.T) {
	p := &fakeProvider{}
	got := statusCheckReport(p)
	for _, want := range []string{"未配置任何后端", "未配置任何流水线", "默认后端: -", "默认流水线: -"} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
	}
}

func TestStatusCheckReport_WithData(t *testing.T) {
	p := &fakeProvider{
		backends: []AgentBackendInfo{
			{ID: "b2", Name: "z-backend", Type: "openai", Enabled: true, HealthOK: true},
			{ID: "b1", Name: "a-backend", Type: "ollama", Enabled: true, Default: true},
		},
		pipelines: []AgentPipelineInfo{
			{ID: "p2", Name: "pipeline-z"},
			{ID: "p1", Name: "pipeline-a", Default: true},
		},
		defaultPID: "p1",
		defaultBID: "b1",
	}
	got := statusCheckReport(p)
	// 排序校验：a-backend 应在 z-backend 前
	aIdx := strings.Index(got, "a-backend")
	zIdx := strings.Index(got, "z-backend")
	if aIdx == -1 || zIdx == -1 || aIdx > zIdx {
		t.Errorf("backends not sorted: a-backend=%d z-backend=%d", aIdx, zIdx)
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"centag/core/internal/agent/skills"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
)

func newTestSkillHandler(t *testing.T) *BuiltinAgentHandler {
	t.Helper()
	dataDir := t.TempDir()
	reg := skills.NewSkillPluginRegistry()
	store := skills.NewFileManifestStore(filepath.Join(dataDir, "agent-skills"))
	skillReg := skills.NewSkillRegistry()
	pr := pipeline.NewPipelineRegistry()
	h := &BuiltinAgentHandler{
		skillRegistry:       skillReg,
		skillPluginRegistry: reg,
		manifestStore:       store,
		pipelineRegistry:    pr,
		defaultBackend:      "default-backend",
		defaultModel:        "default-model",
	}
	return h
}

func ginTestContext(method, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// 从路径解析 :name 参数（模拟路由）
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, seg := range segments {
		if seg != "skills" && seg != "clone" && seg != "messages" && seg != "sessions" && !strings.HasPrefix(seg, "id") {
			c.Params = append(c.Params, gin.Param{Key: "name", Value: seg})
			break
		}
	}
	return c, rec
}

func TestSkillCRUD(t *testing.T) {
	h := newTestSkillHandler(t)

	// 创建
	body := `{"name":"health-check","description":"健康检查","category":"运维诊断","tools":["read_config","read_database"],"steps":["读取配置","生成报告"],"system_prompt":"检查系统健康状态"}`
	c, rec := ginTestContext("POST", "/skills", body)
	h.CreateSkill(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create code = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}

	// 重名 409
	c, rec = ginTestContext("POST", "/skills", body)
	h.CreateSkill(c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create code = %d, want 409", rec.Code)
	}

	// 列表可见（含 pipeline_id / custom）
	c, rec = ginTestContext("GET", "/skills", "")
	h.ListSkills(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("list code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"name":"health-check"`) || !strings.Contains(rec.Body.String(), `"pipeline_id":"`+agentSkillRouterPipelineID+`"`) {
		t.Errorf("list body missing custom skill:\n%s", rec.Body.String())
	}

	// 内置删除 403（先注册内置）
	if err := h.manifestStore.Save("status-check", []byte(testBuiltinManifest)); err != nil {
		t.Fatalf("seed builtin manifest: %v", err)
	}
	builtinP, err := skills.ParseSkillPluginManifest([]byte(testBuiltinManifest))
	if err != nil {
		t.Fatalf("parse builtin manifest: %v", err)
	}
	if err := h.skillPluginRegistry.Register(builtinP); err != nil {
		t.Fatalf("register builtin plugin: %v", err)
	}
	c, rec = ginTestContext("DELETE", "/skills/status-check", "")
	h.DeleteSkill(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete builtin code = %d, want 403", rec.Code)
	}

	// 内置 skill 允许直接更新，且保持 internal=true（仍受删除保护）
	c, rec = ginTestContext("PUT", "/skills/status-check", `{"name":"status-check","description":"更新后的内置","category":"运维","tools":["read_config"],"steps":["读取配置"],"system_prompt":"updated prompt"}`)
	h.UpdateSkill(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("update builtin code = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	p, ok := h.skillPluginRegistry.Get("status-check")
	if !ok {
		t.Fatal("status-check not in registry after update")
	}
	if !p.Internal() {
		t.Error("updated builtin skill should remain internal")
	}
	if got := p.GetSkillDefinition().SystemPrompt; got != "updated prompt" {
		t.Errorf("updated builtin system_prompt = %q, want %q", got, "updated prompt")
	}

	// 复制 status-check（内置）→ custom-check
	c, rec = ginTestContext("POST", "/skills/status-check/clone", `{"name":"custom-check"}`)
	h.CloneSkill(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("clone code = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}
	p, ok = h.skillPluginRegistry.Get("custom-check")
	if !ok {
		t.Fatal("custom-check not in registry after clone")
	}
	if p.Internal() {
		t.Error("cloned skill should be non-internal")
	}

	// 更新 custom-check
	c, rec = ginTestContext("PUT", "/skills/custom-check", `{"name":"custom-check","description":"更新后","category":"运维","tools":["read_config"],"steps":["step"],"system_prompt":"updated prompt"}`)
	h.UpdateSkill(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("update code = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// 删除 custom-check
	c, rec = ginTestContext("DELETE", "/skills/custom-check", "")
	h.DeleteSkill(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code = %d, want 200", rec.Code)
	}
	if _, ok := h.skillPluginRegistry.Get("custom-check"); ok {
		t.Error("custom-check still in registry after delete")
	}
	if _, err := h.manifestStore.Load("custom-check"); err == nil {
		t.Error("custom-check manifest still on disk after delete")
	}
}

func TestNormalizeSkillName(t *testing.T) {
	cases := map[string]string{
		"Health Check": "health-check",
		"agent-skill-foo": "foo",
		"Foo_Bar 2024":    "foo_bar-2024",
		"":                "",
	}
	for in, want := range cases {
		if got := normalizeSkillName(in); got != want {
			t.Errorf("normalizeSkillName(%q) = %q, want %q", in, got, want)
		}
	}
}

const testBuiltinManifest = `api_version: centag.agent-skill/v1alpha1
implementation: builtin.agent-skill-status-check
name: 状态检查
kind: agent.skill
version: 1.0.0
description: 查询centag当前运行状态
skill:
  name: status-check
  category: 运维诊断
  enabled: true
  internal: true
  tools:
    - read_config
    - read_database
    - analyze
  steps:
    - 读取配置
    - 生成报告
  system_prompt: |-
    你是一个 centag 运维助手，正在执行 skill: status-check
    请按照以下步骤执行：
    1. 读取配置
    2. 生成报告
`

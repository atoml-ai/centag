package skills

import (
	"strings"
	"testing"
)

const validManifest = `api_version: centag.agent-skill/v1alpha1
implementation: builtin.agent-skill-status-check
name: 状态检查
kind: agent.skill
version: 1.0.0
description: 检查 centag 运行状态并生成报告
permissions:
  - agent.tool.read_config
  - agent.tool.read_database
  - agent.tool.analyze
skill:
  name: status-check
  category: system
  enabled: true
  internal: true
  tools:
    - read_config
    - read_database
    - analyze
  steps:
    - 调用 read_database 查询系统状态
    - 调用 analyze 生成状态报告
  system_prompt: |
    你是一个 centag 运维助手。
    请按步骤执行。
`

func TestParseSkillPluginManifest_Valid(t *testing.T) {
	p, err := ParseSkillPluginManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest() error = %v", err)
	}

	desc := p.Descriptor()
	if desc.APIVersion != SkillPluginSchemaVersion {
		t.Errorf("APIVersion = %q, want %q", desc.APIVersion, SkillPluginSchemaVersion)
	}
	if desc.Kind != SkillPluginKind {
		t.Errorf("Kind = %q, want %q", desc.Kind, SkillPluginKind)
	}
	if desc.Implementation != "builtin.agent-skill-status-check" {
		t.Errorf("Implementation = %q", desc.Implementation)
	}
	if len(desc.Permissions) != 3 {
		t.Errorf("Permissions len = %d, want 3", len(desc.Permissions))
	}

	def := p.GetSkillDefinition()
	if def.Name != "status-check" {
		t.Errorf("Skill.Name = %q, want status-check", def.Name)
	}
	if def.Category != "system" {
		t.Errorf("Category = %q, want system", def.Category)
	}
	if len(def.Tools) != 3 || len(def.Steps) != 2 {
		t.Errorf("Tools/Steps = %v/%v", def.Tools, def.Steps)
	}
	if !strings.Contains(def.SystemPrompt, "centag 运维助手") {
		t.Errorf("SystemPrompt missing content: %q", def.SystemPrompt)
	}
	if !p.Enabled() || !p.Internal() {
		t.Errorf("Enabled/Internal = %v/%v, want true/true", p.Enabled(), p.Internal())
	}
	if p.PipelineID() != CentagOpsRouterPipelineID {
		t.Errorf("PipelineID = %q, want %q (shared router pipeline)", p.PipelineID(), CentagOpsRouterPipelineID)
	}
	if got := ForcedRoutePipelineID("status-check"); got != "centag-ops-router:status-check" {
		t.Errorf("ForcedRoutePipelineID = %q, want centag-ops-router:status-check", got)
	}
}

func TestParseSkillPluginManifest_Defaults(t *testing.T) {
	in := strings.ReplaceAll(validManifest, "  enabled: true\n", "")
	in = strings.ReplaceAll(in, "  internal: true\n", "")
	p, err := ParseSkillPluginManifest([]byte(in))
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest() error = %v", err)
	}
	if !p.Enabled() || !p.Internal() {
		t.Errorf("Enabled/Internal default = %v/%v, want true/true", p.Enabled(), p.Internal())
	}
}

func TestParseSkillPluginManifest_MalformedYAML(t *testing.T) {
	in := "api_version: [unclosed"
	if _, err := ParseSkillPluginManifest([]byte(in)); err == nil {
		t.Fatal("ParseSkillPluginManifest() want error for malformed YAML, got nil")
	}
}

func TestParseSkillPluginManifest_MissingAPIVersion(t *testing.T) {
	in := strings.Replace(validManifest, "api_version: centag.agent-skill/v1alpha1\n", "", 1)
	if _, err := ParseSkillPluginManifest([]byte(in)); err == nil || !strings.Contains(err.Error(), "api_version") {
		t.Fatalf("want missing api_version error, got %v", err)
	}
}

func TestParseSkillPluginManifest_UnsupportedAPIVersion(t *testing.T) {
	in := strings.Replace(validManifest, "centag.agent-skill/v1alpha1", "centag.agent-skill/v9beta9", 1)
	if _, err := ParseSkillPluginManifest([]byte(in)); err == nil || !strings.Contains(err.Error(), "unsupported api_version") {
		t.Fatalf("want unsupported api_version error, got %v", err)
	}
}

func TestParseSkillPluginManifest_WrongKind(t *testing.T) {
	in := strings.Replace(validManifest, "kind: agent.skill", "kind: pipeline.node", 1)
	if _, err := ParseSkillPluginManifest([]byte(in)); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("want kind mismatch error, got %v", err)
	}
}

func TestParseSkillPluginManifest_MissingSkillName(t *testing.T) {
	in := strings.Replace(validManifest, "  name: status-check\n", "", 1)
	if _, err := ParseSkillPluginManifest([]byte(in)); err == nil || !strings.Contains(err.Error(), "skill.name") {
		t.Fatalf("want missing skill.name error, got %v", err)
	}
}

func TestParseSkillPluginManifest_UnknownFieldsIgnored(t *testing.T) {
	in := validManifest + "  extra_field: ignored\nnot_a_real_key: also_ignored\n"
	p, err := ParseSkillPluginManifest([]byte(in))
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest() error = %v (unknown fields should be ignored)", err)
	}
	if p.GetSkillDefinition().Name != "status-check" {
		t.Errorf("Skill.Name = %q, unknown fields corrupted parse", p.GetSkillDefinition().Name)
	}
}

func TestMarshalSkillPluginManifest_RoundTrip(t *testing.T) {
	p, err := ParseSkillPluginManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest() error = %v", err)
	}
	out, err := MarshalSkillPluginManifest(p)
	if err != nil {
		t.Fatalf("MarshalSkillPluginManifest() error = %v", err)
	}
	back, err := ParseSkillPluginManifest(out)
	if err != nil {
		t.Fatalf("re-parse after marshal: %v\n%s", err, string(out))
	}
	if back.GetSkillDefinition().Name != p.GetSkillDefinition().Name {
		t.Errorf("round-trip name = %q, want %q", back.GetSkillDefinition().Name, p.GetSkillDefinition().Name)
	}
	if back.GetSkillDefinition().SystemPrompt != p.GetSkillDefinition().SystemPrompt {
		t.Errorf("round-trip system_prompt mismatch")
	}
	if back.Descriptor().Implementation != p.Descriptor().Implementation {
		t.Errorf("round-trip implementation mismatch")
	}
}

func TestPipelineIDFromImplementation(t *testing.T) {
	tests := []struct {
		impl string
		want string
	}{
		{"builtin.agent-skill-status-check", "agent-skill-status-check"},
		{"custom.agent-skill-my-skill", "agent-skill-my-skill"},
		{"agent-skill-noprefix", "agent-skill-noprefix"},
	}
	for _, tt := range tests {
		if got := PipelineIDFromImplementation(tt.impl); got != tt.want {
			t.Errorf("PipelineIDFromImplementation(%q) = %q, want %q", tt.impl, got, tt.want)
		}
	}
}

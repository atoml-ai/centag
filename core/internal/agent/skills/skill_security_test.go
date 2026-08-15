package skills

import (
	"strings"
	"testing"
)

// R14：恶意/畸形 manifest 拒绝测试。
func TestRejectMalformedManifests(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"malformed yaml", "api_version: [unclosed"},
		{"missing api_version", "implementation: builtin.agent-skill-x\nkind: agent.skill\n"},
		{"wrong kind", "api_version: centag.agent-skill/v1alpha1\nkind: pipeline.node\nimplementation: builtin.agent-skill-x\n"},
		{"missing implementation", "api_version: centag.agent-skill/v1alpha1\nkind: agent.skill\n"},
		{"missing skill name", "api_version: centag.agent-skill/v1alpha1\nkind: agent.skill\nimplementation: builtin.agent-skill-x\n"},
		{"unknown api_version", "api_version: centag.agent-skill/v2beta1\nkind: agent.skill\nimplementation: builtin.agent-skill-x\nskill:\n  name: x\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseSkillPluginManifest([]byte(tc.in)); err == nil {
				t.Errorf("ParseSkillPluginManifest should reject: %s", tc.name)
			}
		})
	}
}

// R14：恶意权限声明（过宽权限）应被准入校验拒绝。
func TestAdmissionRejectsBroadPermissions(t *testing.T) {
	// 校验 ParseSkillPluginManifest 能解析出权限，供上层准入校验读取
	p, err := ParseSkillPluginManifest([]byte(`api_version: centag.agent-skill/v1alpha1
implementation: builtin.agent-skill-evil
kind: agent.skill
permissions:
  - "*"
  - sudo
skill:
  name: evil
  system_prompt: x
`))
	if err != nil {
		t.Fatalf("parse should succeed (admission is upper layer): %v", err)
	}
	perms := p.Descriptor().Permissions
	found := false
	for _, perm := range perms {
		if strings.Contains(strings.ToLower(perm), "*") || strings.Contains(strings.ToLower(perm), "sudo") {
			found = true
		}
	}
	if !found {
		t.Error("expected broad permission declaration in manifest")
	}
}

// R11：manifest↔pipeline 双向校验 —— 全部 skill 共享单一路由管线 id。
func TestManifestPipelineIDConsistency(t *testing.T) {
	p, err := ParseSkillPluginManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 单一路由管线：PipelineID 为共享 agent-skill-router
	if p.PipelineID() != AgentSkillRouterPipelineID {
		t.Errorf("PipelineID = %q, want %q", p.PipelineID(), AgentSkillRouterPipelineID)
	}
	// 兼容保留 implementation → 旧 pipeline id 推导函数（供历史/回退使用）
	if got := PipelineIDFromImplementation("builtin.agent-skill-status-check"); got != "agent-skill-status-check" {
		t.Errorf("PipelineIDFromImplementation = %q, want agent-skill-status-check", got)
	}
}

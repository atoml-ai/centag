package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// T15：skill manifest（kind: agent.skill）不应被当作流水线模板加载；
// agent-skill-router.yaml（schema centag.pipeline）照常加载。
func TestLoadYAMLFilesFromDir_SkipsSkillManifests(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "agent-skill-status-check.yaml", `
api_version: centag.agent-skill/v1alpha1
kind: agent.skill
name: status-check
`)
	writeFile(t, dir, "agent-skill-router.yaml", `
version: '1.0'
schema_version: centag.pipeline/v1alpha1
id: agent-skill-router
name: Agent Skill Router
nodes:
- id: skill-classifier
  type: router
`)
	writeFile(t, dir, "transparent-proxy.yaml", `
version: '1.0'
schema_version: centag.pipeline/v1alpha1
id: transparent-proxy
name: Transparent
nodes: []
`)

	tmplMap := make(map[string]InitialPipelineTemplate)
	ok := loadYAMLFilesFromDir(dir, tmplMap)
	if !ok {
		t.Fatal("loadYAMLFilesFromDir returned false for valid dir")
	}

	// skill manifest 不应加载
	if _, exists := tmplMap["status-check"]; exists {
		t.Error("skill manifest should not be loaded as pipeline template")
	}
	if _, exists := tmplMap["agent-skill-status-check"]; exists {
		t.Error("skill manifest should not be loaded as pipeline template")
	}

	// router 模板与普通模板应加载
	if _, exists := tmplMap["agent-skill-router"]; !exists {
		t.Error("agent-skill-router pipeline template should be loaded")
	}
	if _, exists := tmplMap["transparent-proxy"]; !exists {
		t.Error("transparent-proxy pipeline template should be loaded")
	}
}

func TestLoadYAMLFilesFromDir_NonYAMLIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "not yaml")
	writeFile(t, dir, "template.txt", "not yaml")

	tmplMap := make(map[string]InitialPipelineTemplate)
	if ok := loadYAMLFilesFromDir(dir, tmplMap); !ok {
		t.Fatal("loadYAMLFilesFromDir returned false")
	}
	if len(tmplMap) != 0 {
		t.Errorf("non-yaml files loaded: %v", tmplMap)
	}
}

func TestLoadYAMLFilesFromDir_MissingID(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "no-id.yaml", "version: '1.0'\nname: no id\n")

	tmplMap := make(map[string]InitialPipelineTemplate)
	if ok := loadYAMLFilesFromDir(dir, tmplMap); !ok {
		t.Fatal("loadYAMLFilesFromDir returned false")
	}
	if len(tmplMap) != 0 {
		t.Errorf("template without id should be skipped: %v", tmplMap)
	}
}

func TestLoadYAMLFilesFromDir_MissingDir(t *testing.T) {
	tmplMap := make(map[string]InitialPipelineTemplate)
	if ok := loadYAMLFilesFromDir(filepath.Join(t.TempDir(), "nope"), tmplMap); ok {
		t.Error("loadYAMLFilesFromDir should return false for missing dir")
	}
}

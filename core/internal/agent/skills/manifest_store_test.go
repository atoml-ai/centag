package skills

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFileManifestStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agent-skills")
	s := NewFileManifestStore(dir)

	// 空目录 List 返回空
	names, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("List() = %v, want empty", names)
	}

	// Save
	custom := strings.Replace(validManifest, "builtin.agent-skill-status-check", "custom.agent-skill-custom-check", 1)
	custom = strings.Replace(custom, "name: status-check", "name: custom-check", 1)
	if err := s.Save("custom-check", []byte(custom)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	names, err = s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 1 || names[0] != "custom-check" {
		t.Fatalf("List() = %v, want [custom-check]", names)
	}

	// Load
	data, err := s.Load("custom-check")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	p, err := ParseSkillPluginManifest(data)
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest(loaded) error = %v", err)
	}
	if p.GetSkillDefinition().Name != "custom-check" {
		t.Errorf("loaded skill name = %q, want custom-check", p.GetSkillDefinition().Name)
	}

	// LoadToRegistry
	r := NewSkillPluginRegistry()
	if err := s.LoadToRegistry(r); err != nil {
		t.Fatalf("LoadToRegistry() error = %v", err)
	}
	got, ok := r.Get("custom-check")
	if !ok {
		t.Fatal("custom-check not in registry after LoadToRegistry")
	}
	if got.Internal() {
		t.Error("custom skill should be non-internal")
	}

	// Delete
	if err := s.Delete("custom-check"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Load("custom-check"); err == nil {
		t.Error("Load after Delete should error")
	}
	if err := s.Delete("custom-check"); err == nil {
		t.Error("Delete missing should error")
	}
}

func TestFileManifestStore_ManifestFileName(t *testing.T) {
	if got := manifestFileName("foo"); got != "agent-skill-foo.yaml" {
		t.Errorf("manifestFileName(foo) = %q", got)
	}
	if got := manifestFileName("agent-skill-foo"); got != "agent-skill-foo.yaml" {
		t.Errorf("manifestFileName(agent-skill-foo) = %q", got)
	}
}

func TestFileManifestStore_LoadToRegistryEmpty(t *testing.T) {
	s := NewFileManifestStore(filepath.Join(t.TempDir(), "missing"))
	r := NewSkillPluginRegistry()
	if err := s.LoadToRegistry(r); err != nil {
		t.Fatalf("LoadToRegistry(missing dir) error = %v, want nil", err)
	}
	if len(r.ListAll()) != 0 {
		t.Error("empty store should not register anything")
	}
}

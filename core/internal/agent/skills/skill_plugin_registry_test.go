package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillPluginRegistry_Register(t *testing.T) {
	r := NewSkillPluginRegistry()
	p, err := ParseSkillPluginManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest() error = %v", err)
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	got, ok := r.Get("status-check")
	if !ok || got.GetSkillDefinition().Name != "status-check" {
		t.Fatalf("Get() = %v, %v", got, ok)
	}
	if len(r.List()) != 1 || len(r.ListAll()) != 1 {
		t.Fatalf("List/ListAll len = %d/%d, want 1/1", len(r.List()), len(r.ListAll()))
	}
}

func TestSkillPluginRegistry_RegisterDuplicate(t *testing.T) {
	r := NewSkillPluginRegistry()
	p, err := ParseSkillPluginManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseSkillPluginManifest() error = %v", err)
	}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := r.Register(p); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestSkillPluginRegistry_EmptyName(t *testing.T) {
	r := NewSkillPluginRegistry()
	if err := r.Register(&skillPlugin{}); err == nil {
		t.Fatal("want error for empty skill name, got nil")
	}
}

func TestSkillPluginRegistry_Delete(t *testing.T) {
	r := NewSkillPluginRegistry()
	p, _ := ParseSkillPluginManifest([]byte(validManifest))
	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := r.Delete("status-check"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(r.ListAll()) != 0 {
		t.Fatalf("ListAll len = %d after delete, want 0", len(r.ListAll()))
	}
	if err := r.Delete("missing"); err == nil {
		t.Fatal("want error deleting missing skill, got nil")
	}
}

func TestSkillPluginRegistry_IsSkillAllowed(t *testing.T) {
	r := NewSkillPluginRegistry()
	internal := strings.Replace(validManifest, "  internal: true\n", "  internal: false\n", 1)
	p, _ := ParseSkillPluginManifest([]byte(internal))
	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if !r.IsSkillAllowed("status-check", false) {
		t.Error("IsSkillAllowed(internalOnly=false) = false, want true")
	}
	if r.IsSkillAllowed("status-check", true) {
		t.Error("IsSkillAllowed(internalOnly=true) for custom = true, want false")
	}
	if r.IsSkillAllowed("missing", false) {
		t.Error("IsSkillAllowed(missing) = true, want false")
	}

	disabled := strings.Replace(validManifest, "  enabled: true\n", "  enabled: false\n", 1)
	p2, _ := ParseSkillPluginManifest([]byte(disabled))
	r2 := NewSkillPluginRegistry()
	_ = r2.Register(p2)
	if r2.IsSkillAllowed("status-check", false) {
		t.Error("IsSkillAllowed for disabled skill = true, want false")
	}
	if len(r2.List()) != 0 {
		t.Error("List should exclude disabled skills")
	}
	if len(r2.ListAll()) != 1 {
		t.Error("ListAll should include disabled skills")
	}
}

func TestSkillPluginRegistry_LoadFromSources(t *testing.T) {
	dir := t.TempDir()
	manifestFile := filepath.Join(dir, "agent-skill-status-check.yaml")
	if err := os.WriteFile(manifestFile, []byte(validManifest), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	// 非 agent-skill 前缀文件（如 pipeline 模板）应被跳过
	if err := os.WriteFile(filepath.Join(dir, "direct-backend.yaml"), []byte("id: direct-backend\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	r := NewSkillPluginRegistry()
	if err := r.LoadFromSources([]ManifestSource{{Dir: dir, Custom: false}}); err != nil {
		t.Fatalf("LoadFromSources() error = %v", err)
	}
	if len(r.ListAll()) != 1 {
		t.Fatalf("ListAll len = %d, want 1 (non agent-skill- files skipped)", len(r.ListAll()))
	}
	if _, ok := r.Get("status-check"); !ok {
		t.Error("status-check not registered")
	}
}

func TestSkillPluginRegistry_LoadFromSources_MissingDir(t *testing.T) {
	r := NewSkillPluginRegistry()
	if err := r.LoadFromSources([]ManifestSource{{Dir: filepath.Join(t.TempDir(), "nonexistent")}}); err != nil {
		t.Fatalf("LoadFromSources(missing dir) error = %v, want nil", err)
	}
}

func TestSkillPluginRegistry_LoadFromSources_ManifestData(t *testing.T) {
	r := NewSkillPluginRegistry()
	src := ManifestSource{
		ManifestData: map[string][]byte{"agent-skill-status-check.yaml": []byte(validManifest)},
	}
	if err := r.LoadFromSources([]ManifestSource{src}); err != nil {
		t.Fatalf("LoadFromSources() error = %v", err)
	}
	if _, ok := r.Get("status-check"); !ok {
		t.Error("status-check not registered from ManifestData")
	}
}

func TestSkillPluginRegistry_CustomOverridesBuiltin(t *testing.T) {
	r := NewSkillPluginRegistry()
	builtin, _ := ParseSkillPluginManifest([]byte(validManifest))
	if err := r.Register(builtin); err != nil {
		t.Fatalf("Register(builtin) error = %v", err)
	}

	customManifest := strings.Replace(validManifest, "  internal: true\n", "  internal: false\n", 1)
	customManifest = strings.Replace(customManifest, "name: 状态检查", "name: 自定义状态检查", 1)
	src := ManifestSource{
		ManifestData: map[string][]byte{"agent-skill-status-check.yaml": []byte(customManifest)},
		Custom:       true,
	}
	if err := r.LoadFromSources([]ManifestSource{src}); err != nil {
		t.Fatalf("LoadFromSources() error = %v", err)
	}
	p, ok := r.Get("status-check")
	if !ok {
		t.Fatal("status-check not found after custom override")
	}
	if p.Internal() {
		t.Error("custom override should be non-internal")
	}
	if len(r.ListAll()) != 1 {
		t.Fatalf("ListAll len = %d after override, want 1", len(r.ListAll()))
	}
}

func TestSkillPluginRegistry_Names(t *testing.T) {
	r := NewSkillPluginRegistry()
	p, _ := ParseSkillPluginManifest([]byte(validManifest))
	_ = r.Register(p)
	names := r.Names()
	if len(names) != 1 || names[0] != "status-check" {
		t.Fatalf("Names() = %v, want [status-check]", names)
	}
}

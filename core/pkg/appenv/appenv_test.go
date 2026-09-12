package appenv

import (
	"os"
	"path/filepath"
	"testing"
)

func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	saved := os.Environ()
	t.Cleanup(func() {
		for _, e := range saved {
			p := splitEnv(e)
			_ = os.Setenv(p[0], p[1])
		}
	})
	for k, v := range kv {
		if v == "" {
			_ = os.Unsetenv(k)
		} else {
			_ = os.Setenv(k, v)
		}
	}
}

func splitEnv(e string) []string {
	for i := 0; i < len(e); i++ {
		if e[i] == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return []string{e, ""}
}

func TestParse(t *testing.T) {
	content := `
# comment
PLAIN=value
export EXPORTED=1
QUOTED="double quoted"
SINGLE='single quoted'
INLINE=value # trailing comment
NOEQUALS
=emptykey
`
	got := parse(content)
	want := map[string]string{
		"PLAIN":    "value",
		"EXPORTED": "1",
		"QUOTED":   "double quoted",
		"SINGLE":   "single quoted",
		"INLINE":   "value",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d pairs, want %d: %+v", len(got), len(want), got)
	}
	for _, p := range got {
		if want[p.key] != p.value {
			t.Errorf("key=%s got=%q want=%q", p.key, p.value, want[p.key])
		}
	}
}

func TestLoad_SetsUnsetVarsOnly(t *testing.T) {
	dir := t.TempDir()
	withEnv(t, map[string]string{"CENTAG_HOME": dir, "APP_EXISTING": "from-env"})

	conf := filepath.Join(dir, confFileName)
	content := "APP_NEW=from-file\nAPP_EXISTING=from-file\n"
	if err := os.WriteFile(conf, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded = false
	legacyWarnings = nil

	if err := Load(); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv("APP_NEW"); v != "from-file" {
		t.Errorf("APP_NEW = %q, want from-file", v)
	}
	if v := os.Getenv("APP_EXISTING"); v != "from-env" {
		t.Errorf("APP_EXISTING = %q, want from-env (env vars must win)", v)
	}
}

func TestLoad_MissingFileOK(t *testing.T) {
	dir := t.TempDir()
	withEnv(t, map[string]string{"CENTAG_HOME": dir})
	loaded = false
	legacyWarnings = nil

	if err := Load(); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
}

func TestLoad_LegacyWarning(t *testing.T) {
	dir := t.TempDir()
	withEnv(t, map[string]string{"CENTAG_HOME": dir})
	loaded = false
	legacyWarnings = nil

	legacyDir := filepath.Join(dir, "config", "secrets")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, ".env"), []byte("X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	_ = Load()
	if len(legacyWarnings) == 0 {
		t.Fatal("expected legacy warning for config/secrets/.env")
	}
}

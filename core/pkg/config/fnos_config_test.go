package config

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSaveLoadDeploymentConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)

	dep := DeploymentConfig{
		CleanDataOnUninstall: true,
		DBDriver:             "postgresql",
		PGHost:               "192.168.1.10",
		PGPort:               "5433",
		PGUser:               "centag_user",
		PGPassword:           "s3cret",
		PGDB:                 "centag_prod",
	}
	if err := SaveDeploymentConfig(dep); err != nil {
		t.Fatalf("SaveDeploymentConfig: %v", err)
	}

	// 文件必须存在且为单行紧凑 JSON（兼容 fnOS grep/sed 解析）
	raw, err := os.ReadFile(filepath.Join(dir, "centag.conf"))
	if err != nil {
		t.Fatalf("read centag.conf: %v", err)
	}
	if lineCount := len(regexp.MustCompile(`\n`).FindAll(raw, -1)); lineCount != 0 {
		t.Fatalf("centag.conf must be single-line, got %d newlines", lineCount)
	}

	got := LoadDeploymentConfig()
	if got != dep {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, dep)
	}
}

// 确保写入的 JSON 能被 fnOS 脚本的 grep/sed 简单解析（与 cmd/main 的 json_get 相同逻辑）。
func TestDeploymentConfigParsableByFnosScripts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)

	dep := DeploymentConfig{
		CleanDataOnUninstall: true,
		DBDriver:             "postgresql",
		PGHost:               "pg.internal",
		PGPort:               "5432",
		PGUser:               "postgres",
		PGPassword:           "pw",
		PGDB:                 "centag",
	}
	if err := SaveDeploymentConfig(dep); err != nil {
		t.Fatalf("SaveDeploymentConfig: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "centag.conf"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(raw)

	jsonGet := func(key string) string {
		re := regexp.MustCompile(`"` + key + `"[[:space:]]*:`)
		if !re.MatchString(content) {
			return ""
		}
		m := regexp.MustCompile(`"` + key + `"[[:space:]]*:[[:space:]]*"([^"]*)"`)
		sub := m.FindStringSubmatch(content)
		if len(sub) != 2 {
			return ""
		}
		return sub[1]
	}

	for key, want := range map[string]string{
		"db_driver":   "postgresql",
		"pg_host":     "pg.internal",
		"pg_port":     "5432",
		"pg_user":     "postgres",
		"pg_password": "pw",
		"pg_db":       "centag",
	} {
		if got := jsonGet(key); got != want {
			t.Errorf("json_get(%q) = %q, want %q", key, got, want)
		}
	}

	// clean_data_on_uninstall 布尔：uninstall_callback 用 (true|false) 捕获
	if !regexp.MustCompile(`"clean_data_on_uninstall"[[:space:]]*:true`).MatchString(content) {
		t.Errorf("clean_data_on_uninstall=true not found in %s", content)
	}
}

func TestLoadDeploymentConfigMissingFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)

	got := LoadDeploymentConfig()
	def := DefaultDeploymentConfig()
	if got != def {
		t.Fatalf("missing file should return defaults, got %+v", got)
	}
	if got.DBDriver != "sqlite" || got.CleanDataOnUninstall {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}

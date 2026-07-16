package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBootstrap_ResolvesSQLitePathToExecutableDir(t *testing.T) {
	t.Setenv("LLM_PROXY_DB_DRIVER", "sqlite")
	t.Setenv("SQLITE_PATH", "./storage/centag.db")

	cfg := LoadBootstrap()
	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	want := filepath.Join(filepath.Dir(execPath), "storage", "centag.db")

	if cfg.DB.Path != want {
		t.Fatalf("bootstrap db path = %q, want %q", cfg.DB.Path, want)
	}
}

func TestBuildSQLiteDSN_ResolvesRelativePath(t *testing.T) {
	t.Setenv("SQLITE_PATH", "./storage/centag.db")
	dsn := buildSQLiteDSN()

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	wantPath := filepath.Join(filepath.Dir(execPath), "storage", "centag.db")

	if !strings.Contains(dsn, "file:"+wantPath) {
		t.Fatalf("dsn = %q, want contains %q", dsn, "file:"+wantPath)
	}
}

func TestNormalizeProxyPathFields_UsesExecutableDir(t *testing.T) {
	cfg := &Config{
		SystemProxy: SystemProxyConfig{
			CACertPath: "./certs/ca.crt",
			CAKeyPath:  "./certs/ca.key",
			CertDir:    "./certs/domains",
		},
		HostProxy: HostProxyConfig{
			CACertPath: "./certs/ca.crt",
			CAKeyPath:  "./certs/ca.key",
			CertDir:    "./certs/domains",
		},
	}

	normalizeProxyPathFields(cfg)

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	execDir := filepath.Dir(execPath)

	wantCert := filepath.Join(execDir, "certs", "ca.crt")
	wantKey := filepath.Join(execDir, "certs", "ca.key")
	wantDir := filepath.Join(execDir, "certs", "domains")

	if cfg.SystemProxy.CACertPath != wantCert || cfg.HostProxy.CACertPath != wantCert {
		t.Fatalf("ca cert path not normalized: system=%q host=%q want=%q", cfg.SystemProxy.CACertPath, cfg.HostProxy.CACertPath, wantCert)
	}
	if cfg.SystemProxy.CAKeyPath != wantKey || cfg.HostProxy.CAKeyPath != wantKey {
		t.Fatalf("ca key path not normalized: system=%q host=%q want=%q", cfg.SystemProxy.CAKeyPath, cfg.HostProxy.CAKeyPath, wantKey)
	}
	if cfg.SystemProxy.CertDir != wantDir || cfg.HostProxy.CertDir != wantDir {
		t.Fatalf("cert dir not normalized: system=%q host=%q want=%q", cfg.SystemProxy.CertDir, cfg.HostProxy.CertDir, wantDir)
	}
}

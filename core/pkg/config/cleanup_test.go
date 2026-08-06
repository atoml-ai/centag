package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupDeploymentData_SkippedWhenSQLite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)
	if err := SaveDeploymentConfig(DeploymentConfig{
		DBDriver: "sqlite",
	}); err != nil {
		t.Fatalf("SaveDeploymentConfig: %v", err)
	}

	res := CleanupDeploymentData(context.Background(), dir)
	if !res.Skipped {
		t.Fatalf("expected skipped for sqlite, got %+v", res)
	}
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestCleanupDeploymentData_SkippedWhenNoConfig(t *testing.T) {
	dir := t.TempDir() // empty dir → no centag.conf
	res := CleanupDeploymentData(context.Background(), dir)
	if !res.Skipped {
		t.Fatalf("expected skipped without config, got %+v", res)
	}
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestCleanupDeploymentData_InvalidConfigIsError(t *testing.T) {
	dir := t.TempDir()
	bad := []byte(`{not-json`)
	if err := os.WriteFile(filepath.Join(dir, deploymentConfigFile), bad, 0o644); err != nil {
		t.Fatalf("write bad conf: %v", err)
	}
	res := CleanupDeploymentData(context.Background(), dir)
	if res.Skipped {
		t.Fatalf("expected error (not skipped) for invalid JSON, got %+v", res)
	}
	if res.Error == nil {
		t.Fatal("expected parse/read error for invalid centag.conf")
	}
	if res.Cleaned {
		t.Fatal("must not clean when config is invalid")
	}
}

func TestCleanupDeploymentData_PGConnectionErrorIsSurface(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)
	if err := SaveDeploymentConfig(DeploymentConfig{
		DBDriver: "postgresql",
		PGHost:   "127.0.0.1",
		PGPort:   "1", // unreachable → connection error
		PGUser:   "postgres",
		PGDB:     "centag",
	}); err != nil {
		t.Fatalf("SaveDeploymentConfig: %v", err)
	}

	res := CleanupDeploymentData(context.Background(), dir)
	if res.Skipped {
		t.Fatalf("expected attempt for postgresql, got skipped")
	}
	if res.Error == nil {
		t.Fatal("expected connection error for unreachable PG")
	}
	if !res.Cleaned {
		t.Logf("ok: connection failed as expected: %v", res.Error)
	}
}

func TestBuildCleanupPGDSN(t *testing.T) {
	dsn := buildCleanupPGDSN(DeploymentConfig{
		PGHost:     "db.internal",
		PGPort:     "5433",
		PGUser:     "app",
		PGPassword: "s3cret",
		PGDB:       "centag",
	})
	for _, want := range []string{
		"host=db.internal", "port=5433", "user=app",
		"password=s3cret", "dbname=centag", "sslmode=disable",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn %q missing %q", dsn, want)
		}
	}
}

func TestBuildCleanupPGDSN_Defaults(t *testing.T) {
	dsn := buildCleanupPGDSN(DeploymentConfig{})
	for _, want := range []string{
		"host=localhost", "port=5432", "user=postgres",
		"dbname=centag", "sslmode=disable",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn %q missing default %q", dsn, want)
		}
	}
}

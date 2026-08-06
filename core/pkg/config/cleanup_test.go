package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupDeploymentData_SQLiteRemovesDBFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CENTAG_DATA_DIR", dir)
	t.Setenv("SQLITE_PATH", "")
	_ = os.Unsetenv("SQLITE_PATH")

	if err := SaveDeploymentConfig(DeploymentConfig{
		DBDriver: "sqlite",
	}); err != nil {
		t.Fatalf("SaveDeploymentConfig: %v", err)
	}

	dbPath := filepath.Join(dir, "centag.db")
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"
	for _, p := range []string{dbPath, walPath, shmPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	res := CleanupDeploymentData(context.Background(), dir)
	if res.Skipped {
		t.Fatalf("expected sqlite cleanup, got skipped: %+v", res)
	}
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if !res.Cleaned {
		t.Fatalf("expected Cleaned=true, got %+v", res)
	}
	for _, p := range []string{dbPath, walPath, shmPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, stat err=%v", p, err)
		}
	}
}

func TestCleanupDeploymentData_SQLiteMissingFileIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	_ = os.Unsetenv("SQLITE_PATH")
	res := CleanupDeploymentData(context.Background(), dir)
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if res.Skipped {
		t.Fatalf("expected cleaned (idempotent) for missing sqlite file, got skipped: %+v", res)
	}
	if !res.Cleaned || res.Driver != "sqlite" {
		t.Fatalf("got %+v", res)
	}
}

func TestCleanupDeploymentData_SQLitePathFromEnv(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom", "app.db")
	if err := os.MkdirAll(filepath.Dir(custom), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(custom, []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQLITE_PATH", custom)

	res := CleanupDeploymentData(context.Background(), dir)
	if res.Error != nil || !res.Cleaned {
		t.Fatalf("got %+v", res)
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatalf("expected env SQLITE_PATH removed, err=%v", err)
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
	}, "postgres")
	for _, want := range []string{
		"host=db.internal", "port=5433", "user=app",
		"password=s3cret", "dbname=postgres", "sslmode=disable",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn %q missing %q", dsn, want)
		}
	}
}

func TestBuildCleanupPGDSN_Defaults(t *testing.T) {
	dsn := buildCleanupPGDSN(DeploymentConfig{}, "")
	for _, want := range []string{
		"host=localhost", "port=5432", "user=postgres",
		"dbname=postgres", "sslmode=disable",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn %q missing default %q", dsn, want)
		}
	}
}

func TestQuotePGIdent(t *testing.T) {
	got, err := quotePGIdent("centag")
	if err != nil || got != `"centag"` {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := quotePGIdent("centag;drop"); err == nil {
		t.Fatal("expected error for unsafe ident")
	}
	if _, err := quotePGIdent("postgres"); err != nil {
		// quote itself allows the name; refuse is in cleanupPostgreSQLDatabase
		t.Fatalf("quote should accept postgres ident: %v", err)
	}
}

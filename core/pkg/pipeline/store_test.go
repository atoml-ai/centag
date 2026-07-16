package pipeline

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const pluginRegistryTableDDL = `
CREATE TABLE IF NOT EXISTS plugin_registry (
    id INTEGER PRIMARY KEY,
    implementation TEXT UNIQUE NOT NULL,
    kind TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    descriptor_json TEXT,
    source TEXT NOT NULL DEFAULT 'unknown',
    enabled BOOLEAN DEFAULT TRUE,
    signature_status TEXT DEFAULT 'none',
    last_health_check TIMESTAMP,
    health_status TEXT DEFAULT 'unknown',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)`

func newSQLiteRegistryStore(t *testing.T) *DBPluginRegistryStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(pluginRegistryTableDDL); err != nil {
		t.Fatalf("create plugin_registry schema failed: %v", err)
	}
	return &DBPluginRegistryStore{db: db, pg: false}
}

func maybeNewPostgresRegistryStore(t *testing.T) *DBPluginRegistryStore {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("LLM_PROXY_TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("postgres integration skipped: set TEST_POSTGRES_DSN or LLM_PROXY_TEST_POSTGRES_DSN")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping postgres failed: %v", err)
	}

	ddl := strings.ReplaceAll(pluginRegistryTableDDL, "INTEGER PRIMARY KEY", "SERIAL PRIMARY KEY")
	ddl = strings.ReplaceAll(ddl, "TEXT,", "TEXT,")
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("create postgres plugin_registry schema failed: %v", err)
	}
	return &DBPluginRegistryStore{db: db, pg: true}
}

func runPluginRegistryCRUDSuite(t *testing.T, store *DBPluginRegistryStore, prefix string) {
	t.Helper()
	impl := fmt.Sprintf("%s-test-plugin", prefix)

	plugin := &PluginRegistration{
		Implementation:  impl,
		Kind:            "builtin.test",
		Version:         "1.0.0",
		DescriptorJSON:  `{"name":"test"}`,
		Source:          "remote",
		Enabled:         true,
		SignatureStatus: "none",
		HealthStatus:    "unknown",
	}

	if err := store.Register(plugin); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := store.Get(impl)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Implementation != impl {
		t.Fatalf("expected implementation %q, got %q", impl, got.Implementation)
	}
	if !got.LastHealthCheck.IsZero() {
		t.Fatalf("last_health_check should be zero before UpdateHealthCheck, got %v", got.LastHealthCheck)
	}

	plugin.Version = "2.0.0"
	plugin.Enabled = false
	if err := store.Update(plugin); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, err = store.Get(impl)
	if err != nil {
		t.Fatalf("Get after Update failed: %v", err)
	}
	if got.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", got.Version)
	}
	if got.Enabled {
		t.Fatalf("expected plugin disabled after update")
	}

	enabled, err := store.ListEnabled()
	if err != nil {
		t.Fatalf("ListEnabled failed: %v", err)
	}
	for _, p := range enabled {
		if p.Implementation == impl {
			t.Fatalf("disabled plugin should not appear in ListEnabled")
		}
	}

	if err := store.UpdateHealthCheck(impl, true, "ok"); err != nil {
		t.Fatalf("UpdateHealthCheck failed: %v", err)
	}
	got, err = store.Get(impl)
	if err != nil {
		t.Fatalf("Get after UpdateHealthCheck failed: %v", err)
	}
	if got.HealthStatus != "healthy" {
		t.Fatalf("expected health_status healthy, got %s", got.HealthStatus)
	}
	if got.LastHealthCheck.IsZero() {
		t.Fatalf("expected last_health_check to be set")
	}
	if time.Since(got.LastHealthCheck) > 2*time.Minute {
		t.Fatalf("unexpected stale last_health_check: %v", got.LastHealthCheck)
	}

	if err := store.Delete(impl); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := store.Get(impl); err == nil {
		t.Fatalf("expected not found after delete")
	}
}

func TestPluginRegistryStoreSQLiteIntegration(t *testing.T) {
	store := newSQLiteRegistryStore(t)
	runPluginRegistryCRUDSuite(t, store, "sqlite")
}

func TestPluginRegistryStorePostgresIntegration(t *testing.T) {
	store := maybeNewPostgresRegistryStore(t)
	runPluginRegistryCRUDSuite(t, store, fmt.Sprintf("pg-%d", time.Now().UnixNano()))
}

func TestPluginRegistryStoreValidation(t *testing.T) {
	store := newSQLiteRegistryStore(t)
	if _, err := store.Get(""); err == nil {
		t.Fatalf("expected error for empty implementation")
	}
	if err := store.Delete(""); err == nil {
		t.Fatalf("expected error for empty implementation delete")
	}
	if err := store.UpdateHealthCheck("", true, ""); err == nil {
		t.Fatalf("expected error for empty implementation health update")
	}
}

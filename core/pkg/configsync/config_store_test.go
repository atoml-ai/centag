package configsync

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func initTestSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func applyConfigStoreMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	migrationSQL, err := os.ReadFile("../database/migrations/044_config_store.sqlite.sql")
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	if _, err := db.Exec(string(migrationSQL)); err != nil {
		t.Fatalf("failed to apply migration: %v", err)
	}
}

func TestDBConfigStore_Upsert(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-001: Insert new config
	if err := store.Upsert("test_key", "test_value"); err != nil {
		t.Fatalf("TC-CS-001: unexpected error: %v", err)
	}

	val, err := store.Get("test_key")
	if err != nil {
		t.Fatalf("TC-CS-001: failed to get config: %v", err)
	}
	if val != "test_value" {
		t.Fatalf("TC-CS-001: got %q, want %q", val, "test_value")
	}

	// TC-CS-002: Update existing config
	if err := store.Upsert("test_key", "updated_value"); err != nil {
		t.Fatalf("TC-CS-002: unexpected error: %v", err)
	}

	val, err = store.Get("test_key")
	if err != nil {
		t.Fatalf("TC-CS-002: failed to get config: %v", err)
	}
	if val != "updated_value" {
		t.Fatalf("TC-CS-002: got %q, want %q", val, "updated_value")
	}
}

func TestDBConfigStore_Get(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-003: Get non-existent key
	_, err := store.Get("non_existent")
	if err == nil {
		t.Fatal("TC-CS-003: expected error for non-existent key")
	}

	// TC-CS-004: Get existing key
	if err := store.Upsert("key1", "value1"); err != nil {
		t.Fatalf("TC-CS-004: unexpected error: %v", err)
	}
	val, err := store.Get("key1")
	if err != nil {
		t.Fatalf("TC-CS-004: failed to get config: %v", err)
	}
	if val != "value1" {
		t.Fatalf("TC-CS-004: got %q, want %q", val, "value1")
	}
}

func TestDBConfigStore_Count(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-005: Count empty store
	count, err := store.Count()
	if err != nil {
		t.Fatalf("TC-CS-005: unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("TC-CS-005: got %d, want 0", count)
	}

	// TC-CS-006: Count after inserts
	_ = store.Upsert("key1", "value1")
	_ = store.Upsert("key2", "value2")
	count, err = store.Count()
	if err != nil {
		t.Fatalf("TC-CS-006: unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("TC-CS-006: got %d, want 2", count)
	}
}

func TestDBConfigStore_ListAll(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-007: ListAll empty store
	configs, err := store.ListAll()
	if err != nil {
		t.Fatalf("TC-CS-007: unexpected error: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("TC-CS-007: got %d configs, want 0", len(configs))
	}

	// TC-CS-008: ListAll with data
	_ = store.Upsert("key1", "value1")
	_ = store.Upsert("key2", "value2")
	_ = store.Upsert("key3", "value3")

	configs, err = store.ListAll()
	if err != nil {
		t.Fatalf("TC-CS-008: unexpected error: %v", err)
	}
	if len(configs) != 3 {
		t.Fatalf("TC-CS-008: got %d configs, want 3", len(configs))
	}
	if configs["key1"] != "value1" {
		t.Fatalf("TC-CS-008: key1 got %q, want %q", configs["key1"], "value1")
	}
	if configs["key2"] != "value2" {
		t.Fatalf("TC-CS-008: key2 got %q, want %q", configs["key2"], "value2")
	}
}

func TestDBConfigStore_Clear(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-009: Clear store
	_ = store.Upsert("key1", "value1")
	_ = store.Upsert("key2", "value2")

	if err := store.Clear(); err != nil {
		t.Fatalf("TC-CS-009: unexpected error: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("TC-CS-009: unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("TC-CS-009: got %d, want 0 after clear", count)
	}
}

func TestDBConfigStore_JSONValue(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-010: Store and retrieve JSON value
	jsonValue := `{"key": "value", "number": 42}`
	if err := store.Upsert("json_config", jsonValue); err != nil {
		t.Fatalf("TC-CS-010: unexpected error: %v", err)
	}

	val, err := store.Get("json_config")
	if err != nil {
		t.Fatalf("TC-CS-010: failed to get config: %v", err)
	}
	if val != jsonValue {
		t.Fatalf("TC-CS-010: got %q, want %q", val, jsonValue)
	}
}

func TestDBConfigStore_LargeValue(t *testing.T) {
	db := initTestSQLiteDB(t)
	applyConfigStoreMigration(t, db)

	store := &DBConfigStore{db: db}

	// TC-CS-011: Store large value (10KB)
	largeValue := make([]byte, 10240)
	for i := range largeValue {
		largeValue[i] = 'a'
	}
	if err := store.Upsert("large_key", string(largeValue)); err != nil {
		t.Fatalf("TC-CS-011: unexpected error: %v", err)
	}

	val, err := store.Get("large_key")
	if err != nil {
		t.Fatalf("TC-CS-011: failed to get config: %v", err)
	}
	if len(val) != 10240 {
		t.Fatalf("TC-CS-011: got length %d, want 10240", len(val))
	}
}

package database

import (
	"os"
	"strings"
	"testing"
)

func TestMigration031032_PricingFilesExist(t *testing.T) {
	files := []string{
		"migrations/031_pricing_rules.postgresql.sql",
		"migrations/031_pricing_rules.sqlite.sql",
		"migrations/032_token_usage_billing.postgresql.sql",
		"migrations/032_token_usage_billing.sqlite.sql",
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(data), "-- +migrate Up") {
			t.Fatalf("%s missing migrate Up marker", f)
		}
	}
}

func TestMigration031_SQLiteIdempotentApply(t *testing.T) {
	db := initTestSQLiteDB(t)
	sqlBytes, err := os.ReadFile("migrations/031_pricing_rules.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := extractMigrateUpSQL(string(sqlBytes))
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("second apply (idempotent): %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pricing_rules`).Scan(&n); err != nil {
		t.Fatal(err)
	}
}

func TestMigration032_SQLiteAddsColumns(t *testing.T) {
	db := initTestSQLiteDB(t)
	_, err := db.Exec(`
		CREATE TABLE token_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			backend_id TEXT NOT NULL,
			model TEXT NOT NULL,
			cost_usd REAL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	sqlBytes, err := os.ReadFile("migrations/032_token_usage_billing.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := extractMigrateUpSQL(string(sqlBytes))
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("apply 032: %v", err)
	}
	_, err = db.Exec(`INSERT INTO token_usage (user_id, backend_id, model, input_cost, output_cost, pricing_rule_id) VALUES (1,'b','m',1,2,3)`)
	if err != nil {
		t.Fatalf("insert with new columns: %v", err)
	}
}

func extractMigrateUpSQL(content string) string {
	parts := strings.Split(content, "-- +migrate")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "Up") {
			return strings.TrimSpace(strings.TrimPrefix(part, "Up"))
		}
	}
	return content
}

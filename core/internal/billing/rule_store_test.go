package billing

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRuleStore_MemoryCRUDAndYAML(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryRuleStore()

	rule := &PricingRule{
		Name: "t", BackendID: "b1", Model: "m1",
		InputPricePerM: 1, OutputPricePerM: 2, Priority: 10, Enabled: true,
	}
	if err := s.CreateRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if rule.ID == 0 {
		t.Fatal("expected id")
	}
	got, err := s.GetRule(ctx, rule.ID)
	if err != nil || got.Model != "m1" {
		t.Fatalf("get: %+v err=%v", got, err)
	}

	rule.OutputPricePerM = 3
	if err := s.UpdateRule(ctx, rule.ID, rule); err != nil {
		t.Fatal(err)
	}
	yamlBytes, err := s.ExportToYAML(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s2 := NewMemoryRuleStore()
	if err := s2.ImportFromYAML(ctx, yamlBytes); err != nil {
		t.Fatal(err)
	}
	n, err := s2.CountRules(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if err := s.DeleteRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRuleStore_MemoryConcurrent(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryRuleStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = s.CreateRule(ctx, &PricingRule{
				Name: "c", BackendID: "b", Model: "m",
				InputPricePerM: float64(i), OutputPricePerM: 1, Enabled: true,
			})
			_, _ = s.ListRules(ctx)
		}(i)
	}
	wg.Wait()
	n, err := s.CountRules(ctx)
	if err != nil || n != 50 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func setupPricingRulesSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE pricing_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			backend_id TEXT NOT NULL,
			model TEXT NOT NULL,
			input_price_per_m REAL NOT NULL,
			output_price_per_m REAL NOT NULL,
			currency TEXT DEFAULT 'USD',
			priority INTEGER DEFAULT 0,
			enabled INTEGER DEFAULT 1,
			created_at TEXT,
			updated_at TEXT
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRuleStore_SQLCRUD(t *testing.T) {
	db := setupPricingRulesSQLite(t)
	defer db.Close()
	ctx := context.Background()
	s := NewSQLRuleStore(db, "sqlite")

	rule := &PricingRule{
		Name: "sql", BackendID: "ppinfra", Model: "deepseek-v3.2",
		InputPricePerM: 1, OutputPricePerM: 1, Priority: 100, Enabled: true,
	}
	if err := s.CreateRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListRules(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	yamlData := []byte(`
version: "1.0"
currency: "USD"
usd_to_cny: 7.2
rules:
  - name: "a"
    backend_id: "ollama-local"
    model: "*"
    input_price_per_m: 0
    output_price_per_m: 0
    priority: 0
`)
	if err := s.ImportFromYAML(ctx, yamlData); err != nil {
		t.Fatal(err)
	}
	n, err := s.CountRules(ctx)
	if err != nil || n != 1 {
		t.Fatalf("after import count=%d err=%v", n, err)
	}
}

func TestPricingRuleStore(t *testing.T) {
	TestRuleStore_MemoryCRUDAndYAML(t)
	TestRuleStore_SQLCRUD(t)
}

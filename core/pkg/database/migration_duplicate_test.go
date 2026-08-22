package database

import "testing"

// TestCheckDuplicateVersions 防回归：不同来源的同号迁移（如 pro 与 open-core
// 各自的 034_*）必须在 Migrate 启动时直接报错，而不是静默跳过或 UNIQUE 冲突。
func TestCheckDuplicateVersions(t *testing.T) {
	migrations := []Migration{
		{Version: "001", Name: "init"},
		{Version: "034", Name: "token_usage_revenue"},
		{Version: "034", Name: "billing_deepening"},
		{Version: "041", Name: "latest"},
	}

	err := checkDuplicateVersions(migrations)
	if err == nil {
		t.Fatal("duplicate versions must be rejected")
	}
	wantSub := `duplicate migration version 034`
	if got := err.Error(); len(got) < len(wantSub) || got[:len(wantSub)] != wantSub {
		t.Fatalf("error %q must mention %q", err, wantSub)
	}
	for _, name := range []string{"token_usage_revenue", "billing_deepening"} {
		for i := 0; i+len(name) <= len(err.Error()); i++ {
			found := err.Error()[i:i+len(name)] == name
			if found {
				break
			}
			if i+len(name) == len(err.Error()) {
				t.Fatalf("error %q must name both conflicting files (missing %s)", err, name)
			}
		}
	}

	clean := []Migration{
		{Version: "001", Name: "init"},
		{Version: "002", Name: "next"},
	}
	if err := checkDuplicateVersions(clean); err != nil {
		t.Fatalf("unique versions must pass, got %v", err)
	}
}

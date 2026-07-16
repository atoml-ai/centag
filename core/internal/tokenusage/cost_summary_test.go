package tokenusage

import "testing"

func TestNormalizeCostGroupBy(t *testing.T) {
	got, err := normalizeCostGroupBy("backend")
	if err != nil || got != "backend" {
		t.Fatalf("backend: got=%q err=%v", got, err)
	}
	got, err = normalizeCostGroupBy("dept")
	if err != nil || got != "dept" {
		t.Fatalf("dept: got=%q err=%v", got, err)
	}
	if _, err := normalizeCostGroupBy("invalid"); err == nil {
		t.Fatal("expected error for invalid group_by")
	}
}
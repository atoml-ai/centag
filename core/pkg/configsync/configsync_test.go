package configsync

import (
	"strings"
	"testing"
	"time"
)

// ---------- B. 校验层（TC-VAL-001~010） ----------

func TestValidation(t *testing.T) {
	t.Run("TC-VAL-001_合法行通过", func(t *testing.T) {
		row := Row{Key: "table.model_price", Edition: "all", Value: []byte(`{"provider":"feishu"}`), Enabled: true}
		if err := ValidateConfigRow(&row); err != nil {
			t.Fatalf("valid row rejected: %v", err)
		}
	})

	t.Run("TC-VAL-002_价格下界拒收", func(t *testing.T) {
		for _, p := range []float64{0, -1} {
			row := ProviderPrice{BaseURL: "https://x", ProviderName: "T", Models: []ModelPrice{{Model: "m", InputPricePerM: p}}}
			if err := ValidatePriceRow(&row); err == nil {
				t.Fatalf("price %v should be rejected", p)
			}
		}
	})

	t.Run("TC-VAL-003_价格上界拒收", func(t *testing.T) {
		row := ProviderPrice{BaseURL: "https://x", ProviderName: "T", Models: []ModelPrice{{Model: "m", InputPricePerM: 10001}}}
		if err := ValidatePriceRow(&row); err == nil {
			t.Fatal("price 10001 should be rejected")
		}
	})

	t.Run("TC-VAL-004_坏版本区间拒收", func(t *testing.T) {
		if err := ValidateConfigRow(&Row{Key: "k", Edition: "all", MinVersion: "0.4.0", MaxVersion: "0.3.0"}); err == nil {
			t.Fatal("min>max should be rejected")
		}
		if err := ValidateConfigRow(&Row{Key: "k", Edition: "all", MinVersion: "abc"}); err == nil {
			t.Fatal("non-semver min should be rejected")
		}
	})

	t.Run("TC-VAL-005_超大载荷拒收", func(t *testing.T) {
		row := Row{Key: "k", Edition: "all", Value: make([]byte, 1<<20+1)}
		if err := ValidateConfigRow(&row); err == nil {
			t.Fatal("1MiB+1 payload should be rejected")
		}
	})

	t.Run("TC-VAL-006_未知value_schema拒收", func(t *testing.T) {
		// release.* without version
		if err := ValidateConfigRow(&Row{Key: "release.channel.stable", Edition: "all", Value: []byte(`{"pkg":"x"}`)}); err == nil {
			t.Fatal("release.* without version should be rejected")
		}
		// table.* with non-object
		if err := ValidateConfigRow(&Row{Key: "table.model_price", Edition: "all", Value: []byte(`[1,2]`)}); err == nil {
			t.Fatal("table.* non-object should be rejected")
		}
		// valid release row passes
		row := Row{Key: "release.channel.stable", Edition: "all", Channel: "stable", Value: []byte(`{"version":"0.3.4"}`)}
		if err := ValidateConfigRow(&row); err != nil {
			t.Fatalf("valid release row rejected: %v", err)
		}
	})

	t.Run("TC-VAL-007_畸形JSON拒收", func(t *testing.T) {
		row := Row{Key: "k", Edition: "all", Value: []byte(`{broken`)}
		if err := ValidateConfigRow(&row); err == nil {
			t.Fatal("malformed JSON should be rejected")
		}
	})

	t.Run("TC-VAL-008_一行非法整批拒收", func(t *testing.T) {
		rows := []Row{
			{Key: "k1", Edition: "all", Value: []byte(`{}`), Enabled: true},
			{Key: "k2", Edition: "all", MinVersion: "9.9.9", MaxVersion: "0.0.1", Enabled: true},
			{Key: "k3", Edition: "all", Value: []byte(`{}`), Enabled: true},
		}
		err := ValidateRows(rows)
		if err == nil {
			t.Fatal("batch with one invalid row must be rejected wholesale")
		}
		if !strings.Contains(err.Error(), "row 1") {
			t.Fatalf("error should identify offending row: %v", err)
		}
	})

	t.Run("TC-VAL-009_数值注入拒收", func(t *testing.T) {
		row := Row{Key: "feature.x", Edition: "all", Value: []byte(`"1;drop table"`)}
		if err := ValidateConfigRow(&row); err != nil {
			// string JSON is valid JSON but harmless; the injection vector is
			// non-JSON payloads which must fail:
			t.Fatalf("quoted string should remain valid JSON: %v", row.Key)
		}
		bad := Row{Key: "feature.x", Edition: "all", Value: []byte(`1;drop table`)}
		if err := ValidateConfigRow(&bad); err == nil {
			t.Fatal("injection payload must be rejected as invalid JSON")
		}
	})

	t.Run("TC-VAL-010_空批次合法且不覆盖", func(t *testing.T) {
		if err := ValidateRows(nil); err != nil {
			t.Fatalf("empty batch must be valid: %v", err)
		}
	})
}

// ---------- C. 版本区间匹配（TC-MAT-001~010） ----------

func matRow(mut func(*Row)) *Row {
	r := Row{Key: "table.model_price", Edition: "all", Enabled: true, Priority: 5}
	if mut != nil {
		mut(&r)
	}
	return &r
}

func TestVersionMatching(t *testing.T) {
	t.Run("TC-MAT-001_闭区间含端点", func(t *testing.T) {
		r := matRow(func(r *Row) { r.MinVersion = "0.3.4" })
		if !MatchVersion(r, Query{Edition: "team", Version: "0.3.4"}) {
			t.Fatal("lower endpoint must match (inclusive)")
		}
		r2 := matRow(func(r *Row) { r.MaxVersion = "0.3.4" })
		if !MatchVersion(r2, Query{Edition: "team", Version: "0.3.4"}) {
			t.Fatal("upper endpoint must match (inclusive)")
		}
	})

	t.Run("TC-MAT-002_空端无界", func(t *testing.T) {
		r := matRow(func(r *Row) { r.MinVersion = "0.3.0" })
		if !MatchVersion(r, Query{Edition: "team", Version: "9.9.9"}) {
			t.Fatal("empty max must be unbounded")
		}
	})

	t.Run("TC-MAT-003_edition_all命中", func(t *testing.T) {
		if !MatchVersion(matRow(nil), Query{Edition: "team"}) {
			t.Fatal("edition=all must match any client edition")
		}
	})

	t.Run("TC-MAT-004_edition不匹配排除", func(t *testing.T) {
		r := matRow(func(r *Row) { r.Edition = "personal" })
		if MatchVersion(r, Query{Edition: "team"}) {
			t.Fatal("personal row must not match team client")
		}
	})

	t.Run("TC-MAT-005_channel不匹配排除", func(t *testing.T) {
		r := matRow(func(r *Row) { r.Key = "release.channel.stable"; r.Channel = "beta" })
		if MatchVersion(r, Query{Edition: "team", Version: "0.3.4", Channel: "stable"}) {
			t.Fatal("beta release row must not match stable build")
		}
	})

	t.Run("TC-MAT-006_priority择优", func(t *testing.T) {
		rows := []Row{
			*matRow(func(r *Row) { r.Priority = 5 }),
			*matRow(func(r *Row) { r.Priority = 10 }),
		}
		if best := SelectBestRow(rows); best.Priority != 10 {
			t.Fatalf("priority 10 must win, got %d", best.Priority)
		}
	})

	t.Run("TC-MAT-007_平手取最新", func(t *testing.T) {
		old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		new := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		rows := []Row{
			*matRow(func(r *Row) { r.UpdatedAt = old }),
			*matRow(func(r *Row) { r.UpdatedAt = new }),
		}
		if best := SelectBestRow(rows); !best.UpdatedAt.Equal(new) {
			t.Fatalf("newest updated_at must win, got %v", best.UpdatedAt)
		}
	})

	t.Run("TC-MAT-008_disabled排除", func(t *testing.T) {
		r := matRow(func(r *Row) { r.Enabled = false })
		if MatchVersion(r, Query{Edition: "team"}) {
			t.Fatal("disabled row must be excluded")
		}
	})

	t.Run("TC-MAT-009_无命中返回空", func(t *testing.T) {
		rows := []Row{*matRow(func(r *Row) { r.MinVersion = "9.0.0" })}
		matched := []Row{}
		for i := range rows {
			if MatchVersion(&rows[i], Query{Edition: "team", Version: "0.3.4"}) {
				matched = append(matched, rows[i])
			}
		}
		if len(matched) != 0 || SelectBestRow(matched) != nil {
			t.Fatal("no match must yield empty selection")
		}
	})

	t.Run("TC-MAT-010_dev与空版本语义", func(t *testing.T) {
		if CompareVersions("dev", "0.3.4") >= 0 || CompareVersions("", "0.3.4") >= 0 {
			t.Fatal("dev/empty must compare below any numeric version")
		}
		if CompareVersions("v0.3.4", "0.3.4") != 0 {
			t.Fatal("v-prefix must be tolerated")
		}
		r := matRow(func(r *Row) { r.MinVersion = "9.9.9" })
		if !MatchVersion(r, Query{Edition: "team", Version: "dev"}) {
			t.Fatal("dev client matches any range (no update gating for dev builds)")
		}
	})
}

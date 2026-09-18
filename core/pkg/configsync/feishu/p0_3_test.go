package feishu

import (
	"context"
	"testing"

	"centag/core/pkg/configsync"
)

// 未配置相应 table ID 时必须显式返回 ErrNotSupported，让调度器保留 last-good，
// 而不是静默当作空表（P0-3）。
func TestFetchSystemConfigRows_NotConfigured(t *testing.T) {
	p := NewProvider(ProviderConfig{})
	if _, err := p.FetchSystemConfigRows(context.Background()); err != configsync.ErrNotSupported {
		t.Fatalf("err = %v, want ErrNotSupported", err)
	}
	if _, err := p.FetchMITMDomainsRows(context.Background()); err != configsync.ErrNotSupported {
		t.Fatalf("err = %v, want ErrNotSupported", err)
	}
}

func TestParseKVRow(t *testing.T) {
	row := parseKVRow(Record{Fields: map[string]any{
		"key": "billing.usd_to_cny", "value": "7.3", "value_type": "number",
		"scope": "core", "enabled": "true", "description": "fx",
	}})
	if row == nil || row.Key != "billing.usd_to_cny" || row.Value != "7.3" || !row.Enabled {
		t.Fatalf("row=%+v", row)
	}
	if parseKVRow(Record{Fields: map[string]any{"value": "x"}}) != nil {
		t.Fatalf("row without key must be skipped")
	}
}

func TestParseMITMDomainRow(t *testing.T) {
	row := parseMITMDomainRow(Record{Fields: map[string]any{
		"domain": "api.openai.com", "category": "llm", "enabled": true, "remark": "openai",
	}})
	if row == nil || row.Domain != "api.openai.com" || !row.Enabled {
		t.Fatalf("row=%+v", row)
	}
	if parseMITMDomainRow(Record{Fields: map[string]any{"category": "llm"}}) != nil {
		t.Fatalf("row without domain must be skipped")
	}
}

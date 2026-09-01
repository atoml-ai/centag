package configsync

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"centag/core/pkg/feishu"
	"centag/core/pkg/logger"
)

func TestMain(m *testing.M) {
	_ = logger.Init(logger.Config{
		Level:  "error",
		Format: "console",
		Output: "stdout",
	})
	os.Exit(m.Run())
}

func TestFeishuRecordToConfigRow(t *testing.T) {
	// TC-FPR-001: Valid config row
	rec := feishu.Record{
		Fields: map[string]interface{}{
			"config_key":  "test.key",
			"value":       `{"enabled": true}`,
			"edition":     "all",
			"channel":     "stable",
			"min_version": "1.0.0",
			"max_version": "2.0.0",
			"priority":    float64(10),
			"enabled":     true,
			"updated_at":  float64(1693500000000),
			"remark":      "test remark",
		},
	}
	row := feishuRecordToConfigRow(rec)
	if row == nil {
		t.Fatal("TC-FPR-001: expected non-nil row")
	}
	if row.Key != "test.key" {
		t.Fatalf("TC-FPR-001: got key %q, want %q", row.Key, "test.key")
	}
	if row.Edition != "all" {
		t.Fatalf("TC-FPR-001: got edition %q, want %q", row.Edition, "all")
	}
	if row.Channel != "stable" {
		t.Fatalf("TC-FPR-001: got channel %q, want %q", row.Channel, "stable")
	}
	if row.MinVersion != "1.0.0" {
		t.Fatalf("TC-FPR-001: got min_version %q, want %q", row.MinVersion, "1.0.0")
	}
	if row.MaxVersion != "2.0.0" {
		t.Fatalf("TC-FPR-001: got max_version %q, want %q", row.MaxVersion, "2.0.0")
	}
	if row.Priority != 10 {
		t.Fatalf("TC-FPR-001: got priority %d, want 10", row.Priority)
	}
	if !row.Enabled {
		t.Fatal("TC-FPR-001: expected enabled to be true")
	}
	if row.Remark != "test remark" {
		t.Fatalf("TC-FPR-001: got remark %q, want %q", row.Remark, "test remark")
	}

	// TC-FPR-002: Empty config_key returns nil
	rec2 := feishu.Record{
		Fields: map[string]interface{}{
			"config_key": "",
		},
	}
	row2 := feishuRecordToConfigRow(rec2)
	if row2 != nil {
		t.Fatal("TC-FPR-002: expected nil row for empty config_key")
	}

	// TC-FPR-003: Missing config_key returns nil
	rec3 := feishu.Record{
		Fields: map[string]interface{}{},
	}
	row3 := feishuRecordToConfigRow(rec3)
	if row3 != nil {
		t.Fatal("TC-FPR-003: expected nil row for missing config_key")
	}

	// TC-FPR-004: updated_at as RFC3339 string
	rec4 := feishu.Record{
		Fields: map[string]interface{}{
			"config_key": "test2",
			"updated_at": "2023-09-01T12:00:00Z",
		},
	}
	row4 := feishuRecordToConfigRow(rec4)
	if row4 == nil {
		t.Fatal("TC-FPR-004: expected non-nil row")
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2023-09-01T12:00:00Z")
	if !row4.UpdatedAt.Equal(expectedTime) {
		t.Fatalf("TC-FPR-004: got updated_at %v, want %v", row4.UpdatedAt, expectedTime)
	}

	// TC-FPR-005: disabled row
	rec5 := feishu.Record{
		Fields: map[string]interface{}{
			"config_key": "disabled.key",
			"enabled":    false,
		},
	}
	row5 := feishuRecordToConfigRow(rec5)
	if row5 == nil {
		t.Fatal("TC-FPR-005: expected non-nil row")
	}
	if row5.Enabled {
		t.Fatal("TC-FPR-005: expected enabled to be false")
	}
}

func TestFeishuRecordToProviderPrice(t *testing.T) {
	// TC-FPR-006: Valid provider price
	modelsJSON := `[{"model":"gpt-4","input_price_per_m":30,"output_price_per_m":60}]`
	rec := feishu.Record{
		Fields: map[string]interface{}{
			"base_url":      "https://api.openai.com/v1",
			"provider_name": "OpenAI",
			"currency":      "USD",
			"models":        modelsJSON,
			"enabled":       true,
		},
	}
	pp := feishuRecordToProviderPrice(rec)
	if pp == nil {
		t.Fatal("TC-FPR-006: expected non-nil provider price")
	}
	if pp.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("TC-FPR-006: got base_url %q, want %q", pp.BaseURL, "https://api.openai.com/v1")
	}
	if pp.ProviderName != "OpenAI" {
		t.Fatalf("TC-FPR-006: got provider_name %q, want %q", pp.ProviderName, "OpenAI")
	}
	if pp.Currency != "USD" {
		t.Fatalf("TC-FPR-006: got currency %q, want %q", pp.Currency, "USD")
	}
	if len(pp.Models) != 1 {
		t.Fatalf("TC-FPR-006: got %d models, want 1", len(pp.Models))
	}
	if pp.Models[0].Model != "gpt-4" {
		t.Fatalf("TC-FPR-006: got model %q, want %q", pp.Models[0].Model, "gpt-4")
	}
	if !pp.Enabled {
		t.Fatal("TC-FPR-006: expected enabled to be true")
	}

	// TC-FPR-007: Empty base_url returns nil
	rec2 := feishu.Record{
		Fields: map[string]interface{}{
			"base_url": "",
		},
	}
	pp2 := feishuRecordToProviderPrice(rec2)
	if pp2 != nil {
		t.Fatal("TC-FPR-007: expected nil for empty base_url")
	}

	// TC-FPR-008: Invalid models JSON returns nil
	rec3 := feishu.Record{
		Fields: map[string]interface{}{
			"base_url": "https://api.example.com",
			"models":   "invalid json",
		},
	}
	pp3 := feishuRecordToProviderPrice(rec3)
	if pp3 != nil {
		t.Fatal("TC-FPR-008: expected nil for invalid models JSON")
	}

	// TC-FPR-009: Empty models field
	rec4 := feishu.Record{
		Fields: map[string]interface{}{
			"base_url": "https://api.example.com",
			"models":   "",
		},
	}
	pp4 := feishuRecordToProviderPrice(rec4)
	if pp4 == nil {
		t.Fatal("TC-FPR-009: expected non-nil provider price")
	}
	if len(pp4.Models) != 0 {
		t.Fatalf("TC-FPR-009: got %d models, want 0", len(pp4.Models))
	}
}

func TestFeishuRecordToPipelineTemplate(t *testing.T) {
	// TC-FPR-010: Valid pipeline template
	contentJSON := `{
		"name": "Test Pipeline",
		"description": "A test pipeline",
		"nodes": [{"id": "node1", "type": "llm"}]
	}`
	rec := feishu.Record{
		Fields: map[string]interface{}{
			"pipeline_id":  "test-pipeline",
			"content_json": contentJSON,
			"enabled":      true,
		},
	}
	tmpl := feishuRecordToPipelineTemplate(rec)
	if tmpl == nil {
		t.Fatal("TC-FPR-010: expected non-nil template")
	}
	if tmpl.ID != "test-pipeline" {
		t.Fatalf("TC-FPR-010: got id %q, want %q", tmpl.ID, "test-pipeline")
	}
	if tmpl.Name != "Test Pipeline" {
		t.Fatalf("TC-FPR-010: got name %q, want %q", tmpl.Name, "Test Pipeline")
	}
	if len(tmpl.Nodes) != 1 {
		t.Fatalf("TC-FPR-010: got %d nodes, want 1", len(tmpl.Nodes))
	}

	// TC-FPR-011: Empty pipeline_id returns nil
	rec2 := feishu.Record{
		Fields: map[string]interface{}{
			"pipeline_id": "",
			"enabled":     true,
		},
	}
	tmpl2 := feishuRecordToPipelineTemplate(rec2)
	if tmpl2 != nil {
		t.Fatal("TC-FPR-011: expected nil for empty pipeline_id")
	}

	// TC-FPR-012: Disabled template returns nil
	rec3 := feishu.Record{
		Fields: map[string]interface{}{
			"pipeline_id": "test-pipeline",
			"enabled":     false,
		},
	}
	tmpl3 := feishuRecordToPipelineTemplate(rec3)
	if tmpl3 != nil {
		t.Fatal("TC-FPR-012: expected nil for disabled template")
	}

	// TC-FPR-013: Invalid content_json returns nil
	rec4 := feishu.Record{
		Fields: map[string]interface{}{
			"pipeline_id":  "test-pipeline",
			"content_json": "invalid json",
			"enabled":      true,
		},
	}
	tmpl4 := feishuRecordToPipelineTemplate(rec4)
	if tmpl4 != nil {
		t.Fatal("TC-FPR-013: expected nil for invalid content_json")
	}

	// TC-FPR-014: Empty content_json creates template with only ID
	rec5 := feishu.Record{
		Fields: map[string]interface{}{
			"pipeline_id":  "test-pipeline",
			"content_json": "",
			"enabled":      true,
		},
	}
	tmpl5 := feishuRecordToPipelineTemplate(rec5)
	if tmpl5 == nil {
		t.Fatal("TC-FPR-014: expected non-nil template")
	}
	if tmpl5.ID != "test-pipeline" {
		t.Fatalf("TC-FPR-014: got id %q, want %q", tmpl5.ID, "test-pipeline")
	}
	if tmpl5.Name != "" {
		t.Fatalf("TC-FPR-014: got name %q, want empty", tmpl5.Name)
	}
}

func TestFeishuRecordToConfigRow_JSONValue(t *testing.T) {
	// TC-FPR-015: JSON value parsing
	rec := feishu.Record{
		Fields: map[string]interface{}{
			"config_key": "json.key",
			"value":      `{"nested": {"key": "value"}}`,
		},
	}
	row := feishuRecordToConfigRow(rec)
	if row == nil {
		t.Fatal("TC-FPR-015: expected non-nil row")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(row.Value, &result); err != nil {
		t.Fatalf("TC-FPR-015: failed to unmarshal value: %v", err)
	}
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("TC-FPR-015: expected nested object")
	}
	if nested["key"] != "value" {
		t.Fatalf("TC-FPR-015: got nested.key %v, want value", nested["key"])
	}
}

func TestNormalizeBaseURLFeishu(t *testing.T) {
	// TC-FPR-016: Strip trailing slash
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.example.com/", "https://api.example.com"},
		{"https://api.example.com//", "https://api.example.com"},
		{"https://api.example.com", "https://api.example.com"},
		{"https://api.example.com/v1/", "https://api.example.com/v1"},
		{"", ""},
	}

	for i, tt := range tests {
		result := NormalizeBaseURLFeishu(tt.input)
		if result != tt.expected {
			t.Errorf("TC-FPR-016-%d: got %q, want %q", i, result, tt.expected)
		}
	}
}

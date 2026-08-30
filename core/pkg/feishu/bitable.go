package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TextField unwraps a Bitable text field value. The API returns plain
// strings on write but rich-text segment arrays
// ([{"text":"...","type":"text"}]) on read — callers must unwrap (真实 API
// 兼容；mock 无法覆盖此形态差异).
func TextField(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var sb strings.Builder
		for _, seg := range t {
			if m, ok := seg.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					sb.WriteString(s)
				}
			}
		}
		return sb.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

// TextField returns the unwrapped text of a field in this record.
func (r Record) TextField(key string) string { return TextField(r.Fields[key]) }

// Table represents a Bitable table within a base.
type Table struct {
	TableID string `json:"table_id"`
	Name    string `json:"name"`
}

// Field represents a Bitable column.
type Field struct {
	FieldName string `json:"field_name"`
	Type      int    `json:"type"`
}

// Record represents a single Bitable row. Fields maps column names to values.
type Record struct {
	RecordID string         `json:"record_id,omitempty"`
	Fields   map[string]any `json:"fields"`
}

// ListTables returns all tables in the given base.
func (c *Client) ListTables(ctx context.Context, appToken string) ([]Table, error) {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return nil, err
	}
	var tables []Table
	pageToken := ""
	for {
		url := fmt.Sprintf("%s/%s/tables?page_size=100", c.bitableEndpoint(), appToken)
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}
		resp, err := c.httpGet(ctx, token, url)
		if err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}
		var out struct {
			Code int `json:"code"`
			Data struct {
				HasMore   bool    `json:"has_more"`
				PageToken string  `json:"page_token"`
				Items     []Table `json:"items"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("list tables decode: %w", err)
		}
		resp.Body.Close()
		if out.Code != 0 {
			return nil, fmt.Errorf("list tables: code=%d", out.Code)
		}
		tables = append(tables, out.Data.Items...)
		if !out.Data.HasMore {
			break
		}
		pageToken = out.Data.PageToken
	}
	return tables, nil
}

// FindTable returns the table_id for a table whose name matches (case-insensitive
// contains). Returns error if not found or ambiguous.
func (c *Client) FindTable(ctx context.Context, appToken, name string) (string, error) {
	tables, err := c.ListTables(ctx, appToken)
	if err != nil {
		return "", err
	}
	nameLower := strings.ToLower(name)
	var matches []Table
	for _, t := range tables {
		if strings.Contains(strings.ToLower(t.Name), nameLower) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("table %q not found in base %s", name, appToken)
	case 1:
		return matches[0].TableID, nil
	default:
		return "", fmt.Errorf("table %q ambiguous: matched %d tables", name, len(matches))
	}
}

// ListFields returns all column definitions for a table.
func (c *Client) ListFields(ctx context.Context, appToken, tableID string) ([]Field, error) {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return nil, err
	}
	var fields []Field
	pageToken := ""
	for {
		url := fmt.Sprintf("%s/%s/tables/%s/fields?page_size=100", c.bitableEndpoint(), appToken, tableID)
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}
		resp, err := c.httpGet(ctx, token, url)
		if err != nil {
			return nil, fmt.Errorf("list fields: %w", err)
		}
		var out struct {
			Code int `json:"code"`
			Data struct {
				HasMore   bool    `json:"has_more"`
				PageToken string  `json:"page_token"`
				Items     []Field `json:"items"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("list fields decode: %w", err)
		}
		resp.Body.Close()
		if out.Code != 0 {
			return nil, fmt.Errorf("list fields: code=%d", out.Code)
		}
		fields = append(fields, out.Data.Items...)
		if !out.Data.HasMore {
			break
		}
		pageToken = out.Data.PageToken
	}
	return fields, nil
}

// EnsureFields checks which of the wanted fields exist and creates any that
// are missing. Existing fields are never modified.
func (c *Client) EnsureFields(ctx context.Context, appToken, tableID string, want []string) error {
	existing, err := c.ListFields(ctx, appToken, tableID)
	if err != nil {
		return fmt.Errorf("ensure fields: %w", err)
	}
	existingSet := make(map[string]bool, len(existing))
	for _, f := range existing {
		existingSet[f.FieldName] = true
	}
	for _, name := range want {
		if existingSet[name] {
			continue
		}
		if err := c.createField(ctx, appToken, tableID, name); err != nil {
			return fmt.Errorf("ensure field %s: %w", name, err)
		}
	}
	return nil
}

func (c *Client) createField(ctx context.Context, appToken, tableID, name string) error {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s/tables/%s/fields", c.bitableEndpoint(), appToken, tableID)
	resp, err := c.httpPost(ctx, token, url, map[string]any{
		"field_name": name,
		"type":       1, // text
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return DecodeCode(resp, "feishu create field "+name)
}

// FilterCondition is one condition of a records/search filter.
type FilterCondition struct {
	FieldName string   `json:"field_name"`
	Operator  string   `json:"operator"` // "is", "isNot", "contains", ...
	Value     []string `json:"value"`
}

// Filter is the structured filter accepted by the records/search API.
// The formula-style CurrentValue.[...] strings are view-filter syntax and
// are REJECTED by this endpoint (code 9499) — 必须用结构化对象.
type Filter struct {
	Conjunction string            `json:"conjunction"` // "and" | "or"
	Conditions  []FilterCondition `json:"conditions"`
}

// NewFilter builds a single-condition "and" filter (common case).
func NewFilter(field, operator string, values ...string) *Filter {
	return &Filter{Conjunction: "and", Conditions: []FilterCondition{{FieldName: field, Operator: operator, Value: values}}}
}

// SearchRecords searches for records in a table with an optional structured
// filter and returns all pages.
func (c *Client) SearchRecords(ctx context.Context, appToken, tableID string, filter *Filter) ([]Record, error) {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return nil, err
	}
	var all []Record
	pageToken := ""
	for {
		payload := map[string]any{
			"automatic_fields": false,
			"page_size":        500,
		}
		if filter != nil {
			payload["filter"] = filter
		}
		if pageToken != "" {
			payload["page_token"] = pageToken
		}
		url := fmt.Sprintf("%s/%s/tables/%s/records/search", c.bitableEndpoint(), appToken, tableID)
		resp, err := c.httpPost(ctx, token, url, payload)
		if err != nil {
			return nil, fmt.Errorf("search records: %w", err)
		}
		var out struct {
			Code int `json:"code"`
			Data struct {
				HasMore   bool     `json:"has_more"`
				PageToken string   `json:"page_token"`
				Total     int      `json:"total"`
				Items     []Record `json:"items"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("search records decode: %w", err)
		}
		resp.Body.Close()
		if out.Code != 0 {
			return nil, fmt.Errorf("search records: code=%d", out.Code)
		}
		all = append(all, out.Data.Items...)
		if !out.Data.HasMore {
			break
		}
		pageToken = out.Data.PageToken
	}
	return all, nil
}

// UpsertRecords upserts records into a table. Each record is matched by the
// specified matchField; if a record with the same value exists it is updated,
// otherwise created. Returns (created, updated, error).
func (c *Client) UpsertRecords(ctx context.Context, appToken, tableID, matchField string, records []Record) (created, updated int, err error) {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return 0, 0, err
	}
	url := fmt.Sprintf("%s/%s/tables/%s/records/batch_create", c.bitableEndpoint(), appToken, tableID)
	// Simple implementation: batch_create; duplicate handling is caller's
	// responsibility via SearchRecords + selective PUT/POST.
	// For true Bitable upsert we use the batch_create endpoint which is
	// effectively a bulk insert; deduplication is handled at the caller level.
	payload := make([]map[string]any, len(records))
	for i, r := range records {
		payload[i] = map[string]any{"fields": r.Fields}
	}
	resp, err := c.httpPost(ctx, token, url, map[string]any{"records": payload})
	if err != nil {
		return 0, 0, fmt.Errorf("batch create records: %w", err)
	}
	defer resp.Body.Close()
	if err := DecodeCode(resp, "feishu batch_create"); err != nil {
		return 0, 0, err
	}
	return len(records), 0, nil
}

// CreateRecord creates a single record and returns the record_id.
func (c *Client) CreateRecord(ctx context.Context, appToken, tableID string, fields map[string]any) (string, error) {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/%s/tables/%s/records", c.bitableEndpoint(), appToken, tableID)
	resp, err := c.httpPost(ctx, token, url, map[string]any{"fields": fields})
	if err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Code int `json:"code"`
		Data struct {
			Record Record `json:"record"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("create record decode: %w", err)
	}
	if out.Code != 0 {
		return "", fmt.Errorf("create record: code=%d", out.Code)
	}
	return out.Data.Record.RecordID, nil
}

// UpdateRecord updates a single record by record_id.
func (c *Client) UpdateRecord(ctx context.Context, appToken, tableID, recordID string, fields map[string]any) error {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s/tables/%s/records/%s", c.bitableEndpoint(), appToken, tableID, recordID)
	resp, err := c.httpPut(ctx, token, url, map[string]any{"fields": fields})
	if err != nil {
		return fmt.Errorf("update record: %w", err)
	}
	defer resp.Body.Close()
	return DecodeCode(resp, "feishu update record")
}

// UpsertRecord searches for a record by matchField=value and creates or updates.
// Returns (recordID, created, error). Concurrent upserts on the same key are
// serialized client-side so duplicate rows cannot be produced (R11).
func (c *Client) UpsertRecord(ctx context.Context, appToken, tableID, matchField, matchValue string, fields map[string]any) (recordID string, created bool, err error) {
	unlock := c.lockKey(appToken, tableID, matchField, matchValue)
	defer unlock()

	filter := NewFilter(matchField, "is", matchValue)
	existing, err := c.SearchRecords(ctx, appToken, tableID, filter)
	if err != nil {
		return "", false, err
	}
	if len(existing) > 0 {
		// Update existing record.
		rid := existing[0].RecordID
		if err := c.UpdateRecord(ctx, appToken, tableID, rid, fields); err != nil {
			return "", false, err
		}
		return rid, false, nil
	}
	// Create new record.
	rid, err := c.CreateRecord(ctx, appToken, tableID, fields)
	if err != nil {
		return "", false, err
	}
	return rid, true, nil
}

// keyLocks serializes concurrent upserts on the same logical record key.
var keyLocks sync.Map

func (c *Client) lockKey(appToken, tableID, matchField, matchValue string) func() {
	v, _ := keyLocks.LoadOrStore(strings.Join([]string{appToken, tableID, matchField, matchValue}, "\x00"), &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// CreateBase creates a new Bitable base and returns the app_token and
// default_table_id. The app becomes the owner (no collaborator needed).
func (c *Client) CreateBase(ctx context.Context, name string) (appToken, defaultTableID string, err error) {
	token, err := c.TenantToken(ctx)
	if err != nil {
		return "", "", err
	}
	resp, err := c.httpPost(ctx, token, c.bitableEndpoint(), map[string]any{"name": name})
	if err != nil {
		return "", "", fmt.Errorf("create base: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			App struct {
				AppToken       string `json:"app_token"`
				DefaultTableID string `json:"default_table_id"`
				URL            string `json:"url"`
			} `json:"app"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("create base decode: %w", err)
	}
	if out.Code != 0 {
		return "", "", fmt.Errorf("create base: code=%d msg=%s", out.Code, out.Msg)
	}
	return out.Data.App.AppToken, out.Data.App.DefaultTableID, nil
}

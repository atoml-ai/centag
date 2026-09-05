// Package feishu provides a read-only Feishu Bitable Provider implementation
// for the configsync framework. It implements the configsync.Provider interface
// using the Feishu Bitable API with a read-only Client App.
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"centag/core/pkg/configsync"
)

// Provider implements configsync.Provider backed by Feishu Bitable.
// It uses a read-only Client App with bitable:app:readonly permission.
type Provider struct {
	appID           string
	appSecret       string
	appToken        string
	configTableID   string
	pricingTableID  string
	pipelineTableID string
	httpClient      *http.Client
	mu              sync.Mutex
	token           string
	tokenExp        time.Time
}

// ProviderConfig holds the configuration for the Feishu Provider.
type ProviderConfig struct {
	// Client credentials (read-only App)
	AppID     string
	AppSecret string
	
	// Bitable table IDs
	AppToken        string // Bitable App Token
	ConfigTableID   string // Table ID for system config
	PricingTableID  string // Table ID for model pricing
	PipelineTableID string // Table ID for pipeline templates
}

// NewProvider creates a new Feishu Provider.
func NewProvider(cfg ProviderConfig) *Provider {
	return &Provider{
		appID:           cfg.AppID,
		appSecret:       cfg.AppSecret,
		appToken:        cfg.AppToken,
		configTableID:   cfg.ConfigTableID,
		pricingTableID:  cfg.PricingTableID,
		pipelineTableID: cfg.PipelineTableID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsConfigured returns true if Feishu configsync is configured via environment variables.
func IsConfigured() bool {
	appID := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_CLIENT_APP_ID")
	appSecret := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_CLIENT_APP_SECRET")
	appToken := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_APP_TOKEN")
	
	return appID != "" && appSecret != "" && appToken != ""
}

// NewProviderFromEnv creates a new Feishu Provider from environment variables.
// Returns nil if not configured.
func NewProviderFromEnv() *Provider {
	if !IsConfigured() {
		return nil
	}
	return NewProvider(ProviderConfig{
		AppID:           os.Getenv("CENTAG_CONFIGSYNC_FEISHU_CLIENT_APP_ID"),
		AppSecret:       os.Getenv("CENTAG_CONFIGSYNC_FEISHU_CLIENT_APP_SECRET"),
		AppToken:        os.Getenv("CENTAG_CONFIGSYNC_FEISHU_APP_TOKEN"),
		ConfigTableID:   os.Getenv("CENTAG_CONFIGSYNC_FEISHU_CONFIG_TABLE_ID"),
		PricingTableID:  os.Getenv("CENTAG_CONFIGSYNC_FEISHU_PRICING_TABLE_ID"),
		PipelineTableID: os.Getenv("CENTAG_CONFIGSYNC_FEISHU_PIPELINE_TABLE_ID"),
	})
}

// GetTenantToken gets or refreshes the tenant access token.
func (p *Provider) GetTenantToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && time.Now().Before(p.tokenExp) {
		return p.token, nil
	}

	body := map[string]string{
		"app_id":     p.appID,
		"app_secret": p.appSecret,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Token  string `json:"tenant_access_token"`
		Expire int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("get tenant token failed: code=%d msg=%s", result.Code, result.Msg)
	}

	p.token = result.Token
	p.tokenExp = time.Now().Add(time.Duration(result.Expire-300) * time.Second)
	return p.token, nil
}

// SearchRecords searches records in a table with optional filter.
func (p *Provider) SearchRecords(ctx context.Context, tableID string, filter *Filter) ([]Record, error) {
	token, err := p.GetTenantToken(ctx)
	if err != nil {
		return nil, err
	}

	var allRecords []Record
	var pageToken string

	for {
		reqBody := SearchRecordsRequest{
			Filter:    filter,
			PageToken: pageToken,
			PageSize:  500,
		}
		data, _ := json.Marshal(reqBody)

		url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records/search",
			p.appToken, tableID)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var result SearchRecordsResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		if result.Code != 0 {
			return nil, fmt.Errorf("search records failed: code=%d msg=%s", result.Code, result.Msg)
		}

		allRecords = append(allRecords, result.Data.Items...)

		if !result.Data.HasMore {
			break
		}
		pageToken = result.Data.PageToken
	}

	return allRecords, nil
}

// FetchConfig returns config rows matching the given query.
func (p *Provider) FetchConfig(ctx context.Context, q configsync.Query) ([]configsync.Row, error) {
	if p.configTableID == "" {
		return nil, fmt.Errorf("config table ID not configured")
	}

	records, err := p.SearchRecords(ctx, p.configTableID,
		NewFilter("enabled", "is", []string{"true"}))
	if err != nil {
		return nil, fmt.Errorf("fetch config: %w", err)
	}

	var rows []configsync.Row
	for _, rec := range records {
		row := parseConfigRow(rec)
		if row != nil {
			// Apply query filters
			if q.Edition != "" && row.Edition != "" && row.Edition != "all" && row.Edition != q.Edition {
				continue
			}
			if q.Channel != "" && row.Channel != "" && row.Channel != q.Channel {
				continue
			}
			rows = append(rows, *row)
		}
	}

	return rows, nil
}

// FetchModelPrices returns model prices from the storage channel.
func (p *Provider) FetchModelPrices(ctx context.Context) ([]configsync.ProviderPrice, error) {
	if p.pricingTableID == "" {
		return nil, configsync.ErrNotSupported
	}

	records, err := p.SearchRecords(ctx, p.pricingTableID,
		NewFilter("enabled", "is", []string{"true"}))
	if err != nil {
		return nil, fmt.Errorf("fetch model prices: %w", err)
	}

	var prices []configsync.ProviderPrice
	for _, rec := range records {
		pp := parseProviderPrice(rec)
		if pp != nil {
			prices = append(prices, *pp)
		}
	}

	return prices, nil
}

// FetchAll returns both config rows and model prices in a single fetch.
func (p *Provider) FetchAll(ctx context.Context, q configsync.Query) ([]configsync.Row, []configsync.ProviderPrice, error) {
	config, err := p.FetchConfig(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	prices, err := p.FetchModelPrices(ctx)
	if err != nil && err != configsync.ErrNotSupported {
		return nil, nil, err
	}

	return config, prices, nil
}

// FetchPipelineTemplates returns pipeline templates from the Feishu Bitable.
func (p *Provider) FetchPipelineTemplates(ctx context.Context) ([]configsync.PipelineTemplate, error) {
	if p.pipelineTableID == "" {
		return nil, configsync.ErrNotSupported
	}

	records, err := p.SearchRecords(ctx, p.pipelineTableID,
		NewFilter("enabled", "is", []string{"true"}))
	if err != nil {
		return nil, fmt.Errorf("fetch pipeline templates: %w", err)
	}

	var templates []configsync.PipelineTemplate
	for _, rec := range records {
		t := parsePipelineTemplate(rec)
		if t != nil {
			templates = append(templates, *t)
		}
	}

	return templates, nil
}

// Filter represents a Feishu filter.
type Filter struct {
	Conditions []Condition `json:"conditions"`
	Conjunction string      `json:"conjunction"`
}

// Condition represents a filter condition.
type Condition struct {
	FieldName string   `json:"field_name"`
	Operator  string   `json:"operator"`
	Value     []string `json:"value"`
}

// NewFilter creates a filter for searching records.
func NewFilter(fieldName, operator string, values []string) *Filter {
	return &Filter{
		Conditions: []Condition{
			{
				FieldName: fieldName,
				Operator:  operator,
				Value:     values,
			},
		},
		Conjunction: "and",
	}
}

// Record represents a Feishu Bitable record.
type Record struct {
	RecordID string         `json:"record_id,omitempty"`
	Fields   map[string]any `json:"fields"`
}

// SearchRecordsRequest represents a search request.
type SearchRecordsRequest struct {
	Filter    *Filter `json:"filter,omitempty"`
	PageToken string  `json:"page_token,omitempty"`
	PageSize  int     `json:"page_size,omitempty"`
}

// SearchRecordsResponse represents a search response.
type SearchRecordsResponse struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool     `json:"has_more"`
		PageToken string   `json:"page_token"`
		Items     []Record `json:"items"`
	} `json:"data"`
}

// TextField extracts a string value from a Feishu record field.
// Handles both simple strings and the SearchRecords format: [{text: "...", type: "text"}]
func TextField(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		// Handle SearchRecords format: [{text: "...", type: "text"}]
		if len(val) > 0 {
			if item, ok := val[0].(map[string]any); ok {
				if text, ok := item["text"].(string); ok {
					return text
				}
			}
		}
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// NumberField extracts a number value from a Feishu record field.
func NumberField(v any) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var n float64
		fmt.Sscanf(val, "%f", &n)
		return n
	default:
		return 0
	}
}

// BoolField extracts a boolean value from a Feishu record field.
func BoolField(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	default:
		return false
	}
}

// parseConfigRow parses a Feishu record into a configsync.Row.
func parseConfigRow(rec Record) *configsync.Row {
	f := rec.Fields
	key := TextField(f["config_key"])
	if key == "" {
		return nil
	}

	row := &configsync.Row{
		Key:        key,
		Edition:    TextField(f["edition"]),
		Channel:    TextField(f["channel"]),
		MinVersion: TextField(f["min_version"]),
		MaxVersion: TextField(f["max_version"]),
		Priority:   int(NumberField(f["priority"])),
		Enabled:    BoolField(f["enabled"]),
		Remark:     TextField(f["remark"]),
	}

	// Parse updated_at
	if ts := NumberField(f["updated_at"]); ts > 0 {
		row.UpdatedAt = time.Unix(int64(ts), 0)
	}

	// Parse value as JSON (Feishu field name is "config_value")
	valueJSON := TextField(f["config_value"])
	if valueJSON == "" {
		valueJSON = TextField(f["value"])
	}
	if valueJSON != "" {
		var value json.RawMessage
		if err := json.Unmarshal([]byte(valueJSON), &value); err == nil {
			row.Value = value
		} else {
			// If the value is plain text (not valid JSON), wrap it as a JSON string
			quoted, _ := json.Marshal(valueJSON)
			row.Value = json.RawMessage(quoted)
		}
	}

	return row
}

// parseProviderPrice parses a Feishu record into a configsync.ProviderPrice.
func parseProviderPrice(rec Record) *configsync.ProviderPrice {
	f := rec.Fields
	baseURL := TextField(f["base_url"])
	if baseURL == "" {
		return nil
	}

	pp := &configsync.ProviderPrice{
		BaseURL:      baseURL,
		ProviderName: TextField(f["provider_name"]),
		Currency:     TextField(f["currency"]),
		Enabled:      BoolField(f["enabled"]),
	}

	// Parse updated_at
	if ts := NumberField(f["updated_at"]); ts > 0 {
		pp.UpdatedAt = time.Unix(int64(ts), 0)
	}

	// Parse models JSON array
	modelsJSON := TextField(f["models"])
	if modelsJSON != "" {
		var models []configsync.ModelPrice
		if err := json.Unmarshal([]byte(modelsJSON), &models); err == nil {
			pp.Models = models
		}
	}

	return pp
}

// parsePipelineTemplate parses a Feishu record into a configsync.PipelineTemplate.
func parsePipelineTemplate(rec Record) *configsync.PipelineTemplate {
	f := rec.Fields
	id := TextField(f["id"])
	if id == "" {
		return nil
	}

	t := &configsync.PipelineTemplate{
		ID:          id,
		Name:        TextField(f["name"]),
		Description: TextField(f["description"]),
		Version:     TextField(f["version"]),
		Edition:     TextField(f["type"]), // "type" field in Feishu maps to edition
	}

	// Parse config JSON
	configJSON := TextField(f["config"])
	if configJSON != "" {
		var configData map[string]any
		if err := json.Unmarshal([]byte(configJSON), &configData); err == nil {
			// Extract nodes from config if present
			if nodes, ok := configData["steps"].([]any); ok {
				for i, node := range nodes {
					if nodeMap, ok := node.(map[string]any); ok {
						t.Nodes = append(t.Nodes, configsync.PipelineNodeConfig{
							ID:   fmt.Sprintf("node-%d", i),
							Name: fmt.Sprintf("%v", nodeMap["name"]),
							Type: fmt.Sprintf("%v", nodeMap["tool"]),
						})
					}
				}
			}
		}
	}

	return t
}

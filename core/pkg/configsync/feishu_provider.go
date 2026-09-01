package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"centag/core/pkg/feishu"
	"centag/core/pkg/logger"
)

// FeishuProvider implements Provider backed by Feishu Bitable.
// It reads the centag_config, centag_model_price, and centag_pipeline_templates tables.
type FeishuProvider struct {
	client           *feishu.Client
	appToken         string
	configTableID    string
	priceTableID     string
	pipelineTableID  string
}

// NewFeishuProvider creates a Feishu provider from environment variables.
// Credential fallback: CENTAG_CONFIGSYNC_FEISHU_* → CENTAG_TELEMETRY_FEISHU_*.
// Only APP_ID/SECRET fall back (same Feishu app); Bitable identifiers (APP_TOKEN,
// TABLE_ID, PRICE_TABLE_ID, PIPELINE_TABLE_ID) are specific to the configsync base and must be set
// explicitly via CENTAG_CONFIGSYNC_FEISHU_*.
func NewFeishuProvider() (*FeishuProvider, error) {
	appID := envOrFeishu("CENTAG_CONFIGSYNC_FEISHU_APP_ID", os.Getenv("CENTAG_TELEMETRY_FEISHU_APP_ID"))
	appSecret := envOrFeishu("CENTAG_CONFIGSYNC_FEISHU_APP_SECRET", os.Getenv("CENTAG_TELEMETRY_FEISHU_APP_SECRET"))
	appToken := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_APP_TOKEN")
	configTableID := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_TABLE_ID")
	priceTableID := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_PRICE_TABLE_ID")
	pipelineTableID := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_PIPELINE_TABLE_ID")

	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("configsync feishu: missing APP_ID/APP_SECRET")
	}
	if appToken == "" {
		return nil, fmt.Errorf("configsync feishu: missing APP_TOKEN")
	}
	if configTableID == "" {
		return nil, fmt.Errorf("configsync feishu: missing TABLE_ID (config table)")
	}

	client := feishu.NewClient(feishu.Config{
		AppID:     appID,
		AppSecret: appSecret,
	})
	return &FeishuProvider{
		client:          client,
		appToken:        appToken,
		configTableID:   configTableID,
		priceTableID:    priceTableID,
		pipelineTableID: pipelineTableID,
	}, nil
}

// MustNewFeishuProvider creates a Feishu provider or returns nil if not configured.
func MustNewFeishuProvider() *FeishuProvider {
	p, err := NewFeishuProvider()
	if err != nil {
		logger.Infof("configsync feishu not configured: %v", err)
		return nil
	}
	return p
}

// IsFeishuConfigured returns true if the provider has valid credentials.
func IsFeishuConfigured() bool {
	appID := envOrFeishu("CENTAG_CONFIGSYNC_FEISHU_APP_ID", os.Getenv("CENTAG_TELEMETRY_FEISHU_APP_ID"))
	appSecret := envOrFeishu("CENTAG_CONFIGSYNC_FEISHU_APP_SECRET", os.Getenv("CENTAG_TELEMETRY_FEISHU_APP_SECRET"))
	appToken := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_APP_TOKEN")
	tableID := os.Getenv("CENTAG_CONFIGSYNC_FEISHU_TABLE_ID")
	return appID != "" && appSecret != "" && appToken != "" && tableID != ""
}

func (p *FeishuProvider) FetchConfig(ctx context.Context, q Query) ([]Row, error) {
	records, err := p.client.SearchRecords(ctx, p.appToken, p.configTableID, feishu.NewFilter("enabled", "is", "true"))
	if err != nil {
		return nil, fmt.Errorf("configsync feishu: fetch config: %w", err)
	}
	var rows []Row
	for _, rec := range records {
		row := feishuRecordToConfigRow(rec)
		if row == nil {
			continue
		}
		// Apply version/edition filtering
		if !MatchVersion(row, q) {
			continue
		}
		rows = append(rows, *row)
	}
	return rows, nil
}

func (p *FeishuProvider) FetchModelPrices(ctx context.Context) ([]ProviderPrice, error) {
	if p.priceTableID == "" {
		return nil, ErrNotSupported
	}
	records, err := p.client.SearchRecords(ctx, p.appToken, p.priceTableID, feishu.NewFilter("enabled", "is", "true"))
	if err != nil {
		return nil, fmt.Errorf("configsync feishu: fetch prices: %w", err)
	}
	var prices []ProviderPrice
	for _, rec := range records {
		pp := feishuRecordToProviderPrice(rec)
		if pp != nil {
			prices = append(prices, *pp)
		}
	}
	return prices, nil
}

func (p *FeishuProvider) FetchPipelineTemplates(ctx context.Context) ([]PipelineTemplate, error) {
	if p.pipelineTableID == "" {
		return nil, ErrNotSupported
	}
	records, err := p.client.SearchRecords(ctx, p.appToken, p.pipelineTableID, feishu.NewFilter("enabled", "is", "true"))
	if err != nil {
		return nil, fmt.Errorf("configsync feishu: fetch pipeline templates: %w", err)
	}
	var templates []PipelineTemplate
	for _, rec := range records {
		tmpl := feishuRecordToPipelineTemplate(rec)
		if tmpl != nil {
			templates = append(templates, *tmpl)
		}
	}
	return templates, nil
}

func (p *FeishuProvider) FetchAll(ctx context.Context, q Query) ([]Row, []ProviderPrice, error) {
	rows, err := p.FetchConfig(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	prices, err := p.FetchModelPrices(ctx)
	if err != nil && err != ErrNotSupported {
		return nil, nil, err
	}
	return rows, prices, nil
}

func feishuRecordToConfigRow(rec feishu.Record) *Row {
	f := rec.Fields
	key := feishu.TextField(f["config_key"])
	if key == "" {
		return nil
	}
	var priority int
	if p, ok := f["priority"].(float64); ok {
		priority = int(p)
	}
	var value json.RawMessage
	if v := feishu.TextField(f["value"]); v != "" {
		value = json.RawMessage(v)
	}
	enabled, _ := f["enabled"].(bool)
	var updatedAt time.Time
	switch t := f["updated_at"].(type) {
	case string:
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			updatedAt = ts
		}
	case float64:
		updatedAt = time.UnixMilli(int64(t))
	}
	return &Row{
		Edition:    feishu.TextField(f["edition"]),
		Key:        key,
		Channel:    feishu.TextField(f["channel"]),
		MinVersion: feishu.TextField(f["min_version"]),
		MaxVersion: feishu.TextField(f["max_version"]),
		Priority:   priority,
		Value:      value,
		Enabled:    enabled,
		UpdatedAt:  updatedAt,
		Remark:     feishu.TextField(f["remark"]),
	}
}

func feishuRecordToProviderPrice(rec feishu.Record) *ProviderPrice {
	f := rec.Fields
	baseURL := feishu.TextField(f["base_url"])
	if baseURL == "" {
		return nil
	}
	var models []ModelPrice
	if modelsJSON := feishu.TextField(f["models"]); modelsJSON != "" {
		if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
			logger.Warnf("configsync: parse models for %s: %v", baseURL, err)
			return nil
		}
	}
	enabled, _ := f["enabled"].(bool)
	return &ProviderPrice{
		BaseURL:      baseURL,
		ProviderName: feishu.TextField(f["provider_name"]),
		Currency:     feishu.TextField(f["currency"]),
		Models:       models,
		Enabled:      enabled,
	}
}

func feishuRecordToPipelineTemplate(rec feishu.Record) *PipelineTemplate {
	f := rec.Fields
	pipelineID := feishu.TextField(f["pipeline_id"])
	if pipelineID == "" {
		return nil
	}
	enabled, _ := f["enabled"].(bool)
	if !enabled {
		return nil
	}
	// 从 content_json 字段解析模板内容
	var tmpl PipelineTemplate
	if contentJSON := feishu.TextField(f["content_json"]); contentJSON != "" {
		if err := json.Unmarshal([]byte(contentJSON), &tmpl); err != nil {
			logger.Warnf("configsync: parse pipeline template %s: %v", pipelineID, err)
			return nil
		}
	}
	// 确保 ID 一致
	tmpl.ID = pipelineID
	return &tmpl
}

// Client returns the underlying feishu.Client for management endpoints.
func (p *FeishuProvider) Client() *feishu.Client { return p.client }

// AppToken returns the bitable app token.
func (p *FeishuProvider) AppToken() string { return p.appToken }

// ConfigTableID returns the config table ID.
func (p *FeishuProvider) ConfigTableID() string { return p.configTableID }

// PriceTableID returns the price table ID (may be empty).
func (p *FeishuProvider) PriceTableID() string { return p.priceTableID }

// PipelineTableID returns the pipeline template table ID (may be empty).
func (p *FeishuProvider) PipelineTableID() string { return p.pipelineTableID }

func envOrFeishu(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// NormalizeBaseURLFeishu strips trailing slashes for consistent matching.
func NormalizeBaseURLFeishu(url string) string {
	return strings.TrimRight(url, "/")
}

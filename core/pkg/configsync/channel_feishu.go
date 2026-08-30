package configsync

import (
	"context"
	"os"
	"strings"

	"centag/core/pkg/feishu"
)

// Built-in channel IDs.
const (
	ChannelFeishu   = "feishu"
	ChannelSnapshot = "snapshot"
)

// FeishuTokenProbe validates feishu credentials by acquiring a tenant token
// and listing the base's tables. Assigned to the descriptor at registration
// so tests can substitute it via the descriptor.
func FeishuTokenProbe(ctx context.Context, values map[string]string) error {
	c := feishu.NewClient(feishu.Config{
		AppID:     values["APP_ID"],
		AppSecret: values["APP_SECRET"],
	})
	if _, err := c.ListTables(ctx, values["APP_TOKEN"]); err != nil {
		return err
	}
	return nil
}

// FeishuWriterProbe additionally checks that the credentials allow editing
// (a read-only app fails on create with 403 in real deployments).
var FeishuWriterProbe = FeishuTokenProbe

func init() {
	RegisterChannel(ChannelDescriptor{
		ID:          ChannelFeishu,
		Description: "飞书多维表格（管理面 + 分发面）",
		Fields: []ChannelField{
			{Name: "APP_ID", Prompt: "Feishu App ID", Required: true},
			{Name: "APP_SECRET", Prompt: "Feishu App Secret", Secret: true, Required: true},
			{Name: "APP_TOKEN", Prompt: "Bitable App Token (centag_config 所在 Base)", Required: true},
			{Name: "TABLE_ID", Prompt: "centag_config 表 ID（留空自动按表名解析）"},
			{Name: "PRICE_TABLE_ID", Prompt: "centag_model_price 表 ID（留空自动按表名解析）"},
		},
		Validate: FeishuTokenProbe,
	})
	RegisterChannel(ChannelDescriptor{
		ID:          ChannelSnapshot,
		Description: "公开静态快照（无凭证，Personal 分发面）",
		Fields: []ChannelField{
			{Name: "SNAPSHOT_URL", Prompt: "快照 JSON URL（空格分隔多源）"},
		},
		Validate: func(ctx context.Context, values map[string]string) error {
			return nil // snapshot has no live credential to probe
		},
	})
}

// FeishuProviderConfig bundles resolved feishu channel values.
type FeishuProviderConfig struct {
	AppID        string
	AppSecret    string
	AppToken     string
	TableID      string
	PriceTableID string
}

// LoadFeishuProviderConfig resolves feishu channel values with the standard
// four-level priority and auto-resolves table IDs by name when unset.
// Device side passes Wizard=nil (TC-CFG-009: missing config is a hard error).
func LoadFeishuProviderConfig(ctx context.Context, opts LoadOptions, client *feishu.Client) (*FeishuProviderConfig, map[string]string, error) {
	values, sources, err := LoadChannelConfig(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	cfg := &FeishuProviderConfig{
		AppID:        values["APP_ID"],
		AppSecret:    values["APP_SECRET"],
		AppToken:     values["APP_TOKEN"],
		TableID:      values["TABLE_ID"],
		PriceTableID: values["PRICE_TABLE_ID"],
	}
	if client != nil {
		if cfg.TableID == "" {
			if id, err := ResolveTableID(ctx, client, cfg.AppToken, "", "centag_config"); err == nil {
				cfg.TableID = id
			}
		}
		if cfg.PriceTableID == "" {
			if id, err := ResolveTableID(ctx, client, cfg.AppToken, "", "centag_model_price"); err == nil {
				cfg.PriceTableID = id
			}
		}
	}
	return cfg, sources, nil
}

// SnapshotURLsFromEnv resolves snapshot channel URLs (env override wins).
func SnapshotURLsFromEnv(defaults []string) []string {
	return ResolveSnapshotURLs(defaults)
}

// envOr returns the first non-empty env value among names.
func envOr(names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(os.Getenv(n)); v != "" {
			return v
		}
	}
	return ""
}

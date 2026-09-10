package config

import (
	"encoding/json"
	"testing"
)

// mcp-interface-layer A-T4（TC-CFG-001）：MCP 配置安全默认与旧配置兼容。
// 旧配置（无 mcp 节）加载后必须按 disabled 默认、无 panic。
func TestMCPConfig(t *testing.T) {
	t.Run("TC-CFG-001: default disabled", func(t *testing.T) {
		def := DefaultMcpConfig()
		if def.Enabled {
			t.Fatalf("默认必须是 disabled")
		}
		if len(def.AllowedTools) != 0 {
			t.Fatalf("默认 allowed_tools 必须为空（= 全部只读工具），got %v", def.AllowedTools)
		}
	})

	t.Run("TC-CFG-001: old config json without mcp section loads disabled", func(t *testing.T) {
		// 模拟旧的全量配置 JSON（不含 mcp 节）
		old, err := json.Marshal(Config{Server: ServerConfig{Port: 20060}})
		if err != nil {
			t.Fatal(err)
		}
		var cfg Config
		if err := json.Unmarshal(old, &cfg); err != nil {
			t.Fatalf("旧配置解析失败: %v", err)
		}
		if cfg.Mcp.Enabled {
			t.Fatalf("无 mcp 节必须是 disabled")
		}
		if cfg.Server.Port != 20060 {
			t.Fatalf("旧配置其它字段不应受影响, port=%d", cfg.Server.Port)
		}
	})

	t.Run("explicit enable roundtrip", func(t *testing.T) {
		cfg := Config{Mcp: McpConfig{Enabled: true, AllowedTools: []string{"read_log"}}}
		b, err := json.Marshal(cfg.Mcp)
		if err != nil {
			t.Fatal(err)
		}
		var got McpConfig
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if !got.Enabled || len(got.AllowedTools) != 1 || got.AllowedTools[0] != "read_log" {
			t.Fatalf("roundtrip 不一致: %+v", got)
		}
	})
}

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mcp-interface-layer A-T1（TC-TOOL-PARA-001）：read_config 行为快照。
// MCP 工具面（pkg/server/mcp）将复用该工具作为单一真源；抽取/重定向前后同一输入必须
// 产生完全一致的输出，本用例锁定当前行为作为回归基线。
func TestReadConfigTool_SnapshotParity(t *testing.T) {
	dataDir := t.TempDir()
	configDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgBody := `{"backends":["a","b"],"port":20060}`
	if err := os.WriteFile(filepath.Join(configDir, "proxy.json"), []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "extra.toml"), []byte("port = 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	pretty := "{\n  \"backends\": [\n    \"a\",\n    \"b\"\n  ],\n  \"port\": 20060\n}"
	missingErrPrefix := "读取配置文件失败: "
	candidatesPrefix := "未指定配置文件路径"

	tool := NewReadConfigTool(dataDir)

	tests := []struct {
		name      string
		params    map[string]any
		wantError bool
		wantEq    string // Content 完全相等（wantPrefix 为空时生效）
		wantPref  string // Content 前缀匹配
	}{
		{"json file deterministic", map[string]any{"path": "config/proxy.json"}, false, pretty, ""},
		{"non-json passthrough", map[string]any{"path": "config/extra.toml"}, false, "port = 1", ""},
		{"missing file stable error", map[string]any{"path": "config/absent.json"}, true, "", missingErrPrefix},
		{"path escape rejected", map[string]any{"path": "../../etc/passwd"}, true, "", missingErrPrefix},
		{"empty path lists candidates", map[string]any{}, false, "", candidatesPrefix},
	}

	for round := 0; round < 2; round++ {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res1, err := tool.Execute(context.Background(), tt.params)
				if err != nil {
					t.Fatalf("Execute 契约应内嵌错误，不应返错: %v", err)
				}
				if res1.IsError != tt.wantError {
					t.Fatalf("IsError = %v, want %v (content=%q)", res1.IsError, tt.wantError, res1.Content)
				}
				if tt.wantEq != "" && res1.Content != tt.wantEq {
					t.Fatalf("Content 与快照不符: got %q want %q", res1.Content, tt.wantEq)
				}
				if tt.wantPref != "" && !strings.HasPrefix(res1.Content, tt.wantPref) {
					t.Fatalf("Content 前缀与快照不符: %q want prefix %q", res1.Content, tt.wantPref)
				}
				// 同一输入二次执行必须完全一致
				res2, _ := tool.Execute(context.Background(), tt.params)
				if res1.IsError != res2.IsError || res1.Content != res2.Content {
					t.Fatalf("read_config 输出不稳定: first=(isError=%v,%q) second=(isError=%v,%q)",
						res1.IsError, res1.Content, res2.IsError, res2.Content)
				}
			})
		}
	}
}

// mcp-interface-layer A-T1（TC-TOOL-PARA-002）：read_database 表白名单/注入拒绝行为快照。
// 校验复用 validateReadOnlyQuery（与 Execute 内部完全一致），不依赖真实驱动以保持确定性。
func TestReadDatabaseTool_SnapshotRejection(t *testing.T) {
	allowed := []string{"agent_sessions", "system_config"}

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"TC-TOOL-PARA-002: whitelist select allowed", "SELECT * FROM agent_sessions LIMIT 5", false},
		{"TC-TOOL-PARA-002: non-whitelist table rejected", "SELECT * FROM users", true},
		{"TC-TOOL-PARA-002: delete injection rejected", "SELECT 1; DELETE FROM agent_sessions", true},
		{"TC-TOOL-PARA-002: drop injection rejected", "DROP TABLE agent_sessions", true},
		{"TC-TOOL-PARA-002: cte write rejected", "WITH d AS (DELETE FROM users RETURNING *) SELECT * FROM d", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err1 := validateReadOnlyQuery(tt.query, allowed)
			err2 := validateReadOnlyQuery(tt.query, allowed)
			if (err1 == nil) != (err2 == nil) {
				t.Fatalf("拒绝行为不稳定: first=%v second=%v", err1, err2)
			}
			if (err1 != nil) != tt.wantErr {
				t.Fatalf("validateReadOnlyQuery(%q) err = %v, wantErr %v", tt.query, err1, tt.wantErr)
			}
		})
	}
}

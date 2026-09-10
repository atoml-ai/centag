package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"

	"centag/core/internal/tokenusage"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupVolumeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE token_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		backend_id TEXT NOT NULL,
		success INTEGER,
		created_at DATETIME
	)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

// TestTokenUsageVolumeProvider_TC-MCP-MET-001 happy path：SQL 聚合由 tokenusage.Service 完成，
// Provider 透传为 MetricsVolume（无裸 SQL 语义出现在 MCP 面）。
func TestTokenUsageVolumeProvider_HappyPath(t *testing.T) {
	db := setupVolumeDB(t)
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO token_usage (backend_id, success, created_at) VALUES
		('b1', 1, datetime('now','-1 hour')),
		('b1', 0, datetime('now')),
		('b2', 1, datetime('now')),
		('b2', 1, datetime('now','-48 hours'))`); err != nil {
		t.Fatalf("seed sqlite: %v", err)
	}
	p := NewTokenUsageVolumeProvider(tokenusage.NewService(db, "sqlite"))
	vol, err := p.RequestVolume(context.Background(), 24)
	if err != nil {
		t.Fatal(err)
	}
	if vol.Hours != 24 {
		t.Fatalf("expected hours 24, got %d", vol.Hours)
	}
	byID := map[string]BackendVolume{}
	for _, b := range vol.Backends {
		byID[b.BackendID] = b
	}
	if b1 := byID["b1"]; b1.Requests != 2 || b1.Failed != 1 {
		t.Fatalf("b1 unexpected: %+v", b1)
	}
	if b2 := byID["b2"]; b2.Requests != 1 || b2.Failed != 0 {
		t.Fatalf("b2 unexpected: %+v", b2)
	}
}

// TC-MCP-MET-002 护栏前置：provider 层拒绝超 720h / <1，且不触发任何查询。
func TestTokenUsageVolumeProvider_Guards(t *testing.T) {
	p := NewTokenUsageVolumeProvider(tokenusage.NewService(nil, "sqlite"))
	for _, h := range []int{0, -1, MaxMetricsHours + 1} {
		if _, err := p.RequestVolume(context.Background(), h); err == nil {
			t.Fatalf("hours=%d expected rejection", h)
		}
	}
}

// TC-MCP-MET 未配置数据源：svc 为 nil 时明确报错（不 panic）。
func TestTokenUsageVolumeProvider_NoStore(t *testing.T) {
	p := NewTokenUsageVolumeProvider(nil)
	if _, err := p.RequestVolume(context.Background(), 24); err == nil {
		t.Fatal("expected error when store not configured")
	}
}

// 反射不可用时的连接桥梁：直接构造 tokenusage.Service 取数方法（生产同路径）。
func TestReadMetricsTool_Metadata(t *testing.T) {
	tool := newReadMetricsTool(&mockProvider{})
	if tool.Name() != "read_metrics" {
		t.Fatalf("name: %q", tool.Name())
	}
	if !tool.IsReadOnly() {
		t.Fatal("read_metrics must be read-only")
	}
	schema := tool.ParamSchema()
	if schema["type"] != "object" {
		t.Fatalf("schema: %v", schema)
	}
	if tool.Description() == "" {
		t.Fatal("description empty")
	}
}

// numericAsIntHours 输入兼容性（json.Number / int / 非法类型）。
func TestNumericAsIntHours(t *testing.T) {
	for _, tc := range []struct {
		in  any
		out int
		ok  bool
	}{{json.Number("24"), 24, true}, {json.Number("x"), 0, false}, {int(8), 8, true}, {"abc", 0, false}} {
		got, err := numericAsIntHours(tc.in)
		if tc.ok != (err == nil) || (tc.ok && got != tc.out) {
			t.Fatalf("input %v: got %d,%v want %d,ok=%v", tc.in, got, err, tc.out, tc.ok)
		}
	}
}

// version 缺省回退（未注入版本号 → dev）。
func TestObservationServer_VersionFallback(t *testing.T) {
	s := &ObservationServer{}
	if s.version() != "dev" {
		t.Fatalf("expected dev, got %q", s.version())
	}
}

// fakeTool 驱动 toSDKTool / toSDKHandler 全分支（含 Details 序列化与执行错误）。
type fakeTool struct{ detailMode bool }

func (f fakeTool) Name() string                { return "fake" }
func (f fakeTool) Description() string         { return "d" }
func (f fakeTool) ParamSchema() map[string]any { return nil } // nil → 触发 toSDKTool 兜底 schema
func (f fakeTool) IsReadOnly() bool            { return true }
func (f fakeTool) Execute(_ context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	if _, bad := params["invalid_args"]; bad {
		return nil, nil // 上面 handler 已在 Unmarshal 阶段返回错误，不会到达
	}
	if f.detailMode {
		return &agentcore.ToolResult{Details: map[string]any{"k": "v"}}, nil
	}
	return &agentcore.ToolResult{Content: "ok", IsError: true}, nil
}

func TestToSDKTool_Adapters(t *testing.T) {
	sd := toSDKTool(fakeTool{})
	if sd.Name != "fake" {
		t.Fatalf("tool: %v schema=%s", sd.Name, sd.InputSchema)
	}
	if _, ok := sd.InputSchema.(json.RawMessage); !ok {
		t.Fatalf("schema type: %s", sd.InputSchema)
	}
}

func TestToSDKHandler_Adapters(t *testing.T) {
	ctx := context.Background()

	// 正常路径：Content 回显 + IsError 透传。
	res, err := toSDKHandler(fakeTool{})(ctx, &gomcp.CallToolRequest{Params: &gomcp.CallToolParamsRaw{Arguments: []byte(`{"a":1}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected IsError passthrough")
	}

	// Details 分支：Content 为空时序列化 Details。
	res2, err := toSDKHandler(fakeTool{detailMode: true})(ctx, &gomcp.CallToolRequest{Params: &gomcp.CallToolParamsRaw{}})
	if err != nil {
		t.Fatal(err)
	}
	if res2.IsError || len(res2.Content) != 1 {
		t.Fatalf("unexpected: %+v", res2)
	}

	// 非法参数 JSON → 协议层报错。
	if _, err := toSDKHandler(fakeTool{})(ctx, &gomcp.CallToolRequest{Params: &gomcp.CallToolParamsRaw{Arguments: []byte(`{`)}}); err == nil {
		t.Fatal("expected invalid-arguments error")
	}
}

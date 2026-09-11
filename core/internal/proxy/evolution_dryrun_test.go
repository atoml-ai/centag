package proxy

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"centag/core/internal/agent/evolution"
	"centag/core/internal/auth"
	"centag/core/internal/tokenusage"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func newUsageContext(t *testing.T, dryrun bool) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat", nil)
	if dryrun {
		c.Request.Header.Set(evolution.HeaderEvolutionDryrun, "true")
	}
	c.Request.Header.Set("X-Session-ID", "sess-42")
	c.Request.Header.Set("X-Request-ID", "req-42")
	c.Set(auth.CtxKeyUserID, int64(7))
	return c
}

func recordingOutput() *pipeline.PipelineOutput {
	return &pipeline.PipelineOutput{
		Content: "ok",
		Metadata: map[string]any{
			"model":   "gpt-4o",
			"backend": "azure",
		},
	}
}

// setupUsageAggregationDB 内存 token_usage 表（常规计费聚合 mock）。
func setupUsageAggregationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	schema := `
	CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL, group_id TEXT);
	INSERT INTO users (id, username) VALUES (1, 'admin');
	CREATE TABLE token_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		api_key_id INTEGER,
		backend_id TEXT NOT NULL,
		model TEXT NOT NULL,
		prompt_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		request_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		tenant_id TEXT,
		cost_usd REAL DEFAULT 0,
		input_cost REAL DEFAULT 0,
		output_cost REAL DEFAULT 0,
		cost_input_price REAL DEFAULT 0,
		cost_output_price REAL DEFAULT 0,
		revenue_usd REAL DEFAULT 0,
		revenue_input_price REAL DEFAULT 0,
		revenue_output_price REAL DEFAULT 0,
		pricing_rule_id INTEGER,
		success INTEGER NOT NULL DEFAULT 1,
		dept_tag TEXT,
		agent_type TEXT,
		group_id TEXT,
		source TEXT NOT NULL DEFAULT 'real',
		session_id TEXT
	);
	CREATE TABLE token_usage_daily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		backend_id TEXT NOT NULL,
		model TEXT NOT NULL,
		agent_type TEXT,
		date DATE NOT NULL,
		total_prompt_tokens INTEGER DEFAULT 0,
		total_completion_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		total_cost_usd REAL DEFAULT 0,
		cost_input_price REAL DEFAULT 0,
		cost_output_price REAL DEFAULT 0,
		total_revenue_usd REAL DEFAULT 0,
		request_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		group_id TEXT,
		UNIQUE(user_id, backend_id, model, agent_type, date)
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

// TC-BILL-EVO-001 / TC-BILL-EVO-002：dryrun 标记请求 → evol-tag 分桶、常规聚合零变化。
func TestDryrunHeaderBucketsOnly(t *testing.T) {
	dir := t.TempDir()
	evolution.SetDataDirForce(dir)
	defer evolution.SetDataDirForce("")

	// 常规计费聚合 mock：内存 token_usage 服务。
	db := setupUsageAggregationDB(t)
	svc := tokenusage.NewService(db, "sqlite")
	tokenusage.SetDefaultService(svc)
	defer tokenusage.SetDefaultService(nil)

	// dryrun 标记请求：服务成功（只走分桶，不写常规聚合）。
	c := newUsageContext(t, true)
	maybeRecordTokenUsage(c, recordingOutput(), "gpt-4o")
	// hooked 记录落盘：分桶文件有条目。
	b, err := os.ReadFile(filepath.Join(dir, "var", "agent", "evolution", "dryrun-usage.jsonl"))
	if err != nil {
		t.Fatalf("分桶文件缺失: %v", err)
	}
	s := bufio.NewScanner(strings.NewReader(strings.TrimRight(string(b), "\n")))
	if s.Scan() == false {
		t.Fatal("分桶文件空")
	}
	var rec map[string]any
	if err := json.Unmarshal(s.Bytes(), &rec); err != nil {
		t.Fatalf("分桶行解析: %v", err)
	}
	if rec["bucket"] != "evol-dryrun" {
		t.Fatalf("bucket = %v", rec["bucket"])
	}
	if int(rec["total_tokens"].(float64)) <= 0 {
		t.Fatalf("分桶行 token 缺失: %v", rec)
	}
	// 常规聚合零变化：GetUserUsage 计数 = 0。
	time.Sleep(100 * time.Millisecond) // RecordUsage 异步落库窗口
	var dbCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM token_usage`).Scan(&dbCount)
	if dbCount != 0 {
		t.Fatalf("常规表被 dryrun 写入 %d 行", dbCount)
	}
	wide := func() (time.Time, time.Time) {
		return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	f, to := wide()
	stats, err := svc.GetUserUsage(context.Background(), 7, f, to)
	if err != nil {
		t.Fatalf("聚合查询: %v", err)
	}
	if stats.RequestCount != 0 || stats.TotalTokens != 0 {
		t.Fatalf("常规聚合被 dryrun 污染: %+v", stats)
	}

	// 对照组：不带标记的请求走常规记录。
	c2 := newUsageContext(t, false)
	maybeRecordTokenUsage(c2, recordingOutput(), "gpt-4o")
	time.Sleep(100 * time.Millisecond)
	_ = db.QueryRow(`SELECT COUNT(*) FROM token_usage`).Scan(&dbCount)
	if dbCount != 1 {
		t.Fatalf("常规表应有 1 行，实际 %d", dbCount)
	}
	f, to = wide()
	stats2, err := svc.GetUserUsage(context.Background(), 7, f, to)
	if err != nil {
		t.Fatalf("聚合查询2: %v", err)
	}
	if stats2.RequestCount != 1 {
		t.Fatalf("常规请求计数 = %d，期望 1", stats2.RequestCount)
	}
}

// TC-BILL-EVO-001 附属：标记形式归一化（1/yes/true 均算标记）。
func TestIsDryrunHeaderVariants(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "YES", "True"} {
		if !evolution.IsDryrunHeader(v) {
			t.Errorf("%q 应被识别为 dryrun", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "on"} {
		if evolution.IsDryrunHeader(v) {
			t.Errorf("%q 不应被视为 dryrun", v)
		}
	}
}

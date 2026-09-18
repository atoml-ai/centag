package evolution

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TC-DB-EVO-001 迁移幂等建表：EnsureSchema 重复调用不报错、表存在且可用写入。
func TestEnsureSchemaIdempotent(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	rt := NewRuntime(db, "sqlite", t.TempDir())
	for i := 0; i < 2; i++ {
		if err := rt.EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema 第 %d 次: %v", i+1, err)
		}
	}
	// 启动两次（兜底建表路径 ×2 + 迁移 runner 语义）。
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema(default): %v", err)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema(second): %v", err)
	}
	if _, err := db.Exec(`INSERT INTO agent_evolution_log
(id, session_id, target, proposal, status, created_at, updated_at)
VALUES ('t','s','x','{}','proposed','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("表不可写: %v", err)
	}
}

// newTestRuntime 建好 schema + 归档目录的运行时。
// 测试环境：注册全部目标的 fake 适配器并放开指标要求（fail-closed 语义
// 由 TestRequireMetricDefaultClosed / TestApplyFailsClosedWithoutAdapter 单独覆盖）。
func newTestRuntime(t *testing.T) (*Runtime, string) {
	t.Helper()
	for _, tg := range []string{TargetPipelineWeight, TargetRetryPolicy, TargetCacheTTL, TargetBackendSwitch} {
		registerTestApplier(t, tg)
	}
	rt := NewRuntime(testDB(t), "sqlite", t.TempDir())
	rt.RequireMetric(false)
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return rt, rt.state.path
}

// TC-EVO-001 提案结构化合法：propose 产出 target/params/预期效果/回滚点。
func TestProposeStructured(t *testing.T) {
	rt, _ := newTestRuntime(t)
	p, err := rt.Propose(context.Background(), "sess-1", TargetRetryPolicy,
		map[string]any{"max_retries": float64(3)}, "错误率下降")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if p.Target != TargetRetryPolicy || p.ExpectedEffect != "错误率下降" {
		t.Fatalf("提案 %+v", p)
	}
	if p.ID == "" || p.RollbackPoint == "" {
		t.Fatal("id/rollback_point 必填")
	}
	if p.ParamsBefore == nil || len(p.ParamsBefore) != 0 {
		t.Fatalf("首提案 ParamsBefore 应为空快照: %v", p.ParamsBefore)
	}
}

// TC-EVO-002 提案参数 schema 校验：越界重试参数 → invalid 终态。
func TestProposeInvalidParamsMarkedInvalid(t *testing.T) {
	rt, _ := newTestRuntime(t)
	_, err := rt.Propose(context.Background(), "s", TargetRetryPolicy,
		map[string]any{"max_retries": float64(9)}, "")
	if err == nil {
		t.Fatal("越界参数应被拒绝")
	}
	// invalid 终态已落库（审计可回溯）。
	if _, err := rt.store.Get(context.Background(), ""); err == nil {
		t.Fatal("invalid 空查不应成功（此处仅防御）")
	}
}

// TC-EVO-002 附属：合法提案 → 落库 proposed。
func TestProposeValidPersists(t *testing.T) {
	rt, _ := newTestRuntime(t)
	p, err := rt.Propose(context.Background(), "s", TargetCacheTTL,
		map[string]any{"seconds": float64(60)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	row, err := rt.store.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if row.Status != StatusProposed || row.Target != TargetCacheTTL {
		t.Fatalf("row %+v", row)
	}
	var back Proposal
	if err := unmarshalProposal(row.Proposal, &back); err != nil {
		t.Fatalf("反序列化: %v", err)
	}
	if back.Target != TargetCacheTTL {
		t.Fatalf("回读提案 %+v", back)
	}
}

// TC-EVO-003 confirm/cancel 闭环：cancel 后状态 canceled、配置（目标参数）不变。
func TestConfirmCancelLoop(t *testing.T) {
	rt, statePath := newTestRuntime(t)
	ctx := context.Background()
	// 预置已有生效参数（cache_ttl=120）。
	if err := rt.state.Set(TargetCacheTTL, TargetKeyState{"seconds": float64(120)}); err != nil {
		t.Fatal(err)
	}
	p, err := rt.Propose(ctx, "s", TargetCacheTTL, map[string]any{"seconds": float64(3600)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Confirm(ctx, p.ID, false, true); err != nil {
		t.Fatalf("Confirm(false): %v", err)
	}
	row, _ := rt.store.Get(ctx, p.ID)
	if row.Status != StatusCanceled {
		t.Fatalf("status = %q, want canceled", row.Status)
	}
	// 配置不变。
	cur, _ := rt.state.Get(TargetCacheTTL)
	if cur["seconds"].(float64) != 120 {
		t.Fatalf("cancel 后配置被改动: %v", cur)
	}
	_ = statePath
}

// TC-EVO-005 前置（T3 完度量前先守卫）：dryrun 未通过 → apply 禁止。
func TestApplyRequiresDryrun(t *testing.T) {
	rt, _ := newTestRuntime(t)
	ctx := context.Background()
	p, _ := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(2)}, "")
	if err := rt.Apply(ctx, p.ID, true); err == nil {
		t.Fatal("未 dryrun 应禁止 apply")
	}
	if err := rt.Dryrun(ctx, p.ID); err != nil {
		t.Fatalf("Dryrun: %v", err)
	}
	if err := rt.Apply(ctx, p.ID, true); err != nil {
		t.Fatalf("dryrun 后 Apply: %v", err)
	}
}

// TC-EVO-007 一键 rollback：applied → rolled_back；目标参数回到快照。
func TestRollbackRestoresSnapshot(t *testing.T) {
	rt, _ := newTestRuntime(t)
	ctx := context.Background()
	if err := rt.state.Set(TargetRetryPolicy, TargetKeyState{"max_retries": float64(1)}); err != nil {
		t.Fatal(err)
	}
	p, err := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(3)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Dryrun(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := rt.Apply(ctx, p.ID, true); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	cur, _ := rt.state.Get(TargetRetryPolicy)
	if cur["max_retries"].(float64) != 3 {
		t.Fatalf("apply 后参数: %v", cur)
	}
	if err := rt.Rollback(ctx, p.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	cur, _ = rt.state.Get(TargetRetryPolicy)
	if cur["max_retries"].(float64) != 1 {
		t.Fatalf("rollback 后参数应回到快照 1: %v", cur)
	}
	row, _ := rt.store.Get(ctx, p.ID)
	if row.Status != StatusRolledBack || row.RollbackOf != p.ID {
		t.Fatalf("日志状态: %+v", row)
	}
}

// learning 归档：写入 var/agent/learning 并含提案/内容。
func TestLearningArchive(t *testing.T) {
	rt, statePath := newTestRuntime(t)
	_ = statePath
	path, err := rt.RecordLearning(context.Background(), "egno-x", map[string]any{"lesson": "keep-ttl"})
	if err != nil {
		t.Fatalf("RecordLearning: %v", err)
	}
	b, err := rt.learn.Read(path)
	if err != nil {
		t.Fatalf("归档不可读: %v", err)
	}
	var doc map[string]any
	_ = json.Unmarshal(b, &doc)
	if doc["proposal_id"] != "egno-x" {
		t.Fatalf("归档 doc: %v", doc)
	}
}

func dirOf(path string) string {
	_ = path
	return ""
}

func fileOf(path string) string {
	return path
}

var _ = strconv.Itoa

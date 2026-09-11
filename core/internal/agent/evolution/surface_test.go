package evolution

import (
	"context"
	"testing"
)

// Surface RunOnce 闭环（确认通过路径）：apply 成功且目标参数更新。
func TestSurfaceRunOnceApproved(t *testing.T) {
	rt, _ := newTestRuntime(t)
	s := NewSurface(rt, "sess", nil)
	s.SetConfirm(func(*Proposal) (bool, error) { return true, nil })
	p, err := s.RunOnce(context.Background(), TargetRetryPolicy,
		map[string]any{"max_retries": float64(4)}, "error rate down")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	row, _ := rt.store.Get(context.Background(), p.ID)
	if row.Status != StatusApplied || row.AppliedAt == "" {
		t.Fatalf("row %+v", row)
	}
	cur, _ := rt.state.Get(TargetRetryPolicy)
	if cur["max_retries"].(float64) != 4 {
		t.Fatalf("目标参数未生效: %v", cur)
	}
}

// Surface RunOnce 闭环（否决路径）：canceled、配置不变。
func TestSurfaceRunOnceDenied(t *testing.T) {
	rt, _ := newTestRuntime(t)
	rt.state.mustSet(TargetCacheTTL, TargetKeyState{"seconds": float64(600)})
	s := NewSurface(rt, "sess", nil)
	s.SetConfirm(func(*Proposal) (bool, error) { return false, nil })
	p, err := s.RunOnce(context.Background(), TargetCacheTTL,
		map[string]any{"seconds": float64(90)}, "")
	if err != nil {
		t.Fatalf("否决不应报错: %v", err)
	}
	row, _ := rt.store.Get(context.Background(), p.ID)
	if row.Status != StatusCanceled {
		t.Fatalf("status = %q", row.Status)
	}
	cur, _ := rt.state.Get(TargetCacheTTL)
	if cur["seconds"].(float64) != 600 {
		t.Fatalf("否决后配置被改: %v", cur)
	}
}

// confirm 未注入 → 闭环拒绝（必经 confirm 守卫）。
func TestSurfaceRequiresConfirm(t *testing.T) {
	rt, _ := newTestRuntime(t)
	s := NewSurface(rt, "sess", nil)
	if _, err := s.RunOnce(context.Background(), TargetRetryPolicy,
		map[string]any{"max_retries": float64(2)}, ""); err == nil {
		t.Fatal("confirm 未注入应报错")
	}
}

// 全部目标校验器的边界。
func TestValidateTargets(t *testing.T) {
	if err := ValidateTarget("nope", map[string]any{}); err == nil {
		t.Fatal("未知目标应拒绝")
	}
	if err := ValidateTarget(TargetBackendSwitch, nil); err == nil {
		t.Fatal("nil 参数应拒绝")
	}
	if err := ValidateTarget(TargetBackendSwitch, map[string]any{"backend": "gemini"}); err != nil {
		t.Fatalf("backend_switch 合法: %v", err)
	}
	if err := ValidateTarget(TargetBackendSwitch, map[string]any{"backend": ""}); err == nil {
		t.Fatal("空 backend 应拒绝")
	}
	if err := ValidateTarget(TargetPipelineWeight,
		map[string]any{"weights": map[string]any{"gemini": float64(60), "azure": float64(40)}}); err != nil {
		t.Fatalf("weights 合法: %v", err)
	}
	if err := ValidateTarget(TargetPipelineWeight,
		map[string]any{"weights": map[string]any{"gemini": float64(120)}}); err == nil {
		t.Fatal("单权重越界应拒绝")
	}
	if err := ValidateTarget(TargetPipelineWeight,
		map[string]any{"weights": map[string]any{"gemini": float64(60), "azure": float64(50)}}); err == nil {
		t.Fatal("权重合计 >100 应拒绝")
	}
}

// 仓储辅助查询。
func TestStoreQueries(t *testing.T) {
	rt, _ := newTestRuntime(t)
	ctx := context.Background()
	p1, _ := rt.Propose(ctx, "s", TargetCacheTTL, map[string]any{"seconds": float64(60)}, "")
	p2, _ := rt.Propose(ctx, "s", TargetCacheTTL, map[string]any{"seconds": float64(120)}, "")
	got, err := rt.store.LatestByTarget(ctx, TargetCacheTTL)
	if err != nil {
		t.Fatalf("LatestByTarget: %v", err)
	}
	if got.ID != p2.ID {
		t.Fatalf("latest = %s, want %s", got.ID, p2.ID)
	}
	if err := rt.store.UpdateStatus(ctx, p1.ID, StatusApplied, now(), ""); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if err := rt.store.StoreEffect(ctx, p1.ID, `{"error_rate":-0.2}`); err != nil {
		t.Fatalf("StoreEffect: %v", err)
	}
	rows, err := rt.store.ListBySession(ctx, "s", 10)
	if err != nil {
		t.Fatalf("ListBySession: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].ID != p2.ID {
		t.Fatalf("降序期望首选 %s", p2.ID)
	}
	if _, err := rt.store.LatestByTarget(ctx, "ghost"); err == nil {
		t.Fatal("不存在 target 应报错")
	}
}

// cancel/rollback 加载链：不存在/状态不符的防御路径。
func TestLoadDefensive(t *testing.T) {
	rt, _ := newTestRuntime(t)
	ctx := context.Background()
	if err := rt.Dryrun(ctx, "ghost"); err == nil {
		t.Fatal("ghost dryrun 应报错")
	}
	if _, err := rt.RollbackOf(ctx, &LogRow{ID: "ghost", RollbackOf: "ghost-0"}); err == nil {
		t.Fatal("ghost rollback-of 应报错")
	}
	p, _ := rt.Propose(ctx, "s", TargetCacheTTL, map[string]any{"seconds": float64(60)}, "")
	if err := rt.Cancel(ctx, p.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	// 确认通过路径：Confirm(true, dryrunRequired=false) 直达 applied。
	p2, _ := rt.Propose(ctx, "s", TargetBackendSwitch, map[string]any{"backend": "gemini"}, "")
	if err := rt.Confirm(ctx, p2.ID, true, false); err != nil {
		t.Fatalf("Confirm(true): %v", err)
	}
	row2, _ := rt.store.Get(ctx, p2.ID)
	if row2.Status != StatusApplied {
		t.Fatalf("status = %q", row2.Status)
	}
	if rt.LearningDir() != "var/agent/learning" {
		t.Fatalf("LearningDir = %q", rt.LearningDir())
	}
	if driverFromDB("") != "sqlite" || driverFromDB("postgresql") != "postgresql" {
		t.Fatal("driverFromDB 归一化异常")
	}

	// canceled 后不可再 apply/dryrun。
	if err := rt.Dryrun(ctx, p.ID); err == nil {
		t.Fatal("canceled 后 dryrun 应拒绝")
	}
	if err := rt.Apply(ctx, p.ID, false); err == nil {
		t.Fatal("canceled 后 apply 应拒绝")
	}
}

// Set 链式辅助（测试用）：直接写目标参数域。
func (t *TargetStateStore) mustSet(target string, state TargetKeyState) {
	if err := t.Set(target, state); err != nil {
		panic(err)
	}
}

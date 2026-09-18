package server

import (
	"context"
	"database/sql"
	"testing"

	"centag/core/internal/agent/evolution"
	"centag/core/pkg/config"

	_ "modernc.org/sqlite"
)

// evolutionE2EDB 为 G5-4 端到端测试提供内存 sqlite。
func evolutionE2EDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// e2eMetricSource 是 G5-4 端到端测试用的固定观测面：dryrun 时给出改善的
// 指标（错误率下降、延迟下降），因此闭环应当放行 apply。
type e2eMetricSource struct {
	base map[string]float64
	meas map[string]float64
}

func (m *e2eMetricSource) Snapshot(ctx context.Context) (map[string]float64, error) {
	return m.base, nil
}

func (m *e2eMetricSource) Measure(ctx context.Context, p *evolution.Proposal) (map[string]float64, error) {
	return m.meas, nil
}

// TestEvolutionEndToEndClosedLoop 覆盖 G5-4：
// propose → dryrun → apply → rollback 全闭环，且宿主配置面真实变更/恢复。
//
// 与单测的差别：这里走 WireEvolutionHost 注册的**真实适配器**（retry_policy
// 落到 backend.Manager，cache_ttl 落到 config.Cache），而不是 fake applier，
// 因此同时验证 P0-1 的宿主应用面接线与 P0-2 的观测面注入。
func TestEvolutionEndToEndClosedLoop(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	oldMetric := evolution.DefaultMetricSource()
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() {
		evolution.HostAppliers = oldRegistry
		evolution.SetDefaultMetricSource(oldMetric)
	})

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	// P0-2：进程级默认观测面注入后，新建 Runtime 自动继承。
	evolution.SetDefaultMetricSource(&e2eMetricSource{
		base: map[string]float64{"error_rate": 0.05, "p95_latency_ms": 800, "token_cost": 1.0},
		meas: map[string]float64{"error_rate": 0.01, "p95_latency_ms": 500, "token_cost": 1.0},
	})
	WireEvolutionHost(sched, mgr, cacheMgr)

	// runtime 采用生产默认（requireMetric 仍为 true）；无显式 SetMetricSource
	// 时依赖默认观测面，验证 P0-2 的"默认继承"而非测试专用绕过。
	rt := evolution.NewRuntime(evolutionE2EDB(t), "sqlite", t.TempDir())
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	surface := evolution.NewSurface(rt, "sess-e2e", nil)
	surface.SetConfirm(func(p *evolution.Proposal) (bool, error) { return true, nil })

	ctx := context.Background()

	// 记录回滚点：把真实宿主当前值（backend b1 MaxRetries=2）镜像进
	// evolution 目标状态域，使首次提案的回滚点能恢复到 2。
	before, err := mgr.Get("b1")
	if err != nil {
		t.Fatalf("Get b1: %v", err)
	}
	if before.MaxRetries != 2 {
		t.Fatalf("前置 MaxRetries=%d want 2", before.MaxRetries)
	}
	if err := rt.SetTargetState(evolution.TargetRetryPolicy,
		evolution.TargetKeyState{"max_retries": float64(before.MaxRetries)}); err != nil {
		t.Fatalf("镜像宿主状态: %v", err)
	}

	// --- propose + dryrun + apply（真实链路） ---
	p, err := surface.RunOnce(ctx, evolution.TargetRetryPolicy,
		map[string]any{"max_retries": float64(5)}, "错误率下降")
	if err != nil {
		t.Fatalf("RunOnce(propose/dryrun/apply): %v", err)
	}

	// 状态迁移到 applied（审计日志）。
	row, err := rt.JournalRow(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if row.Status != evolution.StatusApplied {
		t.Fatalf("闭环后状态=%q want applied", row.Status)
	}
	if row.Effect == "" {
		t.Fatal("dryrun 度量未落库（effect_measure 缺失）")
	}

	// 真实宿主面：backend b1 的 MaxRetries 已变为 5。
	applied, err := mgr.Get("b1")
	if err != nil {
		t.Fatalf("Get b1: %v", err)
	}
	if applied.MaxRetries != 5 {
		t.Fatalf("宿主应用面未生效：MaxRetries=%d want 5", applied.MaxRetries)
	}

	// --- rollback（真实链路） ---
	if err := rt.Rollback(ctx, p.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	rolled, err := mgr.Get("b1")
	if err != nil {
		t.Fatalf("Get b1: %v", err)
	}
	if rolled.MaxRetries != 2 {
		t.Fatalf("回滚未恢复宿主面：MaxRetries=%d want 2", rolled.MaxRetries)
	}
	row, _ = rt.JournalRow(ctx, p.ID)
	if row.Status != evolution.StatusRolledBack {
		t.Fatalf("回滚后状态=%q want rolled_back", row.Status)
	}
}

// TestEvolutionEndToEndCancelKeepsConfig 覆盖 G5-4 的负向分支：
// dryrun 通过但 confirm=false → cancel，宿主配置不变。
func TestEvolutionEndToEndCancelKeepsConfig(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	oldMetric := evolution.DefaultMetricSource()
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() {
		evolution.HostAppliers = oldRegistry
		evolution.SetDefaultMetricSource(oldMetric)
	})

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	evolution.SetDefaultMetricSource(&e2eMetricSource{
		base: map[string]float64{"error_rate": 0.05, "p95_latency_ms": 800},
		meas: map[string]float64{"error_rate": 0.01, "p95_latency_ms": 500},
	})
	WireEvolutionHost(sched, mgr, cacheMgr)

	rt := evolution.NewRuntime(evolutionE2EDB(t), "sqlite", t.TempDir())
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	surface := evolution.NewSurface(rt, "sess-cancel", nil)
	surface.SetConfirm(func(p *evolution.Proposal) (bool, error) { return false, nil })

	ctx := context.Background()
	p, err := surface.RunOnce(ctx, evolution.TargetCacheTTL,
		map[string]any{"seconds": float64(86400)}, "")
	if err != nil {
		t.Fatalf("RunOnce(cancel): %v", err)
	}
	row, _ := rt.JournalRow(ctx, p.ID)
	if row.Status != evolution.StatusCanceled {
		t.Fatalf("状态=%q want canceled", row.Status)
	}
	// 宿主配置面未被取消的提案改动。
	if got := config.Get().Cache.DefaultTTL; got == 86400 {
		t.Fatalf("cancel 后 config.Cache.DefaultTTL 不应变为提案值 86400")
	}
}

// TestEvolutionEndToEndDryrunRegressionBlocksApply 覆盖 G5-4 的度量护栏：
// Measure 显示指标回退 → dryrun 失败，apply 被禁（状态保持 proposed）。
func TestEvolutionEndToEndDryrunRegressionBlocksApply(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	oldMetric := evolution.DefaultMetricSource()
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() {
		evolution.HostAppliers = oldRegistry
		evolution.SetDefaultMetricSource(oldMetric)
	})

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	evolution.SetDefaultMetricSource(&e2eMetricSource{
		base: map[string]float64{"error_rate": 0.01, "p95_latency_ms": 400},
		// 错误率 +0.10 超过默认阈值 0.02 → regression。
		meas: map[string]float64{"error_rate": 0.11, "p95_latency_ms": 2000},
	})
	WireEvolutionHost(sched, mgr, cacheMgr)

	rt := evolution.NewRuntime(evolutionE2EDB(t), "sqlite", t.TempDir())
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	ctx := context.Background()
	p, err := rt.Propose(ctx, "sess-regress", evolution.TargetRetryPolicy,
		map[string]any{"max_retries": float64(5)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Dryrun(ctx, p.ID); err == nil {
		t.Fatal("指标回退时 dryrun 应失败")
	}
	// apply 被 dryrun 门禁阻断。
	if err := rt.Apply(ctx, p.ID, true); err == nil {
		t.Fatal("dryrun 未通过时 apply 应被禁")
	}
	row, _ := rt.JournalRow(ctx, p.ID)
	if row.Status != evolution.StatusProposed {
		t.Fatalf("状态=%q want proposed（未迁移）", row.Status)
	}
}

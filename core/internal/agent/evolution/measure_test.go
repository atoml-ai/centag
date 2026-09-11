package evolution

import (
	"context"
	"testing"
)

// fakeMetricSource fake 观测面：Snapshot 固定基线；Measure 按 delta 注入。
type fakeMetricSource struct {
	base    map[string]float64
	meas    map[string]float64
	snapErr error
	measErr error
}

func (f *fakeMetricSource) Snapshot(ctx context.Context) (map[string]float64, error) {
	if f.snapErr != nil {
		return nil, f.snapErr
	}
	return f.base, nil
}
func (f *fakeMetricSource) Measure(ctx context.Context, p *Proposal) (map[string]float64, error) {
	if f.measErr != nil {
		return nil, f.measErr
	}
	return f.meas, nil
}

func newMeasureRuntime(t *testing.T, src MetricSource) *Runtime {
	t.Helper()
	rt, _ := newTestRuntime(t)
	rt.SetMetricSource(src)
	return rt
}

// TC-EVO-006 同指标/同窗口快照 delta 口径一致。
func TestMetricDeltaSameKeyset(t *testing.T) {
	before := map[string]float64{"error_rate": 0.02, "p95_latency_ms": 500, "token_cost": 1.0}
	after := map[string]float64{"error_rate": 0.01, "p95_latency_ms": 450}
	d := metricDelta(before, after)
	if d["error_rate"] != -0.01 || d["p95_latency_ms"] != -50 {
		t.Fatalf("delta 口径异常: %v", d)
	}
	// 交集口径：after 缺失键不进 delta。
	if _, ok := d["token_cost"]; ok {
		t.Fatal("delta 不应含非交集键")
	}
}

// TC-EVO-004 全闭环.good：dryrun 通过（观测面注入）→ confirm → apply + effect 快照。
func TestClosedLoopGoodEffect(t *testing.T) {
	src := &fakeMetricSource{
		base: map[string]float64{"error_rate": 0.02, "p95_latency_ms": 500},
		meas: map[string]float64{"error_rate": 0.01, "p95_latency_ms": 450},
	}
	rt := newMeasureRuntime(t, src)
	ctx := context.Background()
	p, err := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(3)}, "错误率下降")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Dryrun(ctx, p.ID); err != nil {
		t.Fatalf("Dryrun: %v", err)
	}
	if err := rt.Confirm(ctx, p.ID, true, true); err != nil {
		t.Fatalf("Confirm/Apply: %v", err)
	}
	row, _ := rt.store.Get(ctx, p.ID)
	if row.Status != StatusApplied || row.Effect == "" {
		t.Fatalf("状态/effect 缺失: %+v", row)
	}
	// effect_measure 含同窗口 before/after/delta + verdict。
	if _, err := rt.MeasureEffects(ctx, p.ID, "post"); err != nil {
		t.Fatalf("post 快照: %v", err)
	}
}

// TC-EVO-005 全闭环.bad：dryrun 指标回退超标 → 禁止 apply（回滚点零变化）。
func TestClosedLoopBadEffect(t *testing.T) {
	src := &fakeMetricSource{
		base: map[string]float64{"error_rate": 0.02, "p95_latency_ms": 500},
		meas: map[string]float64{"error_rate": 0.05, "p95_latency_ms": 900},
	}
	rt := newMeasureRuntime(t, src)
	ctx := context.Background()
	rt.state.mustSet(TargetCacheTTL, TargetKeyState{"seconds": float64(600)})
	p, err := rt.Propose(ctx, "s", TargetCacheTTL, map[string]any{"seconds": float64(120)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Dryrun(ctx, p.ID); err == nil {
		t.Fatal("指标回退应dryrun失败")
	}
	// dryrun 位未被置位 → apply 被禁。
	if err := rt.Apply(ctx, p.ID, true); err == nil {
		t.Fatal("dryrun 失败后 apply 应被禁止")
	}
	// 回滚点零变化：现有配置仍是 600。
	cur, _ := rt.state.Get(TargetCacheTTL)
	if cur["seconds"].(float64) != 600 {
		t.Fatalf("bad 路径配置被改: %v", cur)
	}
	// regression 也已审计留痕（verdict=regression）。
	row, _ := rt.store.Get(ctx, p.ID)
	if row.Effect == "" {
		t.Fatal("regression 未留审计")
	}
	// 状态仍是 proposed（可人工复核后 cancel）。
	if row.Status != StatusProposed {
		t.Fatalf("status = %q", row.Status)
	}
}

// dryrun 度量失败（观测面异常）→ dryrun 拒绝。
func TestDryrunMetricSourceError(t *testing.T) {
	rt := newMeasureRuntime(t, &fakeMetricSource{
		base:    map[string]float64{"error_rate": 0.01},
		measErr: context.DeadlineExceeded,
	})
	ctx := context.Background()
	p, _ := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(2)}, "")
	if err := rt.Dryrun(ctx, p.ID); err == nil {
		t.Fatal("度量失败应拒绝 dryrun")
	}
	if err := rt.Apply(ctx, p.ID, true); err == nil {
		t.Fatal("度量失败后 apply 应被禁止")
	}
	// 未注入观测面 → 纯方案校验路径仍可 dryrun。
	rt2, _ := newTestRuntime(t)
	p2, _ := rt2.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(2)}, "")
	if err := rt2.Dryrun(ctx, p2.ID); err != nil {
		t.Fatalf("无观测面 dryrun: %v", err)
	}
	// 未注入观测面 → MeasureEffects 报错。
	if _, err := rt2.MeasureEffects(ctx, p2.ID, "post"); err == nil {
		t.Fatal("无观测面 MeasureEffects 应报错")
	}
}

// 阈值覆盖：放宽/收紧阈值生效。
func TestThresholdOverride(t *testing.T) {
	rt, _ := newTestRuntime(t)
	rt.SetThresholds(MetricThresholds{ErrorRateMaxDelta: 1.0, LatencyMaxRatio: 99})
	if rt.thresholds != (MetricThresholds{ErrorRateMaxDelta: 1.0, LatencyMaxRatio: 99}) {
		t.Fatal("阈值覆盖未生效")
	}
	if DefaultMetricThresholds().ErrorRateMaxDelta != 0.02 {
		t.Fatal("默认阈值回退异常")
	}
}

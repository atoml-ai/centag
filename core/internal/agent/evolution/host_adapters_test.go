package evolution

import (
	"context"
	"sync"
	"testing"
)

// fakeApplier 测试用 TargetApplier：内存记录应用/回滚调用，可注入失败。
type fakeApplier struct {
	mu        sync.Mutex
	target    string
	applied   []TargetKeyState
	rolled    []TargetKeyState
	applyErr  error
	rollbackErr error
}

func (f *fakeApplier) Target() string { return f.target }
func (f *fakeApplier) Apply(_ context.Context, params TargetKeyState) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, params)
	return nil
}
func (f *fakeApplier) Rollback(_ context.Context, before TargetKeyState) error {
	if f.rollbackErr != nil {
		return f.rollbackErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rolled = append(f.rolled, before)
	return nil
}

// registerTestApplier 为某目标注册测试适配器（测试结束还原注册表）。
func registerTestApplier(t *testing.T, target string) *fakeApplier {
	t.Helper()
	a := &fakeApplier{target: target}
	prev := HostAppliers.Get(target)
	HostAppliers.Register(a)
	t.Cleanup(func() {
		if prev != nil {
			HostAppliers.Register(prev)
		} else {
			delete(HostAppliers.adapters, target)
		}
	})
	return a
}

// bareRuntime 未注册任何适配器、保持生产默认 fail-closed 的运行时。
func bareRuntime(t *testing.T) *Runtime {
	t.Helper()
	rt := NewRuntime(testDB(t), "sqlite", t.TempDir())
	if err := rt.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return rt
}

// P0-1 fail-closed：未注册适配器的目标 Apply 必须失败且 targets.json 回滚。
func TestApplyFailsClosedWithoutAdapter(t *testing.T) {
	rt := bareRuntime(t)
	ctx := context.Background()
	rt.state.mustSet(TargetRetryPolicy, TargetKeyState{"max_retries": float64(1)})
	p, err := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(5)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Apply(ctx, p.ID, false); err == nil {
		t.Fatal("无适配器注册应拒绝应用")
	}
	row, _ := rt.store.Get(ctx, p.ID)
	if row.Status == StatusApplied {
		t.Fatalf("无适配器时状态不应迁移到 applied: %s", row.Status)
	}
	cur, _ := rt.state.Get(TargetRetryPolicy)
	if cur["max_retries"].(float64) != 1 {
		t.Fatalf("无适配器时 targets.json 应保持快照: %v", cur)
	}
}

// P0-1：适配器 Apply 失败 → 状态不迁移、targets.json 回滚到 ParamsBefore。
func TestApplyAdapterFailureRestoresSnapshot(t *testing.T) {
	rt := bareRuntime(t)
	a := registerTestApplier(t, TargetRetryPolicy)
	a.applyErr = context.DeadlineExceeded
	ctx := context.Background()
	rt.state.mustSet(TargetRetryPolicy, TargetKeyState{"max_retries": float64(2)})
	p, err := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(4)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Apply(ctx, p.ID, false); err == nil {
		t.Fatal("适配器失败应返回错误")
	}
	row, _ := rt.store.Get(ctx, p.ID)
	if row.Status != StatusProposed {
		t.Fatalf("状态应停留在 proposed: %s", row.Status)
	}
	cur, _ := rt.state.Get(TargetRetryPolicy)
	if cur["max_retries"].(float64) != 2 {
		t.Fatalf("失败后快照应恢复: %v", cur)
	}
}

// P0-1：成功路径适配器收到参数；Rollback 走适配器恢复。
func TestApplyAndRollbackRouteThroughAdapter(t *testing.T) {
	rt := bareRuntime(t)
	a := registerTestApplier(t, TargetRetryPolicy)
	ctx := context.Background()
	rt.state.mustSet(TargetRetryPolicy, TargetKeyState{"max_retries": float64(1)})
	p, err := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(3)}, "")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := rt.Apply(ctx, p.ID, false); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(a.applied) != 1 || a.applied[0]["max_retries"].(float64) != 3 {
		t.Fatalf("适配器未收到应用参数: %v", a.applied)
	}
	if err := rt.Rollback(ctx, p.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(a.rolled) != 1 || a.rolled[0]["max_retries"].(float64) != 1 {
		t.Fatalf("适配器未收到回滚快照: %v", a.rolled)
	}
}

// P0-2：生产默认 requireMetric=true，无观测面 dryrun 拒绝；
// RequireMetric(false) 提供测试/降级逃生口。
func TestRequireMetricDefaultClosed(t *testing.T) {
	rt := bareRuntime(t)
	ctx := context.Background()
	p, _ := rt.Propose(ctx, "s", TargetRetryPolicy, map[string]any{"max_retries": float64(2)}, "")
	if err := rt.Dryrun(ctx, p.ID); err == nil {
		t.Fatal("默认无指标源 dryrun 应拒绝（fail-closed）")
	}
	rt.RequireMetric(false)
	if err := rt.Dryrun(ctx, p.ID); err != nil {
		t.Fatalf("RequireMetric(false) 后应放行: %v", err)
	}
}

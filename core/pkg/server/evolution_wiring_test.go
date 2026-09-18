package server

import (
	"context"
	"path/filepath"
	"testing"

	"centag/core/internal/agent/evolution"
	"centag/core/internal/cache"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/scheduler"
)

func TestWireEvolutionHost_RegistersAllTargets(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	oldMetric := evolution.DefaultMetricSource()
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() {
		evolution.HostAppliers = oldRegistry
		evolution.SetDefaultMetricSource(oldMetric)
	})

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	WireEvolutionHost(sched, mgr, cacheMgr)

	for _, target := range []string{
		evolution.TargetPipelineWeight,
		evolution.TargetRetryPolicy,
		evolution.TargetCacheTTL,
		evolution.TargetBackendSwitch,
	} {
		if ap := evolution.HostAppliers.Get(target); ap == nil {
			t.Fatalf("target %q 未注册宿主适配器", target)
		}
	}
	if evolution.DefaultMetricSource() == nil {
		t.Fatal("默认观测面未注入（P0-2 生产路径仍 fail-closed）")
	}
}

func TestPipelineWeightApplier_ApplyAndRollback(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() { evolution.HostAppliers = oldRegistry })

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	WireEvolutionHost(sched, mgr, cacheMgr)
	ap := evolution.HostAppliers.Get(evolution.TargetPipelineWeight)

	params := evolution.TargetKeyState{"weights": map[string]any{"price": 40.0, "quality": 30.0}}
	if err := ap.Apply(context.Background(), params); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := config.Get().Scheduler.Weights["price"]; got != 40 {
		t.Fatalf("持久化权重未更新：price=%d want 40", got)
	}

	if err := ap.Rollback(context.Background(), evolution.TargetKeyState{}); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if got := config.Get().Scheduler.Weights["price"]; got != config.DefaultSchedulerConfig().Weights["price"] {
		t.Fatalf("回滚未恢复默认：price=%d", got)
	}
}

func TestRetryPolicyApplier_ApplyAndRollback(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() { evolution.HostAppliers = oldRegistry })

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	WireEvolutionHost(sched, mgr, cacheMgr)
	ap := evolution.HostAppliers.Get(evolution.TargetRetryPolicy)

	if err := ap.Apply(context.Background(), evolution.TargetKeyState{"max_retries": 5.0}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	b, err := mgr.Get("b1")
	if err != nil {
		t.Fatalf("Get backend: %v", err)
	}
	if b.MaxRetries != 5 {
		t.Fatalf("后端重试次数未更新：%d want 5", b.MaxRetries)
	}

	if err := ap.Rollback(context.Background(), evolution.TargetKeyState{"max_retries": 2.0}); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	b, _ = mgr.Get("b1")
	if b.MaxRetries != 2 {
		t.Fatalf("回滚未恢复：%d want 2", b.MaxRetries)
	}
}

func TestCacheTTLApplier_ApplyAndRollback(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() { evolution.HostAppliers = oldRegistry })

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	WireEvolutionHost(sched, mgr, cacheMgr)
	ap := evolution.HostAppliers.Get(evolution.TargetCacheTTL)

	if err := ap.Apply(context.Background(), evolution.TargetKeyState{"seconds": 7200.0}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := config.Get().Cache.DefaultTTL; got != 7200 {
		t.Fatalf("缓存 TTL 未持久化：%d want 7200", got)
	}
}

func TestBackendSwitchApplier_Apply(t *testing.T) {
	oldRegistry := evolution.HostAppliers
	evolution.HostAppliers = evolution.NewAdapterRegistry()
	t.Cleanup(func() { evolution.HostAppliers = oldRegistry })

	sched, mgr, cacheMgr := newEvolutionTestHost(t)
	WireEvolutionHost(sched, mgr, cacheMgr)
	ap := evolution.HostAppliers.Get(evolution.TargetBackendSwitch)

	if err := ap.Apply(context.Background(), evolution.TargetKeyState{"backend": "b1"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	b, err := mgr.Get("b1")
	if err != nil {
		t.Fatalf("Get backend: %v", err)
	}
	if b.Weight != 100 {
		t.Fatalf("默认后端未切换：weight=%d want 100", b.Weight)
	}
}

// newEvolutionTestHost 构造最小宿主依赖，并把全局配置替换为测试配置。
func newEvolutionTestHost(t *testing.T) (*scheduler.Scheduler, *backend.Manager, *cache.Manager) {
	t.Helper()

	oldCfg := config.Get()
	cfg := &config.Config{
		Scheduler: config.DefaultSchedulerConfig(),
		Cache:     config.DefaultCacheConfig(),
	}
	config.Set(cfg)
	t.Cleanup(func() { config.Set(oldCfg) })

	mgr := backend.NewManager()
	mgr.SetStore(backend.NewFileBackendStore(filepath.Join(t.TempDir(), "backends.yaml")))
	if err := mgr.Add(&backend.BackendConfig{ID: "b1", Name: "b1", Type: "openai", Enabled: true, MaxRetries: 2}); err != nil {
		t.Fatalf("Add backend: %v", err)
	}

	sched := scheduler.NewScheduler(scheduler.DefaultSchedulerConfig(), mgr)
	cacheMgr, err := cache.NewManager(&cache.CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return sched, mgr, cacheMgr
}

package server

import (
	"context"
	"fmt"
	"time"

	"centag/core/internal/agent/evolution"
	"centag/core/internal/cache"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/scheduler"
)

// WireEvolutionHost 向自进化宿主面注册四类真实适配器（P0-1）。
// 这是生产链路的唯一注册入口：注册后 evolution.Apply 才会真正改动运行组件，
// 未注册目标继续保持 fail-closed（见 evolution.noApplierError）。
//
// 依赖为 nil 的宿主面会被跳过注册（对应目标的 Apply 将继续显式失败）。
func WireEvolutionHost(sched *scheduler.Scheduler, backends *backend.Manager, cacheMgr *cache.Manager) {
	// P0-2: 注入进程级默认观测面，使 NewRuntime 构造的实例不再"无度量放行"。
	if evolution.DefaultMetricSource() == nil {
		evolution.SetDefaultMetricSource(hostMetricSource{})
	}
	if sched != nil {
		evolution.HostAppliers.Register(pipelineWeightApplier{sched: sched})
	}
	if backends != nil {
		evolution.HostAppliers.Register(retryPolicyApplier{mgr: backends})
		evolution.HostAppliers.Register(backendSwitchApplier{mgr: backends})
	}
	// cacheMgr 允许为 nil：此时仍注册，仅做配置持久化（重启后生效）。
	evolution.HostAppliers.Register(cacheTTLApplier{mgr: cacheMgr})
}

// ---- pipeline_weight ----

type pipelineWeightApplier struct {
	sched *scheduler.Scheduler
}

func (a pipelineWeightApplier) Target() string { return evolution.TargetPipelineWeight }

func (a pipelineWeightApplier) Apply(_ context.Context, params evolution.TargetKeyState) error {
	weights, err := weightsFromState(params)
	if err != nil {
		return err
	}
	a.sched.SetScorerDefaultWeights(weights)

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("pipeline_weight: 全局配置未初始化")
	}
	if cfg.Scheduler.Weights == nil {
		cfg.Scheduler.Weights = map[string]int{}
	}
	for k, v := range weights {
		cfg.Scheduler.Weights[k] = v
	}
	return config.SaveConfig(cfg)
}

func (a pipelineWeightApplier) Rollback(_ context.Context, before evolution.TargetKeyState) error {
	weights, err := weightsFromState(before)
	if err != nil {
		// 回滚点无权重快照时，恢复出厂默认，避免"回滚不到位"。
		weights = config.DefaultSchedulerConfig().Weights
	}
	a.sched.SetScorerDefaultWeights(weights)

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("pipeline_weight: 全局配置未初始化")
	}
	cfg.Scheduler.Weights = weights
	return config.SaveConfig(cfg)
}

// weightsFromState 从宿主参数快照提取权重；缺失或非正权重返回错误。
func weightsFromState(state evolution.TargetKeyState) (map[string]int, error) {
	raw, ok := state["weights"]
	if !ok {
		return nil, fmt.Errorf("pipeline_weight: 缺少 weights")
	}
	var src map[string]any
	switch v := raw.(type) {
	case map[string]any:
		src = v
	case map[string]int:
		out := make(map[string]int, len(v))
		for k, n := range v {
			out[k] = n
		}
		return out, nil
	case map[string]float64:
		out := make(map[string]int, len(v))
		for k, n := range v {
			out[k] = int(n)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("pipeline_weight: weights 类型 %T 不受支持", raw)
	}
	if len(src) == 0 {
		return nil, fmt.Errorf("pipeline_weight: weights 为空")
	}
	out := make(map[string]int, len(src))
	for k, v := range src {
		f, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("pipeline_weight: %s 必须为数值", k)
		}
		out[k] = int(f)
	}
	return out, nil
}

// ---- retry_policy ----

type retryPolicyApplier struct {
	mgr *backend.Manager
}

func (a retryPolicyApplier) Target() string { return evolution.TargetRetryPolicy }

func (a retryPolicyApplier) Apply(_ context.Context, params evolution.TargetKeyState) error {
	n, err := maxRetriesFromState(params)
	if err != nil {
		return err
	}
	_, err = a.mgr.SetAllMaxRetries(n)
	return err
}

func (a retryPolicyApplier) Rollback(_ context.Context, before evolution.TargetKeyState) error {
	n, err := maxRetriesFromState(before)
	if err != nil {
		return nil // 无回滚点重试次数则不改动（保持当前值）
	}
	_, err = a.mgr.SetAllMaxRetries(n)
	return err
}

func maxRetriesFromState(state evolution.TargetKeyState) (int, error) {
	v, ok := state["max_retries"]
	if !ok {
		return 0, fmt.Errorf("retry_policy: 缺少 max_retries")
	}
	f, ok := toFloat(v)
	if !ok {
		return 0, fmt.Errorf("retry_policy: max_retries 必须为数值")
	}
	n := int(f)
	if n < 1 {
		return 0, fmt.Errorf("retry_policy: max_retries=%d 非法", n)
	}
	return n, nil
}

// ---- cache_ttl ----

type cacheTTLApplier struct {
	mgr *cache.Manager
}

func (a cacheTTLApplier) Target() string { return evolution.TargetCacheTTL }

func (a cacheTTLApplier) Apply(_ context.Context, params evolution.TargetKeyState) error {
	ttl, err := ttlSecondsFromState(params)
	if err != nil {
		return err
	}
	return a.setTTL(ttl)
}

func (a cacheTTLApplier) Rollback(_ context.Context, before evolution.TargetKeyState) error {
	ttl, err := ttlSecondsFromState(before)
	if err != nil {
		return nil // 无回滚点 TTL 则不改动
	}
	return a.setTTL(ttl)
}

func (a cacheTTLApplier) setTTL(seconds int) error {
	if a.mgr != nil {
		a.mgr.SetDefaultTTL(time.Duration(seconds) * time.Second)
	}
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("cache_ttl: 全局配置未初始化")
	}
	cfg.Cache.DefaultTTL = seconds
	return config.SaveConfig(cfg)
}

func ttlSecondsFromState(state evolution.TargetKeyState) (int, error) {
	v, ok := state["seconds"]
	if !ok {
		return 0, fmt.Errorf("cache_ttl: 缺少 seconds")
	}
	f, ok := toFloat(v)
	if !ok {
		return 0, fmt.Errorf("cache_ttl: seconds 必须为数值")
	}
	n := int(f)
	if n <= 0 {
		return 0, fmt.Errorf("cache_ttl: seconds=%d 非法", n)
	}
	return n, nil
}

// ---- backend_switch ----

type backendSwitchApplier struct {
	mgr *backend.Manager
}

func (a backendSwitchApplier) Target() string { return evolution.TargetBackendSwitch }

func (a backendSwitchApplier) Apply(_ context.Context, params evolution.TargetKeyState) error {
	id, err := backendFromState(params)
	if err != nil {
		return err
	}
	if err := a.mgr.SetDefault(id); err != nil {
		return err
	}
	return a.mgr.Save()
}

func (a backendSwitchApplier) Rollback(_ context.Context, before evolution.TargetKeyState) error {
	id, err := backendFromState(before)
	if err != nil {
		return nil // 无回滚点后端则不改动
	}
	if err := a.mgr.SetDefault(id); err != nil {
		return err
	}
	return a.mgr.Save()
}

func backendFromState(state evolution.TargetKeyState) (string, error) {
	v, ok := state["backend"]
	if !ok {
		return "", fmt.Errorf("backend_switch: 缺少 backend")
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("backend_switch: backend 必须为非空字符串")
	}
	return s, nil
}

// toFloat 兼容 JSON 反序列化后的 float64 与整数形态。
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

package evolution

import (
	"context"
	"encoding/json"
	"fmt"
)

// TC-EVO-006 口径：同指标/同窗口的快照可计算 delta。
const effectWindow = "default"

// MetricSource 观测面注入接口（fake 观测面即可跑通闭环）：
// Snapshot 按当前窗口取指标值；Measure 估算/探针提案变更后的指标值
// （宿主可用沙盒 replay 落地）。
type MetricSource interface {
	Snapshot(ctx context.Context) (map[string]float64, error)
	Measure(ctx context.Context, p *Proposal) (map[string]float64, error)
}

// MetricThresholds 指标回退阈值（超过即 dryrun 失败，禁止 apply）。
type MetricThresholds struct {
	// ErrorRateMaxDelta 错误率相对基线的最大允许增幅（绝对值，如 0.02）。
	ErrorRateMaxDelta float64
	// LatencyMaxRatio p95 延迟最大允许倍增（如 1.5 = 50% 增幅）。
	LatencyMaxRatio float64
}

// DefaultMetricThresholds 默认守卫阈值。
func DefaultMetricThresholds() MetricThresholds {
	return MetricThresholds{ErrorRateMaxDelta: 0.02, LatencyMaxRatio: 1.5}
}

type effectSnapshot struct {
	ProposalID string             `json:"proposal_id"`
	Phase      string             `json:"phase"`
	Window     string             `json:"window"`
	Metrics    map[string]float64 `json:"metrics"`
	CreatedAt  string             `json:"created_at"`
}

// EffectMeasure 存入 effect_measure 的口径（同指标同窗口 before/after + delta）。
type EffectMeasure struct {
	Before  map[string]float64 `json:"before"`
	After   map[string]float64 `json:"after"`
	Delta   map[string]float64 `json:"delta"`
	Window  string             `json:"window"`
	Verdict string             `json:"verdict"` // pass / regression
}

// SetMetricSource 注入观测面（nil = 关闭度量，dryrun 保持纯方案校验）。
func (r *Runtime) SetMetricSource(src MetricSource) { r.metric = src }

// SetBudget 注入循环预算（nil = 不限；TC-EVO-008）。
func (r *Runtime) SetBudget(b *LoopBudget) { r.budget = b }

// Charge 循环记账：无预算则零开销直通。
func (r *Runtime) Charge(iterations, tokens int) error { return r.budget.Charge(iterations, tokens) }

// SetThresholds 覆盖默认阈值。
func (r *Runtime) SetThresholds(t MetricThresholds) { r.thresholds = t }

// metricDelta 同口径快照对比（键集交集）。
func metricDelta(before, after map[string]float64) map[string]float64 {
	delta := make(map[string]float64)
	for k, b := range before {
		if a, ok := after[k]; ok {
			delta[k] = a - b
		}
	}
	return delta
}

// evaluateEffect dryrun 度量裁决（TC-EVO-005：错误率/延迟回退超标 → 拒绝）。
func (r *Runtime) evaluateEffect(ctx context.Context, p *Proposal, before map[string]float64) (EffectMeasure, error) {
	after, err := r.metric.Measure(ctx, p)
	if err != nil {
		return EffectMeasure{}, fmt.Errorf("dryrun 度量失败: %w", err)
	}
	delta := metricDelta(before, after)
	verdict := "pass"
	if er, ok := delta["error_rate"]; ok && er > r.thresholds.ErrorRateMaxDelta {
		verdict = "regression"
	}
	if lat, ok := delta["p95_latency_ms"]; ok && lat > 0 {
		if base, ok := before["p95_latency_ms"]; ok && base > 0 && after["p95_latency_ms"]/base > r.thresholds.LatencyMaxRatio {
			verdict = "regression"
		}
	}
	m := EffectMeasure{Before: before, After: after, Delta: delta, Window: effectWindow, Verdict: verdict}
	if verdict == "regression" {
		return m, fmt.Errorf("effect 度量回退（error_rate Δ=%v, p95_latency Δ=%v），禁止 apply",
			round2(delta["error_rate"]), round2(delta["p95_latency_ms"]))
	}
	return m, nil
}

func round2(v float64) float64 { return float64(int(v*100)) / 100 }

// MeasureEffects 快照落位：应用前后同指标快照对齐存 effect_measure（TC-EVO-006）。
// phase: "dryrun"（试算前基线）/ "apply"（试算后预测）/ "post"（实际观测回填）。
func (r *Runtime) MeasureEffects(ctx context.Context, proposalID, phase string) (*effectSnapshot, error) {
	row, err := r.store.Get(ctx, proposalID)
	if err != nil {
		return nil, fmt.Errorf("提案 %s 不存在: %w", proposalID, err)
	}
	var p Proposal
	if err := unmarshalProposal(row.Proposal, &p); err != nil {
		return nil, err
	}
	if r.metric == nil {
		return nil, fmt.Errorf("未注入 MetricSource，无法度量")
	}
	snap, err := r.metric.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	s := &effectSnapshot{ProposalID: proposalID, Phase: phase, Window: effectWindow, Metrics: snap, CreatedAt: now()}
	return s, nil
}

// storeMeasure 把度量结果写入 effect_measure（json 合并）。
func (r *Runtime) storeMeasure(ctx context.Context, proposalID string, m EffectMeasure) error {
	row, err := r.store.Get(ctx, proposalID)
	if err != nil {
		return err
	}
	var merged map[string]any
	if row.Effect != "" {
		_ = json.Unmarshal([]byte(row.Effect), &merged)
	}
	if merged == nil {
		merged = map[string]any{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	merged[m.Window] = json.RawMessage(b)
	merged["verdict"] = m.Verdict
	b, err = json.Marshal(merged)
	if err != nil {
		return err
	}
	return r.store.StoreEffect(ctx, proposalID, string(b))
}

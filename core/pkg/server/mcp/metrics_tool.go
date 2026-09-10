// read_metrics（A-T3）：运营指标只读查询。
// MCP 面不出现裸 SQL —— SQL 语义归 tokenusage.Service（方言/时间判断单一真源），
// 本包仅经 MetricsProvider 接口取数 + 时间窗口护栏（拒绝超 720h，风险 R07）。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atoml-ai/edgeag/pkg/agentcore"

	"centag/core/internal/tokenusage"
)

// MetricsProvider 运营指标取数接口（生产实现 = TokenUsageVolumeProvider）。
type MetricsProvider interface {
	// RequestVolume 返回最近 hours 小时按 backend 分桶的请求量与失败计数。
	// hours 超出护栏时返回错误，且不得执行任何 SQL（护栏前置）。
	RequestVolume(ctx context.Context, hours int) (MetricsVolume, error)
}

// MetricsVolume 聚合结果。
type MetricsVolume struct {
	Hours    int             `json:"hours"`
	Backends []BackendVolume `json:"backends"`
}

// BackendVolume 单个后端的请求量与失败计数。
type BackendVolume struct {
	BackendID string `json:"backend_id"`
	Requests  int64  `json:"requests"`
	Failed    int64  `json:"failed"`
}

// MaxMetricsHours 查询窗口护栏（小时，需求 A-T3：拒绝超 720h）。
const MaxMetricsHours = 720

// DefaultMetricsHours 缺省窗口（小时，无参数时用 24）。
const DefaultMetricsHours = 24

// TokenUsageVolumeProvider 生产 Provider：透传 tokenusage.Service 聚合方法。
type TokenUsageVolumeProvider struct {
	svc *tokenusage.Service
}

func NewTokenUsageVolumeProvider(svc *tokenusage.Service) *TokenUsageVolumeProvider {
	return &TokenUsageVolumeProvider{svc: svc}
}

func (p *TokenUsageVolumeProvider) RequestVolume(ctx context.Context, hours int) (MetricsVolume, error) {
	if hours < 1 {
		return MetricsVolume{}, fmt.Errorf("hours must be >= 1")
	}
	if hours > MaxMetricsHours {
		return MetricsVolume{}, fmt.Errorf("hours %d exceeds guard limit %d", hours, MaxMetricsHours)
	}
	if p == nil || p.svc == nil {
		return MetricsVolume{}, fmt.Errorf("metrics store not configured")
	}
	rows, err := p.svc.GetRequestVolumeByBackend(ctx, hours)
	if err != nil {
		return MetricsVolume{}, err
	}
	vol := MetricsVolume{Hours: hours, Backends: make([]BackendVolume, 0, len(rows))}
	for _, r := range rows {
		vol.Backends = append(vol.Backends, BackendVolume{
			BackendID: r.BackendID,
			Requests:  r.Requests,
			Failed:    r.Failed,
		})
	}
	return vol, nil
}

// readMetrics 工具本体（MCP 注册；挂到 agentcore.Tool 语义）。
type readMetricsTool struct {
	provider MetricsProvider
}

func newReadMetricsTool(p MetricsProvider) agentcore.Tool {
	return &readMetricsTool{provider: p}
}

func (t *readMetricsTool) Name() string { return "read_metrics" }

func (t *readMetricsTool) Description() string {
	return "查询最近 N 小时按 backend 分桶的请求量与失败计数（≤720h，只读）"
}

func (t *readMetricsTool) ParamSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hours": map[string]any{
				"type":        "integer",
				"description": fmt.Sprintf("查询窗口小时数（1-%d），缺省 %d", MaxMetricsHours, DefaultMetricsHours),
				"minimum":     1,
				"maximum":     MaxMetricsHours,
			},
		},
	}
}

func (t *readMetricsTool) IsReadOnly() bool { return true }

func (t *readMetricsTool) Execute(ctx context.Context, params map[string]any) (*agentcore.ToolResult, error) {
	if t == nil || t.provider == nil {
		return &agentcore.ToolResult{
			Content: "metrics provider 未注入",
			IsError: true,
		}, nil
	}
	hours := DefaultMetricsHours
	if n, ok := params["hours"]; ok && n != nil {
		fn, err := numericAsIntHours(n)
		if err != nil {
			return &agentcore.ToolResult{Content: err.Error(), IsError: true}, nil
		}
		hours = fn
	}
	if hours < 1 {
		return &agentcore.ToolResult{Content: "hours 需 ≥ 1", IsError: true}, nil
	}
	if hours > MaxMetricsHours {
		return &agentcore.ToolResult{
			Content: fmt.Sprintf("查询窗口 %dh 超出护栏上限 %dh（拒绝）", hours, MaxMetricsHours),
			IsError: true,
		}, nil
	}
	vol, err := t.provider.RequestVolume(ctx, hours)
	if err != nil {
		return &agentcore.ToolResult{Content: err.Error(), IsError: true}, nil
	}
	out, jerr := json.Marshal(vol)
	if jerr != nil {
		return &agentcore.ToolResult{Content: jerr.Error(), IsError: true}, nil
	}
	return &agentcore.ToolResult{Content: string(out), IsError: false}, nil
}

// numericAsIntHours 兼容 JSON 数字（float64 / json.Number / int）输入。
func numericAsIntHours(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("hours 需为整数")
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("hours 需为整数")
	}
}

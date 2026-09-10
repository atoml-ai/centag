package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/atoml-ai/edgeag/pkg/agentcore"
)

type mockProvider struct {
	hours int
	resp  MetricsVolume
	err   error
}

func (m *mockProvider) RequestVolume(_ context.Context, hours int) (MetricsVolume, error) {
	m.hours = hours
	if m.err != nil {
		return MetricsVolume{}, m.err
	}
	return m.resp, nil
}

func resultContent(t *testing.T, tr *agentcore.ToolResult) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal([]byte(tr.Content), &out); err != nil {
		t.Fatalf("unmarshal content: %v\nraw: %s", err, tr.Content)
	}
	return out
}

// TC-MCP-MET-001 read_metrics 正常：mock provider 24h 数据，结构化返回计数。
func TestReadMetrics_Normal(t *testing.T) {
	p := &mockProvider{resp: MetricsVolume{Hours: 24, Backends: []BackendVolume{
		{BackendID: "b1", Requests: 100, Failed: 3},
	}}}
	tool := newReadMetricsTool(p)
	res, err := tool.Execute(context.Background(), map[string]any{"hours": float64(24)})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error content: %s", res.Content)
	}
	out := resultContent(t, res)
	if got := out["hours"]; got != float64(24) {
		t.Fatalf("expected hours 24, got %v", got)
	}
	arr, ok := out["backends"].([]any)
	if !ok || len(arr) != 1 {
		t.Fatalf("expected 1 backend, got %v", out["backends"])
	}
	b := arr[0].(map[string]any)
	if b["backend_id"] != "b1" || b["requests"] != float64(100) || b["failed"] != float64(3) {
		t.Fatalf("unexpected backend entry: %v", b)
	}
	if p.hours != 24 {
		t.Fatalf("expected provider called with 24, got %d", p.hours)
	}
}

// TC-MCP-MET-004（回归 031）：无参数时取默认窗口 24h。
func TestReadMetrics_DefaultWindow(t *testing.T) {
	p := &mockProvider{resp: MetricsVolume{Hours: 24}}
	tool := newReadMetricsTool(p)
	res, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if p.hours != DefaultMetricsHours {
		t.Fatalf("expected default %d, got %d", DefaultMetricsHours, p.hours)
	}
}

// TC-MCP-MET-002 窗口超限护栏：>720h 拒绝且不触发 provider（不执行任何查询）。
func TestReadMetrics_WindowGuard(t *testing.T) {
	p := &mockProvider{resp: MetricsVolume{Hours: 720}}
	tool := newReadMetricsTool(p)
	for _, h := range []float64{721, 5000} {
		res, err := tool.Execute(context.Background(), map[string]any{"hours": h})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Fatalf("hours=%v expected rejection", h)
		}
		if p.hours != 0 {
			t.Fatalf("provider must not be called on guard rejection, got %d", p.hours)
		}
	}
}

// TC-MCP-MET-003（回归 031）：720h 边界允许通过；<1 拒绝。
func TestReadMetrics_WindowBounds(t *testing.T) {
	p := &mockProvider{resp: MetricsVolume{Hours: MaxMetricsHours}}
	tool := newReadMetricsTool(p)
	res, err := tool.Execute(context.Background(), map[string]any{"hours": float64(MaxMetricsHours)})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("720h should be allowed, got %s", res.Content)
	}
	if p.hours != MaxMetricsHours {
		t.Fatalf("expected provider called with 720, got %d", p.hours)
	}

	res2, _ := tool.Execute(context.Background(), map[string]any{"hours": float64(0)})
	if !res2.IsError {
		t.Fatal("expected rejection for hours<1")
	}

	// 非整数输入拒绝
	res3, _ := tool.Execute(context.Background(), map[string]any{"hours": "abc"})
	if !res3.IsError {
		t.Fatal("expected rejection for non-numeric hours")
	}
}

// TC-MCP-MET-005（回归 031）：provider 层错误透传为 IsError 内容。
func TestReadMetrics_ProviderError(t *testing.T) {
	sentinel := errors.New("boom")
	tool := newReadMetricsTool(&mockProvider{err: sentinel})
	res, err := tool.Execute(context.Background(), map[string]any{"hours": float64(24)})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || res.Content != sentinel.Error() {
		t.Fatalf("expected errored content, got %s (isError=%v)", res.Content, res.IsError)
	}
}

// TC-MCP-MET-006（回归 031）：未注入 provider 时返回明确错误内容（不 panic）。
func TestReadMetrics_NoProvider(t *testing.T) {
	tool := newReadMetricsTool(nil)
	res, err := tool.Execute(context.Background(), map[string]any{"hours": float64(24)})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected error when provider missing")
	}
}

func BenchmarkReadMetrics(b *testing.B) {
	p := &mockProvider{resp: MetricsVolume{Hours: 24}}
	tool := newReadMetricsTool(p)
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(ctx, map[string]any{"hours": float64(24)})
	}
}

// 用于防止 go vet 未使用告警
var _ = fmt.Sprintf

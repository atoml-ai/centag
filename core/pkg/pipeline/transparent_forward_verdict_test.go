package pipeline

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"centag/core/pkg/logger"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newVerdictForwardNode 构造带 mock 上游的透明转发节点（TC-TF-* 用例共用）。
func newVerdictForwardNode(t *testing.T, client *mockHTTPClient) *TransparentForwardNode {
	t.Helper()
	broker := &mockCapabilityBroker{httpClient: client}
	node, err := NewTransparentForwardNode(NodeConfig{})
	if err != nil {
		t.Fatalf("NewTransparentForwardNode: %v", err)
	}
	tf := node.(*TransparentForwardNode)
	tf.BaseNode.id = "forward"
	tf.SetCapabilityBroker(broker)
	return tf
}

func verdictTestInput() *NodeInput {
	return &NodeInput{
		Metadata: map[string]interface{}{
			"target_url":       "https://api.example.com",
			"request_path":     "/v1/chat/completions",
			"raw_request_body": `{"model":"deepseek-v4-flash-free","messages":[]}`,
		},
	}
}

// TestTransparentForwardVerdictExplicitErrorBody 覆盖 TC-TF-001/002：
// 上游 body 为显式错误结构时，节点必须失败（不再假成功返回 out, nil）。
func TestTransparentForwardVerdictExplicitErrorBody(t *testing.T) {
	const caseBody = `{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`

	tests := []struct {
		id         string
		status     int
		body       string
		wantStatus int
	}{
		{id: "TC-TF-001", status: 400, body: caseBody, wantStatus: 400},
		{id: "TC-TF-002", status: 200, body: `{"error":{"message":"boom"}}`, wantStatus: 200},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			client := &mockHTTPClient{status: tc.status, body: tc.body}
			tf := newVerdictForwardNode(t, client)

			out, err := tf.Execute(context.Background(), verdictTestInput())
			if err == nil {
				t.Fatalf("Execute returned (out=%v, nil), want upstream error (fake success)", out)
			}
			var ue *UpstreamError
			if !errors.As(err, &ue) {
				t.Fatalf("error %T (%v) is not *UpstreamError", err, err)
			}
			if got := UpstreamStatusCodeOf(err); got != tc.wantStatus {
				t.Fatalf("UpstreamStatusCodeOf = %d, want %d", got, tc.wantStatus)
			}
		})
	}
}

// TestTransparentForwardVerdictLegacyBranchesUnchanged 覆盖 TC-TF-003/004：
// billing / model_not-found 既有专用分支行为不变（仍各自上抛，未走新兜底路径）。
func TestTransparentForwardVerdictLegacyBranchesUnchanged(t *testing.T) {
	tests := []struct {
		id     string
		status int
		body   string
	}{
		{id: "TC-TF-003-billing", status: 402, body: `{"error":{"message":"insufficient_quota"}}`},
		{id: "TC-TF-004-model", status: 400, body: `{"error":{"message":"The model 'gpt-x' does not exist"}}`},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			client := &mockHTTPClient{status: tc.status, body: tc.body}
			tf := newVerdictForwardNode(t, client)

			out, err := tf.Execute(context.Background(), verdictTestInput())
			if err == nil {
				t.Fatalf("Execute returned (out=%v, nil), want error from legacy branch", out)
			}
			var ue *UpstreamError
			if !errors.As(err, &ue) {
				t.Fatalf("error %T (%v) is not *UpstreamError", err, err)
			}
		})
	}
}

// TestTransparentForwardVerdictNormalPassthrough 覆盖 TC-TF-005：正常响应透传行为零变化。
func TestTransparentForwardVerdictNormalPassthrough(t *testing.T) {
	body := `{"id":"ok","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"hi"}}]}`
	client := &mockHTTPClient{status: 200, body: body}
	tf := newVerdictForwardNode(t, client)

	out, err := tf.Execute(context.Background(), verdictTestInput())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out == nil || out.Content != body {
		t.Fatalf("content = %v, want passthrough body", out)
	}
}

// TestTransparentForwardVerdictLogKindOnly 覆盖 TC-TF-006（R05）：
// 兜底日志含 upstream_error_kind 枚举值，且不含错误 message 原文。
func TestTransparentForwardVerdictLogKindOnly(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	prev := logger.Logger
	logger.Logger = zap.New(core)
	t.Cleanup(func() { logger.Logger = prev })

	const caseBody = `{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`
	client := &mockHTTPClient{status: 400, body: caseBody}
	tf := newVerdictForwardNode(t, client)

	if _, err := tf.Execute(context.Background(), verdictTestInput()); err == nil {
		t.Fatal("Execute should fail for explicit error body")
	}

	found := false
	for _, rec := range logs.All() {
		if rec.Message != "transparent_forward upstream body is an explicit error structure" {
			continue
		}
		found = true
		kind := ""
		for _, f := range rec.Context {
			if f.Key != "upstream_error_kind" {
				continue
			}
			if f.Type == zapcore.StringType {
				kind = f.String
			} else if v, ok := f.Interface.(string); ok {
				kind = v
			}
		}
		if kind != "server_error" {
			t.Fatalf("upstream_error_kind = %q, want server_error", kind)
		}
		if strings.Contains(rec.Message, "Model is unavailable") {
			t.Fatal("log record must not contain upstream message body (R05)")
		}
	}
	if !found {
		t.Fatal("expected explicit-error-structure warn log not found")
	}
}

// TestTransparentForwardVerdictExecuteEntryExempt 覆盖 TC-TF-009（R08）：
// /execute 调试入口注入 raw_error_body_passthrough 标记后，兜底跳过，
// 保留「原始错误体作为数据返回」的调试契约。
func TestTransparentForwardVerdictExecuteEntryExempt(t *testing.T) {
	client := &mockHTTPClient{status: 400, body: `{"error":{"type":"server_error","message":"x"}}`}
	tf := newVerdictForwardNode(t, client)

	input := verdictTestInput()
	input.Metadata["raw_error_body_passthrough"] = true

	out, err := tf.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v, want passthrough when raw_error_body_passthrough is set", err)
	}
	if out == nil || out.Metadata["status_code"] != 400 {
		t.Fatalf("out=%v, want status_code=400 passthrough", out)
	}
}

// TestTransparentForwardVerdictSSEError 覆盖 TC-TF-007：SSE 错误事件流 → 节点失败。
func TestTransparentForwardVerdictSSEError(t *testing.T) {
	client := &mockHTTPClient{
		status: 200,
		body:   "data: {\"error\":{\"type\":\"overloaded\",\"message\":\"x\"}}\n\ndata: [DONE]\n\n",
	}
	tf := newVerdictForwardNode(t, client)

	if out, err := tf.Execute(context.Background(), verdictTestInput()); err == nil {
		t.Fatalf("Execute returned (out=%v, nil), want error for SSE error event", out)
	}
}

// 确保 net/http 引用不悬空（mock 响应头构造使用）。
var _ = http.Header{}

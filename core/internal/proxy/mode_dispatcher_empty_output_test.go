package proxy

import (
	"strings"
	"testing"

	"centag/core/pkg/pipeline"
)

func TestIsEmptyPipelineOutput(t *testing.T) {
	nonEmpty := &pipeline.PipelineOutput{Content: "hello"}
	reasoning := &pipeline.PipelineOutput{ReasoningContent: "think"}
	toolCalls := &pipeline.PipelineOutput{ToolCalls: []pipeline.ToolCall{{ID: "t1"}}}
	msgOnly := &pipeline.PipelineOutput{Messages: []pipeline.Message{{Role: "assistant", Content: "x"}}}
	empty := &pipeline.PipelineOutput{}

	cases := []struct {
		name        string
		output      *pipeline.PipelineOutput
		totalTokens int
		want        bool
	}{
		{"nil", nil, 0, false},
		{"empty output", empty, 0, true},
		{"content", nonEmpty, 10, false},
		{"reasoning only", reasoning, 5, false},
		{"tool calls", toolCalls, 0, false},
		{"messages only", msgOnly, 0, false},
		{"tokens only", empty, 3, false},
	}
	for _, tc := range cases {
		if got := isEmptyPipelineOutput(tc.output, tc.totalTokens); got != tc.want {
			t.Fatalf("%s: isEmptyPipelineOutput() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPipelineEmptyOutputHint(t *testing.T) {
	if got := pipelineEmptyOutputHint(nil); got != "" {
		t.Fatalf("nil output hint = %q", got)
	}

	withLogError := &pipeline.PipelineOutput{
		ExecutionLog: &pipeline.ExecutionLog{
			Success:      true,
			ErrorMessage: "upstream returned 429",
		},
	}
	if got := pipelineEmptyOutputHint(withLogError); got != "upstream returned 429" {
		t.Fatalf("log error hint = %q", got)
	}

	withNodeError := &pipeline.PipelineOutput{
		LastNode: "forward_fallback",
		ExecutionLog: &pipeline.ExecutionLog{
			NodeLogs: []pipeline.NodeExecutionLog{
				{NodeID: "forward", Success: false, ErrorMessage: "transparent_forward node \"forward\": upstream returned 429"},
				{NodeID: "forward_fallback", Success: true, Model: "deepseek-v4-flash-free"},
			},
		},
	}
	// 末节点（降级）成功但输出为空：提示应指明降级模型返回空响应，并附主节点原因。
	got := pipelineEmptyOutputHint(withNodeError)
	if !strings.Contains(got, "deepseek-v4-flash-free") || !strings.Contains(got, "empty response after fallback") {
		t.Fatalf("fallback-empty hint = %q", got)
	}

	withFailedOnly := &pipeline.PipelineOutput{
		LastNode: "forward",
		ExecutionLog: &pipeline.ExecutionLog{
			NodeLogs: []pipeline.NodeExecutionLog{
				{NodeID: "forward", Success: false, ErrorMessage: "upstream returned 500"},
			},
		},
	}
	if got := pipelineEmptyOutputHint(withFailedOnly); got != "upstream returned 500" {
		t.Fatalf("failed-node hint = %q", got)
	}

	if got := pipelineEmptyOutputHint(&pipeline.PipelineOutput{}); got != "" {
		t.Fatalf("no error hint = %q", got)
	}
}

package server

import (
	"testing"
)

func TestBuildTraceFromLogs_TimelineAndSummary(t *testing.T) {
	logs := []LogEntry{
		{
			Timestamp: "2026-06-10T14:39:59.224Z",
			Level:     "info",
			RequestID: "req-trace-1",
			Message:   "[Chat Completions] request started",
			Path:      "/v1/chat/completions",
			Extra:     map[string]string{"method": "POST"},
		},
		{
			Timestamp: "2026-06-10T14:39:59.224Z",
			Level:     "info",
			RequestID: "req-trace-1",
			Message:   "[Config] proxy mode",
			Extra:     map[string]string{"proxy_mode": "__default__", "source": "default"},
		},
		{
			Timestamp: "2026-06-10T14:39:59.225Z",
			Level:     "info",
			RequestID: "req-trace-1",
			Message:   "[Config] Resolved pipeline: direct-backend (source: system-default)",
			Extra:     map[string]string{"resolved_mode": "direct-backend", "resolved_source": "system-default"},
		},
		{
			Timestamp: "2026-06-10T14:39:59.225Z",
			Level:     "info",
			RequestID: "req-trace-1",
			Message:   "pipeline execution started",
			Extra:     map[string]string{"pipeline_id": "direct-backend", "total_nodes": "1"},
		},
		{
			Timestamp: "2026-06-10T14:39:59.225Z",
			Level:     "info",
			RequestID: "req-trace-1",
			Message:   "[generator] 发送请求",
			BackendID: "bigmodel",
			Model:     "glm-4-flash",
			Extra:     map[string]string{"node_id": "generator"},
		},
		{
			Timestamp:  "2026-06-10T14:40:00.051Z",
			Level:      "info",
			RequestID:  "req-trace-1",
			Message:    "[Request] completed",
			BackendID:  "bigmodel",
			Model:      "glm-4-flash",
			StatusCode: 200,
			DurationMs: 827,
		},
	}

	trace := buildTraceFromLogs("req-trace-1", logs)

	if trace.RawLogCount != 6 {
		t.Fatalf("raw_log_count = %d", trace.RawLogCount)
	}
	if trace.Summary.ProxyMode != "direct-backend" {
		t.Fatalf("summary.proxy_mode = %q", trace.Summary.ProxyMode)
	}
	if trace.Summary.PipelineID != "direct-backend" {
		t.Fatalf("summary.pipeline_id = %q", trace.Summary.PipelineID)
	}
	if trace.Summary.DurationMs != 827 {
		t.Fatalf("summary.duration_ms = %d", trace.Summary.DurationMs)
	}
	if trace.Summary.StatusCode != 200 {
		t.Fatalf("summary.status_code = %d", trace.Summary.StatusCode)
	}
	if !trace.Summary.Success {
		t.Fatal("expected success summary")
	}
	if trace.Routing.DetectedMode != "__default__" {
		t.Fatalf("routing.detected_mode = %q", trace.Routing.DetectedMode)
	}
	if trace.Routing.ResolvedMode != "direct-backend" {
		t.Fatalf("routing.resolved_mode = %q", trace.Routing.ResolvedMode)
	}
	if len(trace.Timeline) != 6 {
		t.Fatalf("timeline len = %d", len(trace.Timeline))
	}
	if trace.Timeline[0].Phase != "request" || trace.Timeline[0].Label != "请求进入" {
		t.Fatalf("first event = %+v", trace.Timeline[0])
	}
	if len(trace.PipelineGraph.ExecutedNodes) != 1 || trace.PipelineGraph.ExecutedNodes[0] != "generator" {
		t.Fatalf("executed_nodes = %v", trace.PipelineGraph.ExecutedNodes)
	}
}

func TestBuildTraceFromLogs_ProcessorNodesFromExtra(t *testing.T) {
	logs := []LogEntry{
		{
			Timestamp: "2026-06-11T17:12:02.542+08:00",
			Message:   "executing node",
			BackendID: "bigmodel",
			Model:     "GLM-4-flash",
			Extra:     map[string]string{"node_id": "mem0_retrieve", "node_type": "processor"},
		},
		{
			Timestamp: "2026-06-11T17:12:30.276+08:00",
			Message:   "executing node",
			BackendID: "bigmodel",
			Model:     "GLM-4-flash",
			Extra:     map[string]string{"node_id": "mem0_storage", "node_type": "processor"},
		},
	}

	trace := buildTraceFromLogs("req-mem0", logs)
	want := []string{"mem0_retrieve", "mem0_storage"}
	if len(trace.PipelineGraph.ExecutedNodes) != len(want) {
		t.Fatalf("executed_nodes = %v, want %v", trace.PipelineGraph.ExecutedNodes, want)
	}
	for i, node := range want {
		if trace.PipelineGraph.ExecutedNodes[i] != node {
			t.Fatalf("executed_nodes[%d] = %q, want %q", i, trace.PipelineGraph.ExecutedNodes[i], node)
		}
	}
}

func TestClassifyTraceEvent_Error(t *testing.T) {
	phase, label, _, _ := classifyTraceEvent(LogEntry{
		Level:   "error",
		Message: "[Request] failed to parse request",
	})
	if phase != "error" || label != "请求失败" {
		t.Fatalf("got phase=%q label=%q", phase, label)
	}
}
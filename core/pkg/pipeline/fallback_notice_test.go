package pipeline

import (
	"strings"
	"testing"

	"centag/core/pkg/config"
)

func TestAnnotateFallbackNotice_MetadataOnly(t *testing.T) {
	out := &NodeOutput{
		Content: "hello",
		Metadata: map[string]interface{}{
			"billing_fallback_used":       true,
			"billing_fallback_from_model": "gpt-5.6-luna",
			"billing_fallback_to_model":   "deepseek-v4-flash-free",
		},
	}
	AnnotateFallbackNotice(out)
	if out.Content != "hello" {
		t.Fatalf("content should stay unchanged, got %q", out.Content)
	}
	if out.Metadata["fallback_notice_applied"] != true {
		t.Fatal("expected notice applied flag")
	}
	if !strings.Contains(fmtString(out.Metadata["fallback_notice"]), "gpt-5.6-luna") {
		t.Fatalf("expected notice metadata, got %v", out.Metadata["fallback_notice"])
	}
	// idempotent
	AnnotateFallbackNotice(out)
	if out.Content != "hello" {
		t.Fatalf("content mutated on second call: %q", out.Content)
	}
}

func TestAnnotateFallbackNotice_SSE_NoContentChange(t *testing.T) {
	orig := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"
	out := &NodeOutput{
		Content: orig,
		Metadata: map[string]interface{}{
			"fallback_used":       true,
			"fallback_from_model": "a",
			"fallback_to_model":   "b",
		},
	}
	AnnotateFallbackNotice(out)
	if out.Content != orig {
		t.Fatalf("SSE content should stay unchanged, got %q", out.Content)
	}
	if !strings.Contains(fmtString(out.Metadata["fallback_notice"]), "a → b") &&
		!strings.Contains(fmtString(out.Metadata["fallback_notice"]), "a") {
		t.Fatalf("missing notice metadata: %v", out.Metadata["fallback_notice"])
	}
}

func TestApplyResponseTraceBanner_Off(t *testing.T) {
	config.Set(&config.Config{Proxy: config.ProxyConfig{ResponseTraceBanner: false}})
	out := &PipelineOutput{
		Content: "hello",
		Metadata: map[string]interface{}{
			"executor_backend": "opencode-zen",
			"executor_model":   "deepseek-v4-flash-free",
		},
	}
	ApplyResponseTraceBanner(out, "transparent-proxy")
	if out.Content != "hello" {
		t.Fatalf("want unchanged content when switch off, got %q", out.Content)
	}
}

func TestApplyResponseTraceBanner_OnWithFallback(t *testing.T) {
	config.Set(&config.Config{Proxy: config.ProxyConfig{ResponseTraceBanner: true}})
	out := &PipelineOutput{
		Content: "hello",
		Metadata: map[string]interface{}{
			"executor_backend":            "opencode-zen",
			"executor_model":              "deepseek-v4-flash-free",
			"billing_fallback_used":       true,
			"billing_fallback_from_model": "gpt-5.6-luna",
			"billing_fallback_to_model":   "deepseek-v4-flash-free",
		},
		ExecutionLog: &ExecutionLog{
			PipelineID: "transparent-proxy",
			NodeLogs: []NodeExecutionLog{
				{NodeID: "generate", Success: true},
			},
		},
	}
	ApplyResponseTraceBanner(out, "transparent-proxy")
	wantPrefix := "[Centag] req → transparent-proxy:generate → opencode-zen/gpt-5.6-luna→deepseek-v4-flash-free → resp"
	if !strings.HasPrefix(out.Content, wantPrefix) {
		t.Fatalf("banner mismatch:\n want prefix %q\n got %q", wantPrefix, out.Content)
	}
	if !strings.Contains(out.Content, "hello") {
		t.Fatal("original content lost")
	}
	ApplyResponseTraceBanner(out, "transparent-proxy")
	if strings.Count(out.Content, "[Centag] req →") != 1 {
		t.Fatalf("banner applied twice: %q", out.Content)
	}
}

func TestBuildResponseTraceBanner_NodeChain(t *testing.T) {
	got := buildResponseTraceBanner(
		"translate-mode",
		"generate→translate",
		"openai",
		"gpt-4o",
		"",
		"",
		false,
	)
	want := "[Centag] req → translate-mode:generate→translate → openai/gpt-4o → resp\n--------------------\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNodePathFromExecLog(t *testing.T) {
	path := nodePathFromExecLog(&ExecutionLog{
		NodeLogs: []NodeExecutionLog{
			{NodeID: "generate", Success: true},
			{NodeID: "generate", Success: true}, // consecutive dedupe
			{NodeID: "translate", Success: true},
		},
	})
	if path != "generate→translate" {
		t.Fatalf("got %q", path)
	}
}

func fmtString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

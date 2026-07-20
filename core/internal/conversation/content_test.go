package conversation

import "testing"

func TestNormalizeAssistantContent_FromSSE(t *testing.T) {
	sse := ": OPENROUTER PROCESSING\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"改动\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"正确\"}}]}\n\n" +
		"data: [DONE]\n\n"
	got := NormalizeAssistantContent(sse)
	if got != "改动正确" {
		t.Fatalf("got %q, want 改动正确", got)
	}
}

func TestNormalizeAssistantContent_ToolCallsOnly(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"name\":\"bash\"}}]}}]}\n\n" +
		"data: [DONE]\n\n"
	got := NormalizeAssistantContent(sse)
	if got != "[tool_calls: bash]" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeAssistantContent_PlainPassthrough(t *testing.T) {
	if got := NormalizeAssistantContent("hello"); got != "hello" {
		t.Fatalf("got %q", got)
	}
}


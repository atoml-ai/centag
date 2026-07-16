package pipeline

import "testing"

func TestRequestIDFromInput(t *testing.T) {
	if got := RequestIDFromInput(nil); got != "" {
		t.Fatalf("nil input: got %q", got)
	}
	if got := RequestIDFromInput(&PipelineInput{}); got != "" {
		t.Fatalf("empty metadata: got %q", got)
	}
	input := &PipelineInput{Metadata: map[string]interface{}{"request_id": "req-123"}}
	if got := RequestIDFromInput(input); got != "req-123" {
		t.Fatalf("got %q, want req-123", got)
	}
}
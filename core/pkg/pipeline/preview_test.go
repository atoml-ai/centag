package pipeline

import (
	"strings"
	"testing"
)

func TestFormatMessagesPreview(t *testing.T) {
	preview := FormatMessagesPreview([]Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
	}, 4000)
	if preview == "" {
		t.Fatal("expected non-empty preview")
	}
	for _, part := range []string{"[system]", "[user]", "Hello"} {
		if !strings.Contains(preview, part) {
			t.Fatalf("preview missing %q: %q", part, preview)
		}
	}
}

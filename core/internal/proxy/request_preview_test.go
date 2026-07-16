package proxy

import (
	"strings"
	"testing"

	"centag/core/pkg/plugin"
)

func TestFormatMessagesPreview(t *testing.T) {
	preview := formatMessagesPreview([]plugin.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}, 100)
	if !strings.Contains(preview, "[user] hello") {
		t.Fatalf("preview = %q", preview)
	}
	if !strings.Contains(preview, "[assistant] hi there") {
		t.Fatalf("preview = %q", preview)
	}
}

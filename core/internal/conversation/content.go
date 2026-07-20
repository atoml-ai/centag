package conversation

import (
	"encoding/json"
	"strings"
)

// NormalizeAssistantContent turns buffered upstream SSE / chat.completion JSON into
// readable assistant text for storage and browse UI. Non-stream plain text is unchanged.
func NormalizeAssistantContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if text := extractTextFromChatBody(content); text != "" {
		return text
	}
	if summary := extractToolCallSummary(content); summary != "" {
		return summary
	}
	return content
}

func extractTextFromChatBody(content string) string {
	var b strings.Builder

	// Full JSON chat.completion / message body
	if appendJSONMessageText(&b, content) {
		if s := strings.TrimSpace(b.String()); s != "" {
			return s
		}
	}

	if !strings.Contains(content, "data:") {
		return ""
	}
	b.Reset()
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		_ = appendJSONMessageText(&b, payload)
	}
	return strings.TrimSpace(b.String())
}

func appendJSONMessageText(b *strings.Builder, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[DONE]" {
		return false
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Content string `json:"content"` // some envelopes
	}
	if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
		return false
	}
	wrote := false
	for _, ch := range chunk.Choices {
		if ch.Delta.Content != "" {
			b.WriteString(ch.Delta.Content)
			wrote = true
		} else if ch.Message.Content != "" {
			b.WriteString(ch.Message.Content)
			wrote = true
		} else if ch.Text != "" {
			b.WriteString(ch.Text)
			wrote = true
		}
	}
	if !wrote && chunk.Content != "" {
		b.WriteString(chunk.Content)
		wrote = true
	}
	return wrote
}

func extractToolCallSummary(content string) string {
	names := make([]string, 0, 4)
	seen := map[string]struct{}{}

	try := func(raw string) {
		var chunk struct {
			Choices []struct {
				Delta struct {
					ToolCalls []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				Message struct {
					ToolCalls []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			return
		}
		add := func(name string) {
			name = strings.TrimSpace(name)
			if name == "" {
				return
			}
			if _, ok := seen[name]; ok {
				return
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
		for _, ch := range chunk.Choices {
			for _, tc := range ch.Delta.ToolCalls {
				add(tc.Function.Name)
			}
			for _, tc := range ch.Message.ToolCalls {
				add(tc.Function.Name)
			}
		}
	}

	if strings.Contains(content, "data:") {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			try(payload)
		}
	} else {
		try(content)
	}
	if len(names) == 0 {
		return ""
	}
	return "[tool_calls: " + strings.Join(names, ", ") + "]"
}

package openai

import (
	"encoding/json"
	"testing"
)

func TestStreamToolCallNormalizer_AggregatesAcrossChunks(t *testing.T) {
	normalizer := &streamToolCallNormalizer{}

	raw1 := buildRawDeltaChunk(`准备调用工具<minimax:tool_call><invoke name="exec">{"command":"pwd"}`)
	out, handled := normalizer.processChunk(raw1, `准备调用工具<minimax:tool_call><invoke name="exec">{"command":"pwd"}`, "", false)
	if !handled {
		t.Fatalf("first chunk should be handled by normalizer")
	}
	if len(out) != 0 {
		t.Fatalf("first chunk should not emit output, got %d", len(out))
	}

	raw2 := buildRawDeltaChunk(`</invoke></minimax:tool_call>`)
	out, handled = normalizer.processChunk(raw2, `</invoke></minimax:tool_call>`, "stop", false)
	if !handled {
		t.Fatalf("second chunk should be handled by normalizer")
	}
	if len(out) != 1 {
		t.Fatalf("second chunk should emit one output, got %d", len(out))
	}

	chunk := out[0]
	if chunk.Content != "准备调用工具" {
		t.Fatalf("normalized content = %q, want 准备调用工具", chunk.Content)
	}
	if !chunk.Done {
		t.Fatalf("chunk.Done = false, want true")
	}
	if chunk.FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", chunk.FinishReason)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(chunk.RawData, &raw); err != nil {
		t.Fatalf("normalized raw json invalid: %v", err)
	}
	choices, _ := raw["choices"].([]interface{})
	if len(choices) == 0 {
		t.Fatalf("choices missing in normalized raw json")
	}
	choice, _ := choices[0].(map[string]interface{})
	delta, _ := choice["delta"].(map[string]interface{})
	if _, exists := delta["tool_calls"]; !exists {
		t.Fatalf("delta.tool_calls missing in normalized raw json")
	}
}

func TestStreamToolCallNormalizer_FlushesIncompleteBufferOnFinish(t *testing.T) {
	normalizer := &streamToolCallNormalizer{}

	raw := buildRawDeltaChunk(`<minimax:tool_call><invoke name="exec">{"command":"pwd"}`)
	out, handled := normalizer.processChunk(raw, `<minimax:tool_call><invoke name="exec">{"command":"pwd"}`, "stop", false)
	if !handled {
		t.Fatalf("chunk should be handled by normalizer")
	}
	if len(out) != 1 {
		t.Fatalf("should flush one chunk on finish, got %d", len(out))
	}
	if !out[0].Done {
		t.Fatalf("flushed chunk.Done = false, want true")
	}
	if out[0].FinishReason != "stop" {
		t.Fatalf("flushed chunk finish reason = %q, want stop", out[0].FinishReason)
	}
}

func TestShouldBufferToolCallChunk(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "minimax tag", content: `<minimax:tool_call>`, want: true},
		{name: "toolcall tag", content: `<toolcall>`, want: true},
		{name: "invoke only", content: `<invoke name="exec">`, want: true},
		{name: "normal text", content: `hello world`, want: false},
	}
	for _, tc := range cases {
		got := shouldBufferToolCallChunk(tc.content)
		if got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func buildRawDeltaChunk(content string) []byte {
	raw := map[string]interface{}{
		"id":      "chatcmpl-test",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "test-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": map[string]interface{}{
					"content": content,
				},
				"finish_reason": nil,
			},
		},
	}
	data, _ := json.Marshal(raw)
	return data
}

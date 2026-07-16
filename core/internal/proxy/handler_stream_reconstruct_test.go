package proxy

import (
	"encoding/json"
	"testing"

	"centag/core/pkg/plugin"
)

func TestBuildReconstructedStreamChunkData_PreservesToolCallsFromRaw(t *testing.T) {
	raw := map[string]interface{}{
		"id":      "chatcmpl-raw",
		"object":  "chat.completion.chunk",
		"created": float64(1),
		"model":   "raw-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": float64(0),
				"delta": map[string]interface{}{
					"reasoning_content": "thinking...",
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id":   "call_1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "exec",
								"arguments": "{\"command\":\"pwd\"}",
							},
						},
					},
				},
				"finish_reason": nil,
			},
		},
	}
	rawBytes, _ := json.Marshal(raw)

	data := buildReconstructedStreamChunkData("fallback-model", plugin.StreamChunk{
		Content:              "normalized text",
		RawData:              rawBytes,
		ContentIsNonStandard: true,
	})

	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("choices missing in reconstructed chunk: %#v", data["choices"])
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("choice[0] type = %T, want map[string]interface{}", choices[0])
	}
	delta, ok := choice0["delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("delta type = %T, want map[string]interface{}", choice0["delta"])
	}
	if got, _ := delta["content"].(string); got != "normalized text" {
		t.Fatalf("delta.content = %q, want normalized text", got)
	}
	if _, exists := delta["tool_calls"]; !exists {
		t.Fatalf("delta.tool_calls missing after reconstruction")
	}
	if _, exists := delta["reasoning_content"]; exists {
		t.Fatalf("delta.reasoning_content should be removed in reconstructed chunk")
	}
}

func TestBuildReconstructedStreamChunkData_FallbackWhenRawInvalid(t *testing.T) {
	data := buildReconstructedStreamChunkData("fallback-model", plugin.StreamChunk{
		Content:              "hello",
		RawData:              []byte("{invalid-json"),
		ContentIsNonStandard: true,
	})

	if got, _ := data["model"].(string); got != "fallback-model" {
		t.Fatalf("model = %q, want fallback-model", got)
	}
	choices, ok := data["choices"].([]map[string]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("choices type = %T, want []map[string]interface{}", data["choices"])
	}
	delta, ok := choices[0]["delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("delta type = %T, want map[string]interface{}", choices[0]["delta"])
	}
	if got, _ := delta["content"].(string); got != "hello" {
		t.Fatalf("delta.content = %q, want hello", got)
	}
	if choices[0]["finish_reason"] != nil {
		t.Fatalf("finish_reason = %#v, want nil", choices[0]["finish_reason"])
	}
}

func TestBuildReconstructedStreamChunkData_SetsFinishReasonWhenPresent(t *testing.T) {
	raw := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"delta": map[string]interface{}{},
			},
		},
	}
	rawBytes, _ := json.Marshal(raw)

	data := buildReconstructedStreamChunkData("test-model", plugin.StreamChunk{
		Content:      "",
		FinishReason: "tool_calls",
		RawData:      rawBytes,
	})

	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("choices missing")
	}
	choice0, _ := choices[0].(map[string]interface{})
	if got, _ := choice0["finish_reason"].(string); got != "tool_calls" {
		t.Fatalf("finish_reason = %q, want tool_calls", got)
	}
}

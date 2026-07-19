//go:build protocol_openairesponses

package openairesponses

import (
	"encoding/json"
	"testing"
)

func TestResponsesInput_StringAndArray(t *testing.T) {
	var ri responsesInput
	if err := json.Unmarshal([]byte(`"hello"`), &ri); err != nil {
		t.Fatal(err)
	}
	if ri.StringVal != "hello" {
		t.Fatalf("StringVal=%q", ri.StringVal)
	}

	ri = responsesInput{}
	raw := `[{"role":"user","content":"plain string"},{"type":"message","role":"assistant","content":[{"type":"input_text","text":"parts"}]}]`
	if err := json.Unmarshal([]byte(raw), &ri); err != nil {
		t.Fatal(err)
	}
	if len(ri.Items) != 2 {
		t.Fatalf("items=%d", len(ri.Items))
	}
	if got := ri.Items[0].Content.PlainText(); got != "plain string" {
		t.Fatalf("item0=%q", got)
	}
	if got := ri.Items[1].Content.PlainText(); got != "parts" {
		t.Fatalf("item1=%q", got)
	}
}

func TestResponsesRequest_OpenCodeZenShape(t *testing.T) {
	// Mirrors the failure: input[].content as string (not []contentPart)
	body := []byte(`{
		"model":"hy3-free",
		"stream":true,
		"input":[
			{"role":"system","content":"You are opencode"},
			{"role":"user","content":"你使用的什么大模型"}
		]
	}`)
	var req responsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Model != "hy3-free" || len(req.Input.Items) != 2 {
		t.Fatalf("model=%q items=%d", req.Model, len(req.Input.Items))
	}
	if got := req.Input.Items[1].Content.PlainText(); got != "你使用的什么大模型" {
		t.Fatalf("user content=%q", got)
	}
}

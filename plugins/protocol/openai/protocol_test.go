package openai

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseRequest_PreservesRawBodyAndToolFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{
		"model":"test-model",
		"stream":true,
		"tool_choice":"auto",
		"tools":[{"type":"function","function":{"name":"exec","description":"run cmd","parameters":{"type":"object"}}}],
		"messages":[
			{"role":"user","content":"hello"},
			{
				"role":"assistant",
				"content":"",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{\"command\":\"pwd\"}"}}]
			},
			{"role":"tool","tool_call_id":"call_1","content":"done"}
		]
	}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))

	p := &Protocol{}
	req, err := p.ParseRequest(c)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	rawBody, ok := req.RawBody.(map[string]interface{})
	if !ok {
		t.Fatalf("RawBody type = %T, want map[string]interface{}", req.RawBody)
	}
	if _, exists := rawBody["tools"]; !exists {
		t.Fatalf("RawBody missing tools field")
	}
	if got, ok := rawBody["tool_choice"].(string); !ok || got != "auto" {
		t.Fatalf("RawBody tool_choice = %#v, want auto", rawBody["tool_choice"])
	}

	if len(req.Messages) != 3 {
		t.Fatalf("Messages length = %d, want 3", len(req.Messages))
	}
	if len(req.Messages[1].ToolCalls) != 1 {
		t.Fatalf("assistant message tool calls = %d, want 1", len(req.Messages[1].ToolCalls))
	}
	if req.Messages[1].ToolCalls[0].Function.Name != "exec" {
		t.Fatalf("assistant tool call name = %s, want exec", req.Messages[1].ToolCalls[0].Function.Name)
	}
	if req.Messages[2].ToolCallID != "call_1" {
		t.Fatalf("tool message tool_call_id = %s, want call_1", req.Messages[2].ToolCallID)
	}
}

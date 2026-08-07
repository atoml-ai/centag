package pipeline

import (
	"encoding/json"
	"testing"
)

func TestReasoningRoundtrip_RepairByToolCallID(t *testing.T) {
	resetReasoningRoundtripStoreForTest()
	meta := map[string]interface{}{"session_id": "s1", "user_id": "u1"}

	resp := []byte(`{
		"choices":[{
			"message":{
				"role":"assistant",
				"content":null,
				"reasoning_content":"step-by-step plan",
				"tool_calls":[{"id":"call_abc","type":"function","function":{"name":"search","arguments":"{}"}}]
			},
			"finish_reason":"tool_calls"
		}]
	}`)
	applyReasoningRoundtripOnResponse(meta, nil, resp, 200)

	req := []byte(`{
		"model":"deepseek-v4-flash",
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_abc","type":"function","function":{"name":"search","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_abc","content":"ok"}
		],
		"tools":[{"type":"function","function":{"name":"search","parameters":{}}}]
	}`)
	out, n := applyReasoningRoundtripOnRequest(meta, req)
	if n != 1 {
		t.Fatalf("repaired=%d want 1", n)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	msgs := raw["messages"].([]interface{})
	asst := msgs[1].(map[string]interface{})
	if asst["reasoning_content"] != "step-by-step plan" {
		t.Fatalf("reasoning_content=%v", asst["reasoning_content"])
	}
}

func TestReasoningRoundtrip_NoInjectWhenNeverSeen(t *testing.T) {
	resetReasoningRoundtripStoreForTest()
	meta := map[string]interface{}{"session_id": "s-never"}

	req := []byte(`{
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","tool_calls":[{"id":"call_x","type":"function","function":{"name":"f","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_x","content":"ok"}
		]
	}`)
	out, n := applyReasoningRoundtripOnRequest(meta, req)
	if n != 0 {
		t.Fatalf("repaired=%d want 0 (no prior reasoning)", n)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	asst := raw["messages"].([]interface{})[1].(map[string]interface{})
	if _, ok := asst["reasoning_content"]; ok {
		t.Fatalf("must not inject reasoning_content for inactive session: %v", asst)
	}
}

func TestReasoningRoundtrip_LearnFromInboundThenSkipIfPresent(t *testing.T) {
	resetReasoningRoundtripStoreForTest()
	meta := map[string]interface{}{"session_id": "s-learn"}

	req := []byte(`{
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","reasoning_content":"already here","tool_calls":[{"id":"call_1","type":"function","function":{"name":"f","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"ok"}
		]
	}`)
	out, n := applyReasoningRoundtripOnRequest(meta, req)
	if n != 0 {
		t.Fatalf("repaired=%d want 0 when field already present", n)
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(out, &raw)
	asst := raw["messages"].([]interface{})[1].(map[string]interface{})
	if asst["reasoning_content"] != "already here" {
		t.Fatalf("got %v", asst["reasoning_content"])
	}
}

func TestReasoningRoundtrip_OrdinalFallbackWhenToolIDChanges(t *testing.T) {
	resetReasoningRoundtripStoreForTest()
	meta := map[string]interface{}{"session_id": "s-ord"}

	resp := []byte(`{
		"choices":[{
			"message":{
				"reasoning_content":"think-A",
				"tool_calls":[{"id":"old_id","type":"function","function":{"name":"f","arguments":"{}"}}]
			}
		}]
	}`)
	applyReasoningRoundtripOnResponse(meta, nil, resp, 200)

	req := []byte(`{
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","tool_calls":[{"id":"new_id","type":"function","function":{"name":"f","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"new_id","content":"ok"}
		]
	}`)
	out, n := applyReasoningRoundtripOnRequest(meta, req)
	if n != 1 {
		t.Fatalf("repaired=%d want 1", n)
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(out, &raw)
	asst := raw["messages"].([]interface{})[1].(map[string]interface{})
	if asst["reasoning_content"] != "think-A" {
		t.Fatalf("ordinal fallback got %v", asst["reasoning_content"])
	}
}

func TestReasoningRoundtrip_UserIsolation(t *testing.T) {
	resetReasoningRoundtripStoreForTest()
	metaA := map[string]interface{}{"session_id": "same-sid", "user_id": "alice"}
	metaB := map[string]interface{}{"session_id": "same-sid", "user_id": "bob"}

	resp := []byte(`{"choices":[{"message":{"reasoning_content":"secret","tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]}}]}`)
	applyReasoningRoundtripOnResponse(metaA, nil, resp, 200)

	req := []byte(`{"messages":[{"role":"user","content":"x"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]}]}`)
	_, n := applyReasoningRoundtripOnRequest(metaB, req)
	if n != 0 {
		t.Fatalf("bob must not see alice cache, repaired=%d", n)
	}
}

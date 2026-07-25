package promptstrategy

import (
	"encoding/json"
	"testing"
)

func TestSystemMode_IsValid(t *testing.T) {
	tests := []struct {
		mode SystemMode
		want bool
	}{
		{SystemModePassthrough, true},
		{SystemModeAppend, true},
		{SystemModeReplace, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.IsValid(); got != tt.want {
				t.Errorf("SystemMode.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplySystemStrategy_Passthrough(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "client system"},
		{Role: "user", Content: "hello"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          SystemModePassthrough,
		GatewayPrompt: "gateway system",
		Messages:      messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied {
		t.Error("passthrough should not apply strategy")
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
}

func TestApplySystemStrategy_Replace(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "client system"},
		{Role: "user", Content: "hello"},
		{Role: "system", Content: "another system"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          SystemModeReplace,
		GatewayPrompt: "gateway system",
		Messages:      messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Applied {
		t.Error("replace should apply strategy")
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || result.Messages[0].Content != "gateway system" {
		t.Errorf("first message should be gateway system, got %v", result.Messages[0])
	}
	if result.Messages[1].Role != "user" {
		t.Errorf("second message should be user, got %v", result.Messages[1])
	}
}

func TestApplySystemStrategy_Append_AfterClient(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "client system"},
		{Role: "user", Content: "hello"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:           SystemModeAppend,
		GatewayPrompt:  "gateway system",
		AppendPosition: AppendPositionAfterClient,
		Messages:       messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Applied {
		t.Error("append should apply strategy")
	}
	if len(result.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "client system" {
		t.Errorf("first system should be client, got %v", result.Messages[0])
	}
	if result.Messages[1].Content != "gateway system" {
		t.Errorf("second system should be gateway, got %v", result.Messages[1])
	}
	if result.Messages[2].Role != "user" {
		t.Errorf("third should be user, got %v", result.Messages[2])
	}
}

func TestApplySystemStrategy_Append_BeforeClient(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "client system"},
		{Role: "user", Content: "hello"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:           SystemModeAppend,
		GatewayPrompt:  "gateway system",
		AppendPosition: AppendPositionBeforeClient,
		Messages:       messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "gateway system" {
		t.Errorf("first should be gateway, got %v", result.Messages[0])
	}
	if result.Messages[1].Content != "client system" {
		t.Errorf("second should be client, got %v", result.Messages[1])
	}
}

func TestApplySystemStrategy_Append_MergeLast(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "client system"},
		{Role: "user", Content: "hello"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:           SystemModeAppend,
		GatewayPrompt:  "gateway system",
		AppendPosition: AppendPositionMergeLast,
		Messages:       messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "client system\ngateway system" {
		t.Errorf("merged system should combine, got %v", result.Messages[0])
	}
}

func TestApplySystemStrategy_Append_NoExistingSystem(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          SystemModeAppend,
		GatewayPrompt: "gateway system",
		Messages:      messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || result.Messages[0].Content != "gateway system" {
		t.Errorf("first should be gateway system, got %v", result.Messages[0])
	}
}

func TestApplySystemStrategy_EmptyGatewayPrompt(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "client system"},
		{Role: "user", Content: "hello"},
	}

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          SystemModeReplace,
		GatewayPrompt: "",
		Messages:      messages,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied {
		t.Error("empty gateway prompt should not apply strategy")
	}
}

func TestApplySystemStrategy_InvalidMode(t *testing.T) {
	_, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          "invalid",
		GatewayPrompt: "gateway system",
		Messages:      []Message{},
	})

	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestResolveSystemMode(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name       string
		strategy   string
		injectBool *bool
		want       SystemMode
	}{
		{"explicit passthrough", "passthrough", nil, SystemModePassthrough},
		{"explicit append", "append", nil, SystemModeAppend},
		{"explicit replace", "replace", nil, SystemModeReplace},
		{"case insensitive", "PASSTHROUGH", nil, SystemModePassthrough},
		{"inject true", "", &trueVal, SystemModeReplace},
		{"inject false", "", &falseVal, SystemModePassthrough},
		{"strategy priority over inject", "append", &trueVal, SystemModeAppend},
		{"default passthrough", "", nil, SystemModePassthrough},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSystemMode(tt.strategy, tt.injectBool)
			if got != tt.want {
				t.Errorf("ResolveSystemMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplySystemStrategy_RawBodySync(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "gateway system"},
		{Role: "user", Content: "hello"},
	}

	rawBody := []byte(`{"model":"test","messages":[{"role":"system","content":"old"},{"role":"user","content":"hello"}]}`)

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          SystemModeReplace,
		GatewayPrompt: "gateway system",
		Messages:      messages,
		RawBody:       rawBody,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(result.RawBody, &body); err != nil {
		t.Fatalf("failed to unmarshal raw body: %v", err)
	}

	msgs, ok := body["messages"].([]interface{})
	if !ok {
		t.Fatal("messages not found in raw body")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages in raw body, got %d", len(msgs))
	}

	firstMsg := msgs[0].(map[string]interface{})
	if firstMsg["content"] != "gateway system" {
		t.Errorf("first message content should be 'gateway system', got %v", firstMsg["content"])
	}
}

func TestReplaceSystemMessages(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "system 1"},
		{Role: "user", Content: "hello"},
		{Role: "system", Content: "system 2"},
		{Role: "assistant", Content: "hi"},
	}

	result := replaceSystemMessages(messages, "new system")

	if len(result) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result))
	}
	if result[0].Content != "new system" {
		t.Errorf("first should be new system, got %v", result[0])
	}
	if result[1].Role != "user" {
		t.Errorf("second should be user, got %v", result[1])
	}
	if result[2].Role != "assistant" {
		t.Errorf("third should be assistant, got %v", result[2])
	}
}

func TestParseChatBody(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		msgs, err := ParseChatBody([]byte(`{"messages":[{"role":"user","content":"hi"},{"role":"system","content":"sys"}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Content != "sys" {
			t.Fatalf("%#v", msgs)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if _, err := ParseChatBody(nil); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("no messages", func(t *testing.T) {
		if _, err := ParseChatBody([]byte(`{"model":"x"}`)); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		if _, err := ParseChatBody([]byte(`{`)); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSyncMessagesToRawBody(t *testing.T) {
	in := []byte(`{"model":"m","messages":[{"role":"user","content":"old"}]}`)
	out, err := SyncMessagesToRawBody(in, []Message{{Role: "user", Content: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["model"] != "m" {
		t.Fatalf("model lost: %v", raw["model"])
	}
	msgs := raw["messages"].([]interface{})
	first := msgs[0].(map[string]interface{})
	if first["content"] != "new" {
		t.Fatalf("content=%v", first["content"])
	}
	empty, err := SyncMessagesToRawBody(nil, nil)
	if err != nil || empty != nil {
		t.Fatalf("empty body: %v %v", empty, err)
	}
}

func TestApplySystemStrategy_RawBodyPreservesToolCallsAndMultimodal(t *testing.T) {
	rawBody := []byte(`{
		"model":"m",
		"messages":[
			{"role":"system","content":"old sys"},
			{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image_url","image_url":{"url":"http://x"}}]},
			{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"c1","content":"ok"}
		]
	}`)

	result, err := ApplySystemStrategy(SystemApplyInput{
		Mode:          SystemModeReplace,
		GatewayPrompt: "gateway",
		RawBody:       rawBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("expected applied")
	}

	var body map[string]interface{}
	if err := json.Unmarshal(result.RawBody, &body); err != nil {
		t.Fatal(err)
	}
	msgs := body["messages"].([]interface{})
	if len(msgs) != 4 {
		t.Fatalf("len=%d want 4", len(msgs))
	}
	sys := msgs[0].(map[string]interface{})
	if sys["content"] != "gateway" {
		t.Fatalf("system=%v", sys["content"])
	}
	user := msgs[1].(map[string]interface{})
	if _, ok := user["content"].([]interface{}); !ok {
		t.Fatalf("multimodal content lost: %#v", user["content"])
	}
	asst := msgs[2].(map[string]interface{})
	if _, ok := asst["tool_calls"].([]interface{}); !ok {
		t.Fatalf("tool_calls lost: %#v", asst)
	}
	tool := msgs[3].(map[string]interface{})
	if tool["tool_call_id"] != "c1" {
		t.Fatalf("tool_call_id lost: %#v", tool)
	}
}

func TestApplySystemStrategy_RawBodyAppendPositions(t *testing.T) {
	rawBody := []byte(`{
		"messages":[
			{"role":"system","content":"client"},
			{"role":"user","content":"hi","name":"u1"}
		]
	}`)

	t.Run("after_client", func(t *testing.T) {
		result, err := ApplySystemStrategy(SystemApplyInput{
			Mode:           SystemModeAppend,
			GatewayPrompt:  "gw",
			AppendPosition: AppendPositionAfterClient,
			RawBody:        rawBody,
			Messages:       []Message{{Role: "system", Content: "client"}, {Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]interface{}
		_ = json.Unmarshal(result.RawBody, &body)
		msgs := body["messages"].([]interface{})
		if len(msgs) != 3 {
			t.Fatalf("len=%d", len(msgs))
		}
		if msgs[1].(map[string]interface{})["content"] != "gw" {
			t.Fatalf("gateway not after client: %#v", msgs)
		}
		if msgs[2].(map[string]interface{})["name"] != "u1" {
			t.Fatalf("user ext field lost: %#v", msgs[2])
		}
	})

	t.Run("before_client", func(t *testing.T) {
		result, err := ApplySystemStrategy(SystemApplyInput{
			Mode:           SystemModeAppend,
			GatewayPrompt:  "gw",
			AppendPosition: AppendPositionBeforeClient,
			RawBody:        rawBody,
			Messages:       []Message{{Role: "system", Content: "client"}, {Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]interface{}
		_ = json.Unmarshal(result.RawBody, &body)
		msgs := body["messages"].([]interface{})
		if msgs[0].(map[string]interface{})["content"] != "gw" {
			t.Fatalf("want gateway first: %#v", msgs[0])
		}
	})

	t.Run("merge_last", func(t *testing.T) {
		result, err := ApplySystemStrategy(SystemApplyInput{
			Mode:           SystemModeAppend,
			GatewayPrompt:  "gw",
			AppendPosition: AppendPositionMergeLast,
			RawBody:        rawBody,
			Messages:       []Message{{Role: "system", Content: "client"}, {Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]interface{}
		_ = json.Unmarshal(result.RawBody, &body)
		msgs := body["messages"].([]interface{})
		if len(msgs) != 2 {
			t.Fatalf("len=%d", len(msgs))
		}
		if msgs[0].(map[string]interface{})["content"] != "client\ngw" {
			t.Fatalf("merge content=%v", msgs[0].(map[string]interface{})["content"])
		}
	})

	t.Run("append_no_system", func(t *testing.T) {
		noSys := []byte(`{"messages":[{"role":"user","content":"hi","name":"u1"}]}`)
		result, err := ApplySystemStrategy(SystemApplyInput{
			Mode:          SystemModeAppend,
			GatewayPrompt: "gw",
			RawBody:       noSys,
			Messages:      []Message{{Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]interface{}
		_ = json.Unmarshal(result.RawBody, &body)
		msgs := body["messages"].([]interface{})
		if len(msgs) != 2 || msgs[0].(map[string]interface{})["content"] != "gw" {
			t.Fatalf("%#v", msgs)
		}
		if msgs[1].(map[string]interface{})["name"] != "u1" {
			t.Fatalf("ext lost: %#v", msgs[1])
		}
	})
}

func TestSyncMessagesToRawBody_PreservesToolCalls(t *testing.T) {
	in := []byte(`{"messages":[{"role":"assistant","content":"","tool_calls":[{"id":"c1"}]},{"role":"user","content":"x"}]}`)
	out, err := SyncMessagesToRawBody(in, []Message{
		{Role: "assistant", Content: ""},
		{Role: "user", Content: "y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	asst := raw["messages"].([]interface{})[0].(map[string]interface{})
	if _, ok := asst["tool_calls"].([]interface{}); !ok {
		t.Fatalf("tool_calls not preserved: %#v", asst)
	}
	user := raw["messages"].([]interface{})[1].(map[string]interface{})
	if user["content"] != "y" {
		t.Fatalf("user content=%v", user["content"])
	}
}

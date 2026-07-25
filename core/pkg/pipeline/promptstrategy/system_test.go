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

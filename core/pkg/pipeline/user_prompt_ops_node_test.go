package pipeline

import (
	"context"
	"encoding/json"
	"testing"
)

func TestUserPromptOpsNode_Type(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"check": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}
	if node.Type() != NodeTypeUserPromptOps {
		t.Errorf("expected type %s, got %s", NodeTypeUserPromptOps, node.Type())
	}
}

func TestUserPromptOpsNode_CheckDenyPatterns(t *testing.T) {
	tests := []struct {
		name        string
		denyPattern string
		content     string
		wantBlock   bool
		wantRedact  bool
	}{
		{
			name:        "API key pattern blocks",
			denyPattern: `(?i)sk-[a-z0-9]{20,}`,
			content:     "Use sk-abcdefghijklmnopqrstuvwxyz123456 as the key",
			wantBlock:   true,
		},
		{
			name:        "No match passes",
			denyPattern: `(?i)sk-[a-z0-9]{20,}`,
			content:     "This is a normal message",
			wantBlock:   false,
		},
		{
			name:        "Case insensitive match",
			denyPattern: `(?i)password`,
			content:     "My PASSWORD is secret",
			wantBlock:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NodeConfig{
				CustomConfig: map[string]interface{}{
					"check": map[string]interface{}{
						"enabled":      true,
						"deny_patterns": []interface{}{tt.denyPattern},
						"on_hit":       "block",
					},
				},
			}
			node, err := NewUserPromptOpsNode(config)
			if err != nil {
				t.Fatalf("NewUserPromptOpsNode failed: %v", err)
			}

			input := &NodeInput{Content: tt.content}
			_, err = node.Execute(context.Background(), input)

			if tt.wantBlock {
				if err == nil {
					t.Error("expected block error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestUserPromptOpsNode_OptimizeTruncate(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"optimize": map[string]interface{}{
				"enabled":       true,
				"max_user_chars": 20,
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "This is a long message that should be truncated"}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// TruncateString adds "..." when truncating
	if len(output.Content) > 23 { // 20 + "..."
		t.Errorf("expected content truncated, got %d: %q", len(output.Content), output.Content)
	}
}

func TestUserPromptOpsNode_OptimizeWhitespace(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"optimize": map[string]interface{}{
				"enabled":             true,
				"collapse_whitespace": true,
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "Hello   World\t\tTest"}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content != "Hello World Test" {
		t.Errorf("expected whitespace collapsed, got: %q", output.Content)
	}
}

func TestUserPromptOpsNode_WithRawBody(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"check": map[string]interface{}{
				"enabled":      true,
				"deny_patterns": []interface{}{`(?i)secret`},
				"on_hit":       "block",
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}

	rawBody := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Normal message"},
			map[string]interface{}{"role": "user", "content": "My secret is here"},
		},
	}
	bodyBytes, _ := json.Marshal(rawBody)

	input := &NodeInput{
		Content: "test",
		Metadata: map[string]interface{}{
			"raw_request_body": string(bodyBytes),
		},
	}

	_, err = node.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected block error for secret in raw body, got nil")
	}
}

func TestUserPromptOpsNode_NoContent(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"check": map[string]interface{}{
				"enabled":      true,
				"deny_patterns": []interface{}{`(?i)secret`},
				"on_hit":       "block",
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: ""}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content != "" {
		t.Errorf("expected empty content, got: %q", output.Content)
	}
}

func TestUserPromptOpsNode_InvalidPattern(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"check": map[string]interface{}{
				"enabled":      true,
				"deny_patterns": []interface{}{`[invalid`},
			},
		},
	}
	_, err := NewUserPromptOpsNode(config)
	if err == nil {
		t.Error("expected error for invalid regex pattern, got nil")
	}
}

func TestUserPromptOpsNode_LooksLikeSecretKey(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"sk-abcdefghijklmnopqrstuvwxyz", true},
		{"AKIAIOSFODNN7EXAMPLE", false}, // AK_ prefix not present
		{"api_key=abc123", true},
		{"-----BEGIN RSA PRIVATE KEY-----", true},
		{"normal text", false},
		{"Bearer token123", true},
	}

	for _, tt := range tests {
		got := looksLikeSecretKey(tt.text)
		if got != tt.want {
			t.Errorf("looksLikeSecretKey(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestUserPromptOpsNode_RedactAction(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"check": map[string]interface{}{
				"enabled":      true,
				"deny_patterns": []interface{}{`(?i)secret`},
				"on_hit":       "redact",
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "This is secret information"}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content != "[REDACTED]" {
		t.Errorf("expected redacted content, got: %q", output.Content)
	}
}

func TestUserPromptOpsNode_LogAction(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"check": map[string]interface{}{
				"enabled":      true,
				"deny_patterns": []interface{}{`(?i)secret`},
				"on_hit":       "log",
			},
		},
	}
	node, err := NewUserPromptOpsNode(config)
	if err != nil {
		t.Fatalf("NewUserPromptOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "This is secret information"}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// log action should pass through content
	if output.Content != "This is secret information" {
		t.Errorf("expected original content, got: %q", output.Content)
	}
}

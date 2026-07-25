package pipeline

import (
	"context"
	"testing"
)

func TestOutputPostOpsNode_Type(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"ops": []interface{}{"trim_space"},
		},
	}
	node, err := NewOutputPostOpsNode(config)
	if err != nil {
		t.Fatalf("NewOutputPostOpsNode failed: %v", err)
	}
	if node.Type() != NodeTypeOutputPostOps {
		t.Errorf("expected type %s, got %s", NodeTypeOutputPostOps, node.Type())
	}
}

func TestOutputPostOpsNode_TrimSpace(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"ops": []interface{}{"trim_space"},
		},
	}
	node, err := NewOutputPostOpsNode(config)
	if err != nil {
		t.Fatalf("NewOutputPostOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "  Hello World  "}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content != "Hello World" {
		t.Errorf("expected trimmed content, got: %q", output.Content)
	}
}

func TestOutputPostOpsNode_StripMarkdownFence(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "json fence",
			content: "```json\n{\"key\": \"value\"}\n```",
			want:    `{"key": "value"}`,
		},
		{
			name:    "plain fence",
			content: "```\nsome code\n```",
			want:    "some code",
		},
		{
			name:    "no fence",
			content: "plain text",
			want:    "plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NodeConfig{
				CustomConfig: map[string]interface{}{
					"ops": []interface{}{"strip_markdown_fence"},
				},
			}
			node, err := NewOutputPostOpsNode(config)
			if err != nil {
				t.Fatalf("NewOutputPostOpsNode failed: %v", err)
			}

			input := &NodeInput{Content: tt.content}
			output, err := node.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			if output.Content != tt.want {
				t.Errorf("expected %q, got %q", tt.want, output.Content)
			}
		})
	}
}

func TestOutputPostOpsNode_ExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "valid json object",
			content: `{"key": "value"}`,
			want:    `{"key": "value"}`,
		},
		{
			name:    "json with surrounding text",
			content: `Here is the result: {"key": "value"} done`,
			want:    `{"key": "value"}`,
		},
		{
			name:    "invalid json",
			content: "not json at all",
			want:    "not json at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NodeConfig{
				CustomConfig: map[string]interface{}{
					"ops": []interface{}{"extract_json"},
				},
			}
			node, err := NewOutputPostOpsNode(config)
			if err != nil {
				t.Fatalf("NewOutputPostOpsNode failed: %v", err)
			}

			input := &NodeInput{Content: tt.content}
			output, err := node.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			if output.Content != tt.want {
				t.Errorf("expected %q, got %q", tt.want, output.Content)
			}
		})
	}
}

func TestOutputPostOpsNode_JSONCompact(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"ops": []interface{}{"json_compact"},
		},
	}
	node, err := NewOutputPostOpsNode(config)
	if err != nil {
		t.Fatalf("NewOutputPostOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: `{"key": "value", "nested": {"a": 1}}`}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := `{"key":"value","nested":{"a":1}}`
	if output.Content != expected {
		t.Errorf("expected compacted JSON, got: %q", output.Content)
	}
}

func TestOutputPostOpsNode_CombinedOps(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"ops": []interface{}{"strip_markdown_fence", "extract_json", "json_compact"},
		},
	}
	node, err := NewOutputPostOpsNode(config)
	if err != nil {
		t.Fatalf("NewOutputPostOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "```json\n{\"key\":   \"value\"}\n```"}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := `{"key":"value"}`
	if output.Content != expected {
		t.Errorf("expected %q, got %q", expected, output.Content)
	}
}

func TestOutputPostOpsNode_EmptyContent(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"ops": []interface{}{"trim_space"},
		},
	}
	node, err := NewOutputPostOpsNode(config)
	if err != nil {
		t.Fatalf("NewOutputPostOpsNode failed: %v", err)
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

func TestOutputPostOpsNode_OnInvalidJSON(t *testing.T) {
	config := NodeConfig{
		CustomConfig: map[string]interface{}{
			"ops":              []interface{}{"extract_json"},
			"on_invalid_json":  "wrap_error_object",
		},
	}
	node, err := NewOutputPostOpsNode(config)
	if err != nil {
		t.Fatalf("NewOutputPostOpsNode failed: %v", err)
	}

	input := &NodeInput{Content: "not json"}
	output, err := node.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if output.Content == "not json" {
		t.Error("expected wrapped error object, got original content")
	}
}

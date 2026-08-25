package agent

import (
	"testing"
)

func TestAgentEventTypes(t *testing.T) {
	// 验证事件类型常量
	eventTypes := []EventType{
		EventAgentStart,
		EventAgentEnd,
		EventMessageUpdate,
		EventToolStart,
		EventToolEnd,
		EventToolPermissionRequest,
		EventToolPermissionResponse,
		EventError,
		EventProgress,
	}

	for _, eventType := range eventTypes {
		if eventType == "" {
			t.Errorf("EventType is empty")
		}
	}
}

func TestAgentRequest(t *testing.T) {
	req := &AgentRequest{
		RequestID: "test_request_123",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		Model:    "gpt-4",
		MaxTurns: 10,
		Timeout:  60,
	}

	if req.RequestID != "test_request_123" {
		t.Errorf("RequestID = %v, want test_request_123", req.RequestID)
	}

	if len(req.Messages) != 1 {
		t.Errorf("Messages length = %v, want 1", len(req.Messages))
	}

	if req.Model != "gpt-4" {
		t.Errorf("Model = %v, want gpt-4", req.Model)
	}
}

func TestToolDefinition(t *testing.T) {
	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type": "string",
				},
			},
		},
		IsReadOnly: true,
	}

	if tool.Name != "test_tool" {
		t.Errorf("Name = %v, want test_tool", tool.Name)
	}

	if tool.Description != "A test tool" {
		t.Errorf("Description = %v, want A test tool", tool.Description)
	}

	if !tool.IsReadOnly {
		t.Errorf("IsReadOnly = %v, want true", tool.IsReadOnly)
	}
}

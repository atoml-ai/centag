package billing

import (
	"context"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	s := NewService()
	if s == nil {
		t.Fatal("expected non-nil service")
	}
	s.Close()
}

func TestRecordEvent(t *testing.T) {
	s := NewService()
	defer s.Close()

	handler := NewMockHandler()
	s.RegisterHandler(handler)

	event := &Event{
		ID:        "test-1",
		Type:      "request",
		UserID:    1,
		TeamID:    "team-1",
		Backend:   "openai",
		Model:     "gpt-4",
		Tokens:    100,
		Timestamp: time.Now(),
	}

	s.RecordEvent(event)

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != "test-1" {
		t.Errorf("expected event ID test-1, got %s", events[0].ID)
	}
}

func TestMockHandler(t *testing.T) {
	handler := NewMockHandler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	event := &Event{
		ID:   "test-2",
		Type: "token_usage",
	}

	err := handler.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestNewRequestEvent(t *testing.T) {
	event := NewRequestEvent(1, "team-1", "openai", "gpt-4", 100)
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Type != "request" {
		t.Errorf("expected type request, got %s", event.Type)
	}
	if event.UserID != 1 {
		t.Errorf("expected user ID 1, got %d", event.UserID)
	}
}

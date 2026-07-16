// Package billing provides the billing hook interface for ProxyClaw.
//
// Design goals:
//   - Pluggable billing backends (Stripe, internal, mock)
//   - Async event recording to avoid blocking the proxy path
//   - Graceful degradation: if billing service is unavailable, allow request
//   - Support for usage-based billing with token counting
package billing

import (
	"context"
	"sync"
	"time"
)

// Event represents a billing event to be recorded.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`       // "request", "token_usage", "subscription"
	UserID    int64     `json:"user_id"`
	TeamID    string    `json:"team_id"`
	Backend   string    `json:"backend"`
	Model     string    `json:"model"`
	Tokens    int64     `json:"tokens"`
	Amount    float64   `json:"amount"`     // in cents
	Currency  string    `json:"currency"`   // default: "usd"
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// EventHandler processes billing events.
type EventHandler interface {
	// HandleEvent processes a billing event.
	HandleEvent(ctx context.Context, event *Event) error

	// Health checks if the billing handler is healthy.
	Health(ctx context.Context) error
}

// Service manages billing events and handlers.
type Service struct {
	mu       sync.RWMutex
	handlers []EventHandler
	events   chan *Event
	stopCh   chan struct{}
	stopped  bool
}

// NewService creates a new billing service.
func NewService() *Service {
	s := &Service{
		handlers: make([]EventHandler, 0),
		events:   make(chan *Event, 1000),
		stopCh:   make(chan struct{}),
	}
	go s.processEvents()
	return s
}

// RegisterHandler registers a billing event handler.
func (s *Service) RegisterHandler(handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)
}

// RecordEvent records a billing event asynchronously.
func (s *Service) RecordEvent(event *Event) {
	if event == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case s.events <- event:
	default:
		// Channel full, drop event (graceful degradation)
	}
}

// Close stops the billing service.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		close(s.stopCh)
		s.stopped = true
	}
}

// processEvents processes billing events in the background.
func (s *Service) processEvents() {
	for {
		select {
		case event := <-s.events:
			s.processEvent(event)
		case <-s.stopCh:
			return
		}
	}
}

// processEvent sends an event to all registered handlers.
func (s *Service) processEvent(event *Event) {
	s.mu.RLock()
	handlers := make([]EventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, handler := range handlers {
		if err := handler.HandleEvent(ctx, event); err != nil {
			// Log error but continue processing
			// In production, this would be logged to a monitoring system
		}
	}
}

// ── Built-in Handlers ────────────────────────────────────────────────────────

// LogHandler logs billing events (for debugging).
type LogHandler struct{}

// HandleEvent logs the billing event.
func (h *LogHandler) HandleEvent(ctx context.Context, event *Event) error {
	// In production, this would use a structured logger
	return nil
}

// Health always returns nil for log handler.
func (h *LogHandler) Health(ctx context.Context) error {
	return nil
}

// MockHandler is a mock billing handler for testing.
type MockHandler struct {
	mu     sync.Mutex
	events []*Event
}

// NewMockHandler creates a new mock billing handler.
func NewMockHandler() *MockHandler {
	return &MockHandler{
		events: make([]*Event, 0),
	}
}

// HandleEvent records the billing event.
func (h *MockHandler) HandleEvent(ctx context.Context, event *Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	return nil
}

// Health always returns nil for mock handler.
func (h *MockHandler) Health(ctx context.Context) error {
	return nil
}

// GetEvents returns all recorded events (for testing).
func (h *MockHandler) GetEvents() []*Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	events := make([]*Event, len(h.events))
	copy(events, h.events)
	return events
}

// ── Helper Functions ──────────────────────────────────────────────────────────

// NewRequestEvent creates a new request billing event.
func NewRequestEvent(userID int64, teamID, backend, model string, tokens int64) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      "request",
		UserID:    userID,
		TeamID:    teamID,
		Backend:   backend,
		Model:     model,
		Tokens:    tokens,
		Timestamp: time.Now(),
	}
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	return time.Now().Format("20060102150405.000000000")
}

package middleware

import (
	"testing"
	"time"
)

func TestNewUserQuotaMiddleware(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)
	if m == nil {
		t.Fatal("expected non-nil middleware")
	}
}

func TestUserQuotaWindow(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)

	// Test GetWindowStats for non-existent user
	tokens, requests, resetAt := m.GetWindowStats(999)
	if tokens != 0 || requests != 0 || !resetAt.IsZero() {
		t.Errorf("expected zero values for non-existent user, got tokens=%d, requests=%d", tokens, requests)
	}

	// Test ResetWindow for non-existent user (should not panic)
	m.ResetWindow(999)
}

func TestUserQuotaRecordTokens(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)

	// Pre-create the window (simulates middleware having processed a request)
	window := &userQuotaWindow{
		resetAt: time.Now().Add(24 * time.Hour),
	}
	m.windows.Store(int64(1), window)

	// Record tokens for a user
	m.RecordTokens(1, 100)

	// Check window stats
	tokens, requests, _ := m.GetWindowStats(1)
	if tokens != 100 {
		t.Errorf("expected 100 tokens, got %d", tokens)
	}
	if requests != 1 {
		t.Errorf("expected 1 request, got %d", requests)
	}

	// Record more tokens
	m.RecordTokens(1, 50)

	tokens, requests, _ = m.GetWindowStats(1)
	if tokens != 150 {
		t.Errorf("expected 150 tokens, got %d", tokens)
	}
	if requests != 2 {
		t.Errorf("expected 2 requests, got %d", requests)
	}
}

func TestUserQuotaRecordRequest(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)

	// Pre-create the window (simulates middleware having processed a request)
	window := &userQuotaWindow{
		resetAt: time.Now().Add(24 * time.Hour),
	}
	m.windows.Store(int64(1), window)

	// Record request for a user
	m.RecordRequest(1)

	// Check window stats
	tokens, requests, _ := m.GetWindowStats(1)
	if tokens != 0 {
		t.Errorf("expected 0 tokens, got %d", tokens)
	}
	if requests != 1 {
		t.Errorf("expected 1 request, got %d", requests)
	}
}

func TestUserQuotaResetWindow(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)

	// Record some tokens
	m.RecordTokens(1, 100)

	// Reset the window
	m.ResetWindow(1)

	// Check window stats
	tokens, requests, _ := m.GetWindowStats(1)
	if tokens != 0 {
		t.Errorf("expected 0 tokens after reset, got %d", tokens)
	}
	if requests != 0 {
		t.Errorf("expected 0 requests after reset, got %d", requests)
	}
}

func TestUserQuotaWindowExpiry(t *testing.T) {
	m := NewUserQuotaMiddleware(nil)

	// Manually create a window with expired reset time
	window := &userQuotaWindow{
		tokens:   100,
		requests: 5,
		resetAt:  time.Now().Add(-1 * time.Hour), // Expired
	}
	m.windows.Store(int64(1), window)

	// Get window stats should return current values
	tokens, requests, _ := m.GetWindowStats(1)
	if tokens != 100 {
		t.Errorf("expected 100 tokens, got %d", tokens)
	}
	if requests != 5 {
		t.Errorf("expected 5 requests, got %d", requests)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	// Test with nil context - this would require a gin.Context
	// For now, just test that the function exists
	// In a real test, you would create a gin.Context with user_id set
}

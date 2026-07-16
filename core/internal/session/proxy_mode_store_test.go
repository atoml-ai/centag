package session

import (
	"testing"
	"time"
)

func TestNewProxyModeStore(t *testing.T) {
	store := NewProxyModeStore()
	if store == nil {
		t.Fatal("NewProxyModeStore() returned nil")
	}
}

func TestStore_SetAndGet(t *testing.T) {
	store := NewProxyModeStore()
	
	session := SessionProxyMode{
		UserID:    "user123",
		ModeKey:   "#d",
		BackendID: "ollama-local",
		ModelName: "qwen2.5:7b",
		TTL:       3600,
	}
	
	err := store.Set("session1", session)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	
	retrieved, exists := store.Get("session1")
	if !exists {
		t.Fatal("Expected session to exist")
	}
	if retrieved.UserID != "user123" {
		t.Errorf("Expected UserID user123, got %s", retrieved.UserID)
	}
	if retrieved.ModeKey != "#d" {
		t.Errorf("Expected ModeKey #d, got %s", retrieved.ModeKey)
	}
}

func TestStore_Get_NonExistent(t *testing.T) {
	store := NewProxyModeStore()
	
	_, exists := store.Get("nonexistent")
	if exists {
		t.Error("Expected nonexistent session to not exist")
	}
}

func TestStore_Delete(t *testing.T) {
	store := NewProxyModeStore()
	
	session := SessionProxyMode{
		UserID:  "user123",
		ModeKey: "#d",
		TTL:     3600,
	}
	
	err := store.Set("session1", session)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	
	err = store.Delete("session1")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	
	_, exists := store.Get("session1")
	if exists {
		t.Error("Expected session to be deleted")
	}
}

func TestStore_Delete_NonExistent(t *testing.T) {
	store := NewProxyModeStore()
	
	err := store.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete() should not error on nonexistent key: %v", err)
	}
}

func TestStore_TTL_Expiration(t *testing.T) {
	store := NewProxyModeStore()
	
	session := SessionProxyMode{
		UserID:  "user123",
		ModeKey: "#d",
		TTL:     1, // 1 second
	}
	
	err := store.Set("session1", session)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	
	// Should exist immediately
	_, exists := store.Get("session1")
	if !exists {
		t.Error("Expected session to exist immediately after set")
	}
	
	// Wait for expiration
	time.Sleep(2 * time.Second)
	
	// Should be expired
	_, exists = store.Get("session1")
	if exists {
		t.Error("Expected session to be expired")
	}
}

func TestStore_TTL_NoExpiration(t *testing.T) {
	store := NewProxyModeStore()
	
	session := SessionProxyMode{
		UserID:  "user123",
		ModeKey: "#d",
		TTL:     3600, // 1 hour
	}
	
	err := store.Set("session1", session)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	
	// Should still exist after short delay
	time.Sleep(100 * time.Millisecond)
	
	_, exists := store.Get("session1")
	if !exists {
		t.Error("Expected session to still exist")
	}
}

func TestStore_Update(t *testing.T) {
	store := NewProxyModeStore()
	
	// Initial set
	session := SessionProxyMode{
		UserID:  "user123",
		ModeKey: "#d",
		TTL:     3600,
	}
	err := store.Set("session1", session)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	
	// Update
	session.ModeKey = "#s"
	session.BackendID = "openai-api"
	err = store.Set("session1", session)
	if err != nil {
		t.Fatalf("Set() update error: %v", err)
	}
	
	retrieved, exists := store.Get("session1")
	if !exists {
		t.Fatal("Expected session to exist after update")
	}
	if retrieved.ModeKey != "#s" {
		t.Errorf("Expected updated ModeKey #s, got %s", retrieved.ModeKey)
	}
	if retrieved.BackendID != "openai-api" {
		t.Errorf("Expected updated BackendID openai-api, got %s", retrieved.BackendID)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewProxyModeStore()
	done := make(chan bool, 10)
	
	// Concurrent reads and writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := "session" + string(rune('0'+id))
				session := SessionProxyMode{
					UserID:  "user" + string(rune('0'+id)),
					ModeKey: "#d",
					TTL:     3600,
				}
				store.Set(key, session)
				store.Get(key)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timeout")
		}
	}
}

func TestStore_Cleanup(t *testing.T) {
	store := NewProxyModeStore()
	
	// Set multiple sessions with short TTL
	for i := 0; i < 5; i++ {
		session := SessionProxyMode{
			UserID:  "user" + string(rune('0'+i)),
			ModeKey: "#d",
			TTL:     1,
		}
		store.Set("session"+string(rune('0'+i)), session)
	}
	
	// Wait for expiration
	time.Sleep(2 * time.Second)
	
	// Manually trigger cleanup
	store.Cleanup()
	
	// Verify all are cleaned up
	for i := 0; i < 5; i++ {
		_, exists := store.Get("session" + string(rune('0'+i)))
		if exists {
			t.Errorf("Expected session%d to be cleaned up", i)
		}
	}
}

func TestSessionProxyMode_IsExpired(t *testing.T) {
	// Not expired
	session := SessionProxyMode{
		UserID:    "user123",
		ModeKey:   "#d",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if session.IsExpired() {
		t.Error("Expected session to not be expired")
	}
	
	// Expired
	session.ExpiresAt = time.Now().Add(-1 * time.Hour)
	if !session.IsExpired() {
		t.Error("Expected session to be expired")
	}
}

func TestSessionProxyMode_CalculateExpiresAt(t *testing.T) {
	session := SessionProxyMode{
		UserID:  "user123",
		ModeKey: "#d",
		TTL:     3600,
	}
	
	session.CalculateExpiresAt()
	
	expectedMin := time.Now().Add(3599 * time.Second)
	expectedMax := time.Now().Add(3601 * time.Second)
	
	if session.ExpiresAt.Before(expectedMin) || session.ExpiresAt.After(expectedMax) {
		t.Errorf("Expected ExpiresAt to be around %v, got %v", 
			time.Now().Add(3600*time.Second), session.ExpiresAt)
	}
}

func TestStore_MultipleUsers(t *testing.T) {
	store := NewProxyModeStore()
	
	// Set sessions for multiple users
	users := []string{"user1", "user2", "user3"}
	for _, user := range users {
		session := SessionProxyMode{
			UserID:  user,
			ModeKey: "#d",
			TTL:     3600,
		}
		store.Set("session_"+user, session)
	}
	
	// Verify all exist
	for _, user := range users {
		retrieved, exists := store.Get("session_" + user)
		if !exists {
			t.Errorf("Expected session for %s to exist", user)
		}
		if retrieved.UserID != user {
			t.Errorf("Expected UserID %s, got %s", user, retrieved.UserID)
		}
	}
}

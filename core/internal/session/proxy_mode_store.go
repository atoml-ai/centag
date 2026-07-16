package session

import (
	"sync"
	"time"
)

// SessionProxyMode 会话代理模式设置
type SessionProxyMode struct {
	UserID    string    `json:"user_id"`
	ModeKey   string    `json:"mode"`
	BackendID string    `json:"backend,omitempty"`
	ModelName string    `json:"model,omitempty"`
	TTL       int       `json:"ttl"`           // 有效期（秒）
	ExpiresAt time.Time `json:"expires_at"`    // 过期时间
}

// IsExpired 检查是否已过期
func (s *SessionProxyMode) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// CalculateExpiresAt 计算过期时间
func (s *SessionProxyMode) CalculateExpiresAt() {
	s.ExpiresAt = time.Now().Add(time.Duration(s.TTL) * time.Second)
}

// ProxyModeStore 会话模式存储
type ProxyModeStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionProxyMode
}

// NewProxyModeStore 创建新的会话存储
func NewProxyModeStore() *ProxyModeStore {
	store := &ProxyModeStore{
		sessions: make(map[string]*SessionProxyMode),
	}
	// 启动后台清理协程
	go store.startCleanup()
	return store
}

// Set 设置会话模式
func (s *ProxyModeStore) Set(sessionID string, session SessionProxyMode) error {
	session.CalculateExpiresAt()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = &session
	return nil
}

// Get 获取会话模式
func (s *ProxyModeStore) Get(sessionID string) (*SessionProxyMode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, false
	}

	if session.IsExpired() {
		return nil, false
	}

	sessionCopy := *session
	return &sessionCopy, true
}

// Delete 删除会话模式
func (s *ProxyModeStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

// startCleanup 启动后台清理协程
func (s *ProxyModeStore) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.Cleanup()
	}
}

// Cleanup 清理过期的会话
func (s *ProxyModeStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

// Count 返回当前会话数量（用于测试）
func (s *ProxyModeStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

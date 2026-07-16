package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const maxSessionEvents = 500

// SessionEventEntry 是会话内 Pi 事件的只读快照（供 HTTP 轮询等使用）。
type SessionEventEntry struct {
	Seq     int64           `json:"seq"`
	Time    time.Time       `json:"time"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// AgentManager 管理多个 Pi 会话
type AgentManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	workDir  string
	config   *ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxSessions      int           `json:"maxSessions"`
	SessionTimeout   time.Duration `json:"sessionTimeout"`
	DefaultModel     string        `json:"defaultModel"`
	DefaultProvider  string        `json:"defaultProvider"`
	ThinkingLevel    string        `json:"thinkingLevel"`
	AutoCompaction   bool          `json:"autoCompaction"`
}

// Session 表示一个 Pi 会话
type Session struct {
	ID        string
	Client    *PiClient
	CreatedAt time.Time
	LastUsed  time.Time
	Context   context.Context
	Cancel    context.CancelFunc
	Metadata  map[string]interface{}

	eventMu  sync.Mutex
	eventSeq int64
	eventLog []SessionEventEntry
}

// DefaultManagerConfig 返回默认配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxSessions:    10,
		SessionTimeout: 30 * time.Minute,
		ThinkingLevel:  "medium",
		AutoCompaction: true,
	}
}

// NewAgentManager 创建新的管理器
func NewAgentManager(workDir string, config *ManagerConfig) *AgentManager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	
	return &AgentManager{
		sessions: make(map[string]*Session),
		workDir:  workDir,
		config:   config,
	}
}

func (s *Session) recordEvent(ev Event) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.eventSeq++
	ent := SessionEventEntry{
		Seq:     s.eventSeq,
		Time:    time.Now(),
		Type:    string(ev.Type),
		Payload: b,
	}
	s.eventLog = append(s.eventLog, ent)
	if len(s.eventLog) > maxSessionEvents {
		s.eventLog = s.eventLog[len(s.eventLog)-maxSessionEvents:]
	}
}

// EventsSince 返回 seq 大于 since 的最多 limit 条事件，以及本批最大 seq（无新事件时为 since）。
func (s *Session) EventsSince(since int64, limit int) ([]SessionEventEntry, int64) {
	if limit <= 0 || limit > maxSessionEvents {
		limit = 100
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	maxSeq := since
	var out []SessionEventEntry
	for _, e := range s.eventLog {
		if e.Seq <= since {
			continue
		}
		out = append(out, e)
		if e.Seq > maxSeq {
			maxSeq = e.Seq
		}
		if len(out) >= limit {
			break
		}
	}
	return out, maxSeq
}

// CreateSession 创建新会话
func (m *AgentManager) CreateSession(sessionID string, args ...string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 检查会话数量限制
	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("max sessions reached: %d", m.config.MaxSessions)
	}
	
	// 检查会话 ID 是否已存在
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session already exists: %s", sessionID)
	}
	
	// 创建 Pi 客户端
	client, err := NewPiClient(m.workDir, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建会话（先挂事件回调再 Connect，避免丢失早期事件）
	session := &Session{
		ID:        sessionID,
		Client:    client,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Context:   ctx,
		Cancel:    cancel,
		Metadata:  make(map[string]interface{}),
	}
	client.SetEventHandler(func(ev Event) {
		session.recordEvent(ev)
	})

	// 连接客户端
	if err := client.Connect(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect client: %w", err)
	}

	m.sessions[sessionID] = session

	// 配置会话
	if m.config.DefaultProvider != "" && m.config.DefaultModel != "" {
		client.SetModel(m.config.DefaultProvider, m.config.DefaultModel)
	}
	
	if m.config.ThinkingLevel != "" {
		client.SetThinkingLevel(m.config.ThinkingLevel)
	}
	
	client.SetAutoCompaction(m.config.AutoCompaction)
	
	return session, nil
}

// GetSession 获取会话
func (m *AgentManager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	
	session.LastUsed = time.Now()
	return session, nil
}

// CloseSession 关闭会话
func (m *AgentManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	session.Cancel()
	session.Client.Close()
	delete(m.sessions, sessionID)
	
	return nil
}

// ListSessions 列出所有会话
func (m *AgentManager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	
	return sessions
}

// CloseAll 关闭所有会话
func (m *AgentManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for id, session := range m.sessions {
		session.Cancel()
		session.Client.Close()
		delete(m.sessions, id)
	}
}

// CleanupIdleSessions 清理空闲会话
func (m *AgentManager) CleanupIdleSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for id, session := range m.sessions {
		if now.Sub(session.LastUsed) > m.config.SessionTimeout {
			session.Cancel()
			session.Client.Close()
			delete(m.sessions, id)
		}
	}
}

// StartCleanupRoutine 启动清理协程
func (m *AgentManager) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.CleanupIdleSessions()
			}
		}
	}()
}

// Workflow 工作流定义
type Workflow struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
}

// WorkflowStep 工作流步骤
type WorkflowStep struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	WaitNext bool   `json:"waitNext"`
	Timeout  int    `json:"timeout"`
}

// ExecuteWorkflow 执行工作流
func (m *AgentManager) ExecuteWorkflow(ctx context.Context, workflow *Workflow, sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	
	for i, step := range workflow.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		fmt.Printf("Executing step %d/%d: %s\n", i+1, len(workflow.Steps), step.Name)
		
		// 发送提示
		if err := session.Client.Prompt(step.Prompt); err != nil {
			return fmt.Errorf("step %s failed: %w", step.Name, err)
		}
		
		// 等待响应
		if step.WaitNext {
			if err := m.waitForCompletion(ctx, session, step.Timeout); err != nil {
				return fmt.Errorf("step %s timeout: %w", step.Name, err)
			}
		}
	}
	
	return nil
}

// waitForCompletion 等待完成
func (m *AgentManager) waitForCompletion(ctx context.Context, session *Session, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	
	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for completion")
		case event, ok := <-session.Client.Events():
			if !ok {
				return fmt.Errorf("event channel closed")
			}
			
			if event.Type == EventAgentEnd {
				return nil
			}
		}
	}
}

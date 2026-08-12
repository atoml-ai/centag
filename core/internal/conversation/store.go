package conversation

import (
	"context"
	"time"
)

// Session is a conversation container (one chat thread).
type Session struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	TenantID     string    `json:"tenant_id,omitempty"`
	Title        string    `json:"title,omitempty"`
	Category     string    `json:"category,omitempty"`
	PipelineID   string    `json:"pipeline_id,omitempty"`
	ProxyMode    string    `json:"proxy_mode,omitempty"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Message is one turn (user or assistant) inside a session.
type Message struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	Role         string    `json:"role"` // user | assistant | system
	Content      string    `json:"content"`
	RequestID    string    `json:"request_id,omitempty"`
	Model        string    `json:"model,omitempty"`
	Backend      string    `json:"backend,omitempty"`
	PipelineID   string    `json:"pipeline_id,omitempty"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	LatencyMs    int64     `json:"latency_ms,omitempty"`
	StatusCode   int       `json:"status_code,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListSessionsQuery filters session listing.
type ListSessionsQuery struct {
	UserID    int64
	TenantID  string
	Category  string
	Limit     int
	Offset    int
	Since     time.Time
	Until     time.Time
}

// PageQuery pages messages within a session.
type PageQuery struct {
	Limit  int
	Offset int
}

// DeleteSessionsQuery selects sessions to delete.
// IDs takes precedence; otherwise the filter fields (same as ListSessionsQuery) select matches.
type DeleteSessionsQuery struct {
	IDs      []string
	UserID   int64
	TenantID string
	Category string
	Since    time.Time
	Until    time.Time
}

// DeleteMessagesQuery selects messages to delete within one session.
// IDs takes precedence; otherwise Role matches messages of that role.
type DeleteMessagesQuery struct {
	SessionID string
	IDs       []string
	Role      string
}

// Store is the unified conversation persistence API.
// Implementations: FileStore (minimal), SQLiteStore (personal), PostgresStore (team).
type Store interface {
	EnsureSession(ctx context.Context, s *Session) (*Session, error)
	AppendMessage(ctx context.Context, m *Message) error
	ListSessions(ctx context.Context, q ListSessionsQuery) ([]*Session, error)
	GetSession(ctx context.Context, id string) (*Session, error)
	ListMessages(ctx context.Context, sessionID string, q PageQuery) ([]*Message, error)
	ListCategories(ctx context.Context, userID int64, tenantID string) ([]string, error)
	// DeleteSessions removes matched sessions and their messages, returning how many were deleted.
	DeleteSessions(ctx context.Context, q DeleteSessionsQuery) (int64, error)
	// DeleteMessages removes matched messages within a session, returning how many were deleted.
	DeleteMessages(ctx context.Context, q DeleteMessagesQuery) (int64, error)
}

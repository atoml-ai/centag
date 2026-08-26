package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"centag/core/internal/agent"
	"centag/core/internal/agent/skills"
	"centag/core/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// setupAgentSessionsDB 建立 040 迁移同构的内存库。
func setupAgentSessionsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	schema := `
	CREATE TABLE IF NOT EXISTS agent_sessions (
	    id VARCHAR(64) PRIMARY KEY,
	    user_id BIGINT NOT NULL DEFAULT 0,
	    tenant_id VARCHAR(255) DEFAULT '',
	    title TEXT DEFAULT '',
	    skill VARCHAR(128) DEFAULT '',
	    backend_id VARCHAR(255) DEFAULT '',
	    model VARCHAR(255) DEFAULT '',
	    status VARCHAR(32) DEFAULT 'active',
	    message_count INTEGER DEFAULT 0,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS agent_messages (
	    id VARCHAR(64) PRIMARY KEY,
	    session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id),
	    role VARCHAR(32) NOT NULL,
	    content TEXT DEFAULT '',
	    skill VARCHAR(128) DEFAULT '',
	    tool_name VARCHAR(255) DEFAULT '',
	    tool_params TEXT,
	    tool_result TEXT,
	    is_error BOOLEAN DEFAULT FALSE,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestAgentSession(userID int64) *AgentSession {
	now := time.Now()
	return &AgentSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		TenantID:  "",
		Title:     "s",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestAgentSessionStoreRoundtrip(t *testing.T) {
	store := newAgentSessionStore(setupAgentSessionsDB(t), "sqlite")
	ctx := context.Background()

	sess := newTestAgentSession(42)
	sess.Skill = "status-check"
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.Get(ctx, sess.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v (%v)", got, err)
	}
	if got.UserID != 42 || got.Skill != "status-check" {
		t.Fatalf("unexpected session: %+v", got)
	}

	if err := store.AppendMessage(ctx, &AgentMessage{ID: uuid.New().String(), SessionID: sess.ID, Role: "user", Content: "hi", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("append message: %v", err)
	}
	if err := store.AppendMessage(ctx, &AgentMessage{
		ID: uuid.New().String(), SessionID: sess.ID, Role: "tool", Skill: "status-check",
		ToolName: "read_database", ToolParams: `{"table":"agent_sessions"}`,
		ToolResult: `{"content":"2 rows","is_error":false}`, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("append tool message: %v", err)
	}
	if err := store.AppendMessage(ctx, &AgentMessage{ID: uuid.New().String(), SessionID: sess.ID, Role: "assistant", Content: "ok", Skill: "status-check", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	msgs, ok, err := store.ListMessages(ctx, sess.ID)
	if err != nil || !ok || len(msgs) != 3 {
		t.Fatalf("list messages: ok=%v n=%d err=%v", ok, len(msgs), err)
	}
	if msgs[1].ToolName != "read_database" || msgs[1].ToolParams != `{"table":"agent_sessions"}` || msgs[1].ToolResult != `{"content":"2 rows","is_error":false}` {
		t.Fatalf("tool message details lost: %+v", msgs[1])
	}

	if err := store.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := store.Get(ctx, sess.ID); got != nil {
		t.Fatal("session must be gone after delete")
	}
	if _, ok, _ := store.ListMessages(ctx, sess.ID); ok {
		t.Fatal("messages must be gone after delete")
	}
}

func ginCtxWithUser(userID int64, role string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if userID > 0 {
		c.Set(auth.CtxKeyUserID, userID)
	}
	if role != "" {
		c.Set(auth.CtxKeyRole, role)
	}
	return c
}

func TestAgentSessionVisible(t *testing.T) {
	owner := newTestAgentSession(7)
	foreign := newTestAgentSession(8)
	shared := newTestAgentSession(0)

	adminCtx := ginCtxWithUser(1, auth.RoleAdmin)
	ownerCtx := ginCtxWithUser(7, auth.RoleNormal)
	strangerCtx := ginCtxWithUser(9, auth.RoleNormal)
	anonCtx := ginCtxWithUser(0, "")

	if !agentSessionVisible(adminCtx, foreign) {
		t.Error("admin must see all sessions")
	}
	if !agentSessionVisible(ownerCtx, owner) {
		t.Error("owner must see own session")
	}
	if agentSessionVisible(strangerCtx, owner) {
		t.Error("stranger must NOT see another user's session")
	}
	if !agentSessionVisible(strangerCtx, shared) {
		t.Error("shared (user_id=0) sessions stay visible to authenticated users")
	}
	if agentSessionVisible(anonCtx, owner) {
		t.Error("unauthenticated context must not see any session")
	}
	if agentSessionVisible(ownerCtx, nil) {
		t.Error("nil session must be invisible")
	}
}

// TestAgentHandlerOwnershipEnforcement 走 HTTP 层验证用户 A 无法读取/删除/向
// 用户 B 的会话发消息，且 List 只返回本人会话，管理员可见全部。
func TestAgentHandlerOwnershipEnforcement(t *testing.T) {
	store := newAgentSessionStore(setupAgentSessionsDB(t), "sqlite")
	h := &BuiltinAgentHandler{
		store:         store,
		skillRegistry: skills.NewSkillRegistry(),
		config:        &agent.AgentConfig{Skills: agent.SkillsConfig{}},
	}

	sessA := newTestAgentSession(7)
	sessB := newTestAgentSession(8)
	ctx := context.Background()
	for _, s := range []*AgentSession{sessA, sessB} {
		if err := store.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	// 用户 A 列表只看到自己（+共享）
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(auth.CtxKeyUserID, int64(7))
	c.Set(auth.CtxKeyRole, auth.RoleNormal)
	h.ListSessions(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, sessA.ID) || strings.Contains(body, sessB.ID) {
		t.Fatalf("user A list leaked user B session:\n%s", body)
	}

	// 用户 A 访问用户 B 的会话：Get / Delete / Messages 全部 404
	for _, tc := range []struct {
		name   string
		call   func(*gin.Context)
		method string
		path   string
	}{
		{"get", h.GetSession, http.MethodGet, "/" + sessB.ID},
		{"delete", h.DeleteSession, http.MethodDelete, "/" + sessB.ID},
		{"messages", h.ListMessages, http.MethodGet, "/" + sessB.ID + "/messages"},
	} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(tc.method, "/api/v1/builtin-agent/sessions"+tc.path, nil)
		c.Params = gin.Params{{Key: "id", Value: sessB.ID}}
		c.Set(auth.CtxKeyUserID, int64(7))
		c.Set(auth.CtxKeyRole, auth.RoleNormal)
		tc.call(c)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s foreign session: status=%d want 404", tc.name, rec.Code)
		}
	}

	// SendMessage 归属校验（engine 未初始化也应先命中 404）
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/builtin-agent/sessions/"+sessB.ID+"/messages", nil)
	c.Params = gin.Params{{Key: "id", Value: sessB.ID}}
	c.Set(auth.CtxKeyUserID, int64(7))
	c.Set(auth.CtxKeyRole, auth.RoleNormal)
	h.SendMessage(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("send message to foreign session: status=%d want 404", rec.Code)
	}

	// 管理员可见全部
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(auth.CtxKeyUserID, int64(1))
	c.Set(auth.CtxKeyRole, auth.RoleAdmin)
	h.ListSessions(c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), sessA.ID) || !strings.Contains(rec.Body.String(), sessB.ID) {
		t.Fatalf("admin list must contain all sessions:\n%s", rec.Body.String())
	}

	// 用户 B 的会话未被用户 A 被拒的删除请求波及
	if got, _ := store.Get(ctx, sessB.ID); got == nil {
		t.Fatal("user B session must survive user A's denied delete")
	}
}

package server

import (
	"database/sql"
	"sync"
	"testing"

	"centag/core/internal/agent"

	_ "modernc.org/sqlite"
)

// newAgentHandlerForTest 构造最小可用的 BuiltinAgentHandler（内存库，无外部依赖）。
func newAgentHandlerForTest(t *testing.T) *BuiltinAgentHandler {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	h := NewBuiltinAgentHandler(
		&agent.AgentConfig{}, t.TempDir(), db, "sqlite", nil,
		"http://127.0.0.1:1", "", nil, nil, "", "",
	)
	return h
}

// TestEngineForPerSessionIsolation 是 P0-2 的核心回归：不同会话必须持有
// 不同的引擎实例，EnsureBackend/RefreshToken 的凭据互不串扰。
func TestEngineForPerSessionIsolation(t *testing.T) {
	h := newAgentHandlerForTest(t)

	s1 := &AgentSession{ID: "sess-1"}
	s2 := &AgentSession{ID: "sess-2"}

	e1 := h.engineFor(s1)
	e2 := h.engineFor(s2)
	if e1 == e2 {
		t.Fatal("distinct sessions must not share a RuntimeEngine")
	}

	// 两会话分别应用各自的凭据/路由状态
	e1.EnsureBackend(agent.AgentEngineOptions{BaseURL: "http://a", Token: "jwt-alice", BackendID: "bA", PipelineID: "pA", SessionID: "sess-1"})
	e2.EnsureBackend(agent.AgentEngineOptions{BaseURL: "http://b", Token: "jwt-bob", BackendID: "bB", PipelineID: "pB", SessionID: "sess-2"})
	e2.RefreshToken("jwt-bob-rotated")

	snap1 := e1.BackendSnapshot()
	snap2 := e2.BackendSnapshot()
	if snap1.Token != "jwt-alice" || snap1.BackendID != "bA" || snap1.PipelineID != "pA" {
		t.Fatalf("session-1 credentials polluted: %+v", snap1)
	}
	if snap2.Token != "jwt-bob-rotated" || snap2.BackendID != "bB" || snap2.PipelineID != "pB" {
		t.Fatalf("session-2 credentials polluted: %+v", snap2)
	}

	// 同一会话重复获取返回同一实例（缓存语义）
	if again := h.engineFor(s1); again != e1 {
		t.Fatal("engineFor must return the cached engine for the same session")
	}
}

// TestEngineForConcurrentSessionsNoRace 并发为多个会话获取/使用引擎，
// 在 -race 下验证映射与引擎内部状态的并发安全。
func TestEngineForConcurrentSessionsNoRace(t *testing.T) {
	h := newAgentHandlerForTest(t)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sess := &AgentSession{ID: "sess-concurrent"}
			for j := 0; j < 50; j++ {
				e := h.engineFor(sess)
				opts := agent.AgentEngineOptions{
					BaseURL:    "http://127.0.0.1:1",
					Token:      "jwt",
					BackendID:  "backend",
					PipelineID: "pipeline",
					SessionID:  sess.ID,
				}
				e.EnsureBackend(opts)
				e.RefreshToken(opts.Token)
				_ = e.BackendSnapshot()
			}
		}(i)
	}
	wg.Wait()

	if got := len(h.engines); got != 1 {
		t.Fatalf("expected exactly one cached engine, got %d", got)
	}
}

// TestDropCoreReleasesEngine 会话删除时同时释放 Agent 句柄与会话引擎。
func TestDropCoreReleasesEngine(t *testing.T) {
	h := newAgentHandlerForTest(t)
	h.engineFor(&AgentSession{ID: "sess-gone"})
	h.dropCore("sess-gone")
	if _, ok := h.engines["sess-gone"]; ok {
		t.Fatal("dropCore must release the session engine")
	}
}

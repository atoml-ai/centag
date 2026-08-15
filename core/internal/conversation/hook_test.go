package conversation

import (
	"context"
	"testing"
	"time"

	"centag/core/pkg/hooks"
	"centag/core/pkg/types"
)

// helper to send request + response through hook and collect messages.
func roundTrip(t *testing.T, hm *hooks.DefaultHookManager, store Store, sessionID, requestID, userContent, assistantContent string, extraMeta map[string]interface{}) []*Message {
	t.Helper()
	ctx := context.Background()

	existing, _ := store.ListMessages(ctx, sessionID, PageQuery{})
	expected := len(existing) + 2

	meta := map[string]interface{}{
		"session_id":   sessionID,
		"user_id":      int64(9),
		"category":     "code",
		"user_content": userContent,
		"request_id":   requestID,
	}
	for k, v := range extraMeta {
		meta[k] = v
	}

	req := &types.UnifiedRequest{
		Model:    "m",
		Metadata: meta,
	}
	if err := hm.TriggerRequestHooks(ctx, req); err != nil {
		t.Fatal(err)
	}

	resp := &types.UnifiedResponse{
		Content: assistantContent,
		Model:   "m",
		Metadata: map[string]interface{}{
			"session_id": sessionID,
			"request_id": requestID,
			"backend":    "b1",
		},
	}
	if err := hm.TriggerResponseHooks(ctx, resp); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs, err := store.ListMessages(ctx, sessionID, PageQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) >= expected {
			return msgs
		}
		time.Sleep(20 * time.Millisecond)
	}

	msgs, _ := store.ListMessages(ctx, sessionID, PageQuery{})
	return msgs
}

func TestLoggingHook_RequestResponseRoundTrip(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	msgs := roundTrip(t, hm, store, "sess-1", "r1", "how to sort?", "use sort.Slice", nil)

	if len(msgs) < 2 {
		t.Fatalf("expected user+assistant messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("roles = %s/%s", msgs[0].Role, msgs[1].Role)
	}
}

func TestLoggingHook_SkipAuxiliaryRequest(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	msgs := roundTrip(t, hm, store, "sess-aux", "r_aux", "你是使用的什么大模型", "应该被跳过的回复", map[string]interface{}{
		"is_auxiliary": "true",
	})

	if len(msgs) >= 2 {
		t.Fatalf("auxiliary request should be skipped, got %d messages", len(msgs))
	}
}

func TestLoggingHook_DedupSameContentWithinWindow(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	sessionID := "sess-dedup"
	content := "你是使用的什么大模型"
	reply := "正常回复"

	// first request — should be recorded.
	msgs1 := roundTrip(t, hm, store, sessionID, "r_first", content, reply, nil)
	if len(msgs1) < 2 {
		t.Fatalf("first request should produce 2 messages, got %d", len(msgs1))
	}

	// second request with identical content AND identical reply within dedupWindow
	// — 视为客户端短期重试，整轮跳过。
	time.Sleep(50 * time.Millisecond) // tiny gap, well within dedupWindow
	msgs2 := roundTrip(t, hm, store, sessionID, "r_dup", content, reply, nil)
	if len(msgs2) > len(msgs1) {
		t.Fatalf("retry with same content+reply within window should be skipped, got %d messages (was %d)", len(msgs2), len(msgs1))
	}
}

func TestLoggingHook_AgentTurns_UserOnceAssistantPerTurn(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	sessionID := "sess-agent"
	content := "执行系统状态检查" // agent 多轮推理每轮复用同一任务输入

	// 首轮：记录 user + assistant。
	msgs1 := roundTrip(t, hm, store, sessionID, "r_turn1", content, "第1轮：调用工具读取日志", nil)
	if len(msgs1) < 2 {
		t.Fatalf("first turn should produce 2 messages, got %d", len(msgs1))
	}

	// 后续轮：同内容、不同回复 —— user 不重复记录，assistant 每轮保留。
	msgs2 := roundTrip(t, hm, store, sessionID, "r_turn2", content, "第2轮：继续分析", nil)
	if len(msgs2) != len(msgs1)+1 {
		t.Fatalf("second turn should append only assistant (1), got +%d messages", len(msgs2)-len(msgs1))
	}
	if last := msgs2[len(msgs2)-1]; last.Role != "assistant" || last.Content != "第2轮：继续分析" {
		t.Fatalf("last message = %s/%q, want assistant/第2轮：继续分析", last.Role, last.Content)
	}

	// 第三轮：依旧只追加 assistant。
	msgs3 := roundTrip(t, hm, store, sessionID, "r_turn3", content, "第3轮：输出最终报告", nil)
	if len(msgs3) != len(msgs2)+1 {
		t.Fatalf("third turn should append only assistant (1), got +%d messages", len(msgs3)-len(msgs2))
	}
	userCount := 0
	for _, m := range msgs3 {
		if m.Role == "user" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("user message should be recorded once, got %d", userCount)
	}
}

func TestLoggingHook_AllowDifferentContent(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	sessionID := "sess-diff"

	msgs1 := roundTrip(t, hm, store, sessionID, "r_a", "问题A", "回答A", nil)
	if len(msgs1) < 2 {
		t.Fatalf("first request, got %d", len(msgs1))
	}

	msgs2 := roundTrip(t, hm, store, sessionID, "r_b", "问题B", "回答B", nil)
	if len(msgs2) <= len(msgs1) {
		t.Fatalf("different content should both record, got %d (was %d)", len(msgs2), len(msgs1))
	}
}

func TestLoggingHook_AllowSameContentDifferentSession(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	content := "相同的用户问题"

	msgs1 := roundTrip(t, hm, store, "sess-a", "r_1", content, "回答1", nil)
	if len(msgs1) < 2 {
		t.Fatalf("session A, got %d", len(msgs1))
	}

	msgs2 := roundTrip(t, hm, store, "sess-b", "r_2", content, "回答2", nil)
	if len(msgs2) < 2 {
		t.Fatalf("different session with same content should both record, got %d", len(msgs2))
	}
}

func TestConversationHandler_UserIsolation(t *testing.T) {
	// covered via canAccessSession logic in handler tests (server package)
}

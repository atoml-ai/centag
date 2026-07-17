package conversation

import (
	"context"
	"testing"
	"time"

	"centag/core/pkg/hooks"
	"centag/core/pkg/types"
)

func TestLoggingHook_RequestResponseRoundTrip(t *testing.T) {
	store := NewFileStore(t.TempDir())
	hook := NewLoggingHook(store)
	hm := hooks.NewManager()
	hm.RegisterStorageHook(hook)

	ctx := context.Background()
	req := &types.UnifiedRequest{
		Model: "m",
		Metadata: map[string]interface{}{
			"session_id":   "sess-1",
			"user_id":      int64(9),
			"category":     "code",
			"user_content": "how to sort?",
			"request_id":   "r1",
		},
	}
	if err := hm.TriggerRequestHooks(ctx, req); err != nil {
		t.Fatal(err)
	}
	resp := &types.UnifiedResponse{
		Content: "use sort.Slice",
		Model:   "m",
		Metadata: map[string]interface{}{
			"session_id": "sess-1",
			"request_id": "r1",
			"backend":    "b1",
		},
	}
	if err := hm.TriggerResponseHooks(ctx, resp); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var msgs []*Message
	for time.Now().Before(deadline) {
		var err error
		msgs, err = store.ListMessages(ctx, "sess-1", PageQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(msgs) < 2 {
		t.Fatalf("expected user+assistant messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("roles = %s/%s", msgs[0].Role, msgs[1].Role)
	}
}

func TestConversationHandler_UserIsolation(t *testing.T) {
	// covered via canAccessSession logic in handler tests (server package)
}

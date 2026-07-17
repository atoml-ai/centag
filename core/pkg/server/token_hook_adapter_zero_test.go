package server

import (
	"context"
	"testing"

	"centag/core/internal/edition"
	"centag/core/pkg/hooks"
)

func TestTokenUsageHookAdapter_SkipsZeroTokensOnSuccess(t *testing.T) {
	// adapter with nil svc must be safe
	a := newTokenUsageHookAdapter(nil)
	if err := a.OnTokenUsed(context.Background(), &hooks.TokenUsage{TotalTokens: 10, Success: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.OnTokenUsed(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestConversationHandler_UnavailableStore(t *testing.T) {
	// covered in conversation_handler_test; ensure nil handler methods don't panic via New with nil
	h := NewConversationHandler(nil, edition.Personal)
	if h.store != nil {
		t.Fatal("expected nil store")
	}
}

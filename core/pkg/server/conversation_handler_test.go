package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"centag/core/internal/auth"
	"centag/core/internal/conversation"
	"centag/core/internal/edition"
)

func TestConversationHandler_ListAndMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := conversation.NewFileStore(t.TempDir())
	ctx := context.Background()
	sess, err := store.EnsureSession(ctx, &conversation.Session{ID: "s1", UserID: 1, Category: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	_ = store.AppendMessage(ctx, &conversation.Message{SessionID: sess.ID, Role: "user", Content: "hi"})
	_ = store.AppendMessage(ctx, &conversation.Message{SessionID: sess.ID, Role: "assistant", Content: "hello"})

	h := NewConversationHandler(store, edition.Personal)
	r := gin.New()
	r.GET("/sessions", func(c *gin.Context) {
		c.Set(auth.CtxKeyUserID, int64(1))
		h.ListSessions(c)
	})
	r.GET("/sessions/:id/messages", func(c *gin.Context) {
		c.Set(auth.CtxKeyUserID, int64(1))
		h.ListMessages(c)
	})
	r.GET("/categories", func(c *gin.Context) {
		c.Set(auth.CtxKeyUserID, int64(1))
		h.ListCategories(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if w.Code != 200 {
		t.Fatalf("list sessions status=%d body=%s", w.Code, w.Body.String())
	}
	var listResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp["count"].(float64) < 1 {
		t.Fatalf("expected sessions, got %v", listResp)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/sessions/s1/messages", nil))
	if w2.Code != 200 {
		t.Fatalf("messages status=%d body=%s", w2.Code, w2.Body.String())
	}

	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/categories", nil))
	if w3.Code != 200 {
		t.Fatalf("categories status=%d", w3.Code)
	}
}

func TestConversationHandler_TeamForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := conversation.NewFileStore(t.TempDir())
	ctx := context.Background()
	_, _ = store.EnsureSession(ctx, &conversation.Session{ID: "s2", UserID: 2, Category: "chat"})

	h := NewConversationHandler(store, edition.Team)
	r := gin.New()
	r.GET("/sessions/:id", func(c *gin.Context) {
		c.Set(auth.CtxKeyUserID, int64(1))
		h.GetSession(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/s2", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestConversationHandler_NotFoundAndUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := conversation.NewFileStore(t.TempDir())
	h := NewConversationHandler(store, edition.Personal)
	r := gin.New()
	r.GET("/sessions/:id", func(c *gin.Context) {
		c.Set(auth.CtxKeyUserID, int64(1))
		h.GetSession(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sessions/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	hNil := NewConversationHandler(nil, edition.Personal)
	r2 := gin.New()
	r2.GET("/sessions", func(c *gin.Context) { hNil.ListSessions(c) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if w2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w2.Code)
	}
}

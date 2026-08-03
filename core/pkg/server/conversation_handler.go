package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"centag/core/internal/auth"
	"centag/core/internal/conversation"
	"centag/core/internal/edition"
)

// ConversationHandler serves conversation browse APIs.
type ConversationHandler struct {
	store   conversation.Store
	edition edition.Edition
}

// NewConversationHandler creates a handler.
func NewConversationHandler(store conversation.Store, ed edition.Edition) *ConversationHandler {
	return &ConversationHandler{store: store, edition: ed}
}

// ListSessions GET /api/v1/conversations/sessions
func (h *ConversationHandler) ListSessions(c *gin.Context) {
	if h == nil || h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "conversation store unavailable"})
		return
	}
	userID, _ := auth.GetUserID(c)
	q := conversation.ListSessionsQuery{
		UserID:   userID,
		Category: strings.TrimSpace(c.Query("category")),
		Limit:    queryInt(c, "limit", 50),
		Offset:   queryInt(c, "offset", 0),
	}
	if h.edition.IsTeam() {
		// 组模型（036）：会话按 user_id 隔离，租户字段不再作为作用域。
		// team 用户仅看自己的会话，管理员可通过 ?user_id= 查询指定用户。
		if auth.IsAdmin(c) {
			if raw := c.Query("user_id"); raw != "" {
				if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
					q.UserID = id
				}
			}
		}
	}
	if since := c.Query("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q.Since = t
		}
	}
	if until := c.Query("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			q.Until = t
		}
	}
	list, err := h.store.ListSessions(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*conversation.Session{}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": list, "count": len(list)})
}

// GetSession GET /api/v1/conversations/sessions/:id
func (h *ConversationHandler) GetSession(c *gin.Context) {
	if h == nil || h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "conversation store unavailable"})
		return
	}
	id := c.Param("id")
	sess, err := h.store.GetSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if !h.canAccessSession(c, sess) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	c.JSON(http.StatusOK, sess)
}

// ListMessages GET /api/v1/conversations/sessions/:id/messages
func (h *ConversationHandler) ListMessages(c *gin.Context) {
	if h == nil || h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "conversation store unavailable"})
		return
	}
	id := c.Param("id")
	sess, err := h.store.GetSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if !h.canAccessSession(c, sess) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	msgs, err := h.store.ListMessages(c.Request.Context(), id, conversation.PageQuery{
		Limit:  queryInt(c, "limit", 100),
		Offset: queryInt(c, "offset", 0),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if msgs == nil {
		msgs = []*conversation.Message{}
	}
	// 历史记录可能存了完整上游 SSE；浏览时规范化为可读文本（不改库）
	for _, m := range msgs {
		if m == nil || m.Role != "assistant" {
			continue
		}
		m.Content = conversation.NormalizeAssistantContent(m.Content)
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs, "count": len(msgs)})
}

// ListCategories GET /api/v1/conversations/categories
func (h *ConversationHandler) ListCategories(c *gin.Context) {
	if h == nil || h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "conversation store unavailable"})
		return
	}
	userID, _ := auth.GetUserID(c)
	// 组模型（036）：租户不再作为作用域，会话按 user_id 归类。
	tenantID := ""
	cats, err := h.store.ListCategories(c.Request.Context(), userID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cats == nil {
		cats = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

func (h *ConversationHandler) canAccessSession(c *gin.Context, sess *conversation.Session) bool {
	if sess == nil {
		return false
	}
	if !h.edition.IsTeam() {
		return true
	}
	if auth.IsAdmin(c) {
		return true
	}
	userID, err := auth.GetUserID(c)
	if err != nil {
		return false
	}
	return sess.UserID == 0 || sess.UserID == userID
}

func queryInt(c *gin.Context, key string, def int) int {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return n
}

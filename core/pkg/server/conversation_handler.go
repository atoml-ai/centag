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

// deleteSessionsReq is the body for batch session deletion.
// ids（多选）优先；否则按 all + 筛选条件（单选/全选/条件筛选后删除）。
type deleteSessionsReq struct {
	IDs      []string `json:"ids"`
	All      bool     `json:"all"`
	Category string   `json:"category"`
	UserID   int64    `json:"user_id"`
	Since    string   `json:"since"`
	Until    string   `json:"until"`
}

// DeleteSession DELETE /api/v1/conversations/sessions/:id
// 删除单个会话及其全部消息（单选）。
func (h *ConversationHandler) DeleteSession(c *gin.Context) {
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
	n, err := h.store.DeleteSessions(c.Request.Context(), conversation.DeleteSessionsQuery{IDs: []string{id}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// DeleteSessions POST /api/v1/conversations/sessions/delete
// 批量删除会话：ids 多选；all + category/since/until 全选或条件筛选后删除。
func (h *ConversationHandler) DeleteSessions(c *gin.Context) {
	if h == nil || h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "conversation store unavailable"})
		return
	}
	var body deleteSessionsReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	q := conversation.DeleteSessionsQuery{IDs: body.IDs}
	if len(body.IDs) == 0 {
		q.Category = body.Category
		if h.edition.IsTeam() {
			if auth.IsAdmin(c) {
				q.UserID = body.UserID
			} else {
				userID, err := auth.GetUserID(c)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
					return
				}
				q.UserID = userID
			}
		}
		if t, err := parseTime(body.Since); err == nil {
			q.Since = t
		}
		if t, err := parseTime(body.Until); err == nil {
			q.Until = t
		}
		// 既无多选也无筛选条件时，必须显式 all 才允许全量删除（防误操作）。
		if !body.All && len(q.IDs) == 0 && q.Category == "" && q.Since.IsZero() && q.Until.IsZero() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to delete: provide ids, filters, or all=true"})
			return
		}
	} else if h.edition.IsTeam() && !auth.IsAdmin(c) {
		// 组模型：非管理员多选删除时校验每个会话归属，防止越权删除他人会话。
		userID, err := auth.GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		for _, id := range body.IDs {
			sess, err := h.store.GetSession(c.Request.Context(), id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if sess == nil {
				continue
			}
			if !(sess.UserID == 0 || sess.UserID == userID) {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	n, err := h.store.DeleteSessions(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// deleteMessagesReq is the body for message deletion within a session.
// ids（多选）优先；否则 role 删除该角色全部消息。
type deleteMessagesReq struct {
	IDs  []string `json:"ids"`
	Role string   `json:"role"`
}

// DeleteMessages POST /api/v1/conversations/sessions/:id/messages/delete
// 删除会话内消息：ids 单选/多选，或按 role 条件筛选删除。
func (h *ConversationHandler) DeleteMessages(c *gin.Context) {
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
	var body deleteMessagesReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if len(body.IDs) == 0 && body.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to delete: provide ids or role"})
		return
	}
	n, err := h.store.DeleteMessages(c.Request.Context(), conversation.DeleteMessagesQuery{
		SessionID: id,
		IDs:       body.IDs,
		Role:      body.Role,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
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

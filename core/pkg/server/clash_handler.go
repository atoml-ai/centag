package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"centag/core/internal/auth"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ClashHandler 处理 Clash 订阅规则的 CRUD 接口
type ClashHandler struct {
	server *Server // reference to server for remote config access
}

func NewClashHandler(server *Server) *ClashHandler {
	return &ClashHandler{server: server}
}

// readDefaultRule reads clash rules from remote configsync.
func (h *ClashHandler) readDefaultRule() (string, error) {
	// Remote configsync (clash.rules config_key)
	if h.server != nil {
		if rules := h.server.GetClashRules(); rules != "" {
			logger.Infof("clash: using remote rules from configsync")
			return rules, nil
		}
	}
	return "", fmt.Errorf("no clash rules configured (remote configsync not available)")
}

// generateToken 生成 32 字节随机十六进制令牌
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// subscribeURL 根据请求拼接订阅 URL
func subscribeURL(c *gin.Context, token string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return fmt.Sprintf("%s://%s/clash/subscribe/%s", scheme, c.Request.Host, token)
}

// ruleResponse 对外返回的规则视图（附带订阅 URL）
type ruleResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	RuleContent    string `json:"rule_content"`
	HasCustomRule  bool   `json:"has_custom_rule"`
	SubscribeToken string `json:"subscribe_token"`
	SubscribeURL   string `json:"subscribe_url"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func toRuleResponse(c *gin.Context, r *database.ClashRule) ruleResponse {
	return ruleResponse{
		ID:             r.ID,
		Name:           r.Name,
		RuleContent:    r.RuleContent,
		HasCustomRule:  r.RuleContent != "",
		SubscribeToken: r.SubscribeToken,
		SubscribeURL:   subscribeURL(c, r.SubscribeToken),
		CreatedAt:      r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ownerCheck 验证规则属于当前用户，返回规则或写入错误响应
func ownerCheck(c *gin.Context, userID int64, ruleID int64) (*database.ClashRule, bool) {
	ctx := c.Request.Context()
	rule, err := database.Get().ClashRuleStore().GetByID(ctx, ruleID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondNotFound(c, "rule not found")
			return nil, false
		}
		logger.Errorf("clash ownerCheck: %v", err)
		RespondInternalError(c, "failed to get rule")
		return nil, false
	}
	if rule.UserID != userID {
		RespondError(c, http.StatusForbidden, "access denied")
		return nil, false
	}
	return rule, true
}

// ── 列表 ─────────────────────────────────────────────────────────────────────

// ListRules 获取当前用户的所有规则
//
// GET /api/v1/user/clash/rules
func (h *ClashHandler) ListRules(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	rules, err := database.Get().ClashRuleStore().ListByUserID(c.Request.Context(), userID)
	if err != nil {
		logger.Errorf("ListRules: %v", err)
		RespondInternalError(c, "failed to list rules")
		return
	}

	resp := make([]ruleResponse, 0, len(rules))
	for _, r := range rules {
		resp = append(resp, toRuleResponse(c, r))
	}
	RespondSuccess(c, resp)
}

// ── 创建 ─────────────────────────────────────────────────────────────────────

// CreateRule 为当前用户新建一条规则
//
// POST /api/v1/user/clash/rules
func (h *ClashHandler) CreateRule(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name        string `json:"name"`
		RuleContent string `json:"rule_content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "invalid request body")
		return
	}
	if req.Name == "" {
		req.Name = "Default Rule"
	}

	token, err := generateToken()
	if err != nil {
		logger.Errorf("CreateRule: generate token: %v", err)
		RespondInternalError(c, "failed to generate subscribe token")
		return
	}

	rule := &database.ClashRule{
		UserID:         userID,
		Name:           req.Name,
		RuleContent:    req.RuleContent,
		SubscribeToken: token,
	}

	if err := database.Get().ClashRuleStore().Create(c.Request.Context(), rule); err != nil {
		logger.Errorf("CreateRule: %v", err)
		RespondInternalError(c, "failed to create rule")
		return
	}

	RespondCreated(c, toRuleResponse(c, rule))
}

// ── 查询单条 ──────────────────────────────────────────────────────────────────

// GetRule 获取单条规则
//
// GET /api/v1/user/clash/rules/:id
func (h *ClashHandler) GetRule(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid rule id")
		return
	}

	rule, ok := ownerCheck(c, userID, ruleID)
	if !ok {
		return
	}

	RespondSuccess(c, toRuleResponse(c, rule))
}

// ── 更新 ─────────────────────────────────────────────────────────────────────

// UpdateRule 更新规则名称或内容
//
// PUT /api/v1/user/clash/rules/:id
func (h *ClashHandler) UpdateRule(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid rule id")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		RuleContent *string `json:"rule_content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "invalid request body")
		return
	}

	rule, ok := ownerCheck(c, userID, ruleID)
	if !ok {
		return
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.RuleContent != nil {
		rule.RuleContent = *req.RuleContent
	}

	if err := database.Get().ClashRuleStore().Update(c.Request.Context(), rule); err != nil {
		logger.Errorf("UpdateRule: %v", err)
		RespondInternalError(c, "failed to update rule")
		return
	}

	RespondSuccess(c, toRuleResponse(c, rule))
}

// ── 删除 ─────────────────────────────────────────────────────────────────────

// DeleteRule 删除一条规则
//
// DELETE /api/v1/user/clash/rules/:id
func (h *ClashHandler) DeleteRule(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid rule id")
		return
	}

	if _, ok := ownerCheck(c, userID, ruleID); !ok {
		return
	}

	if err := database.Get().ClashRuleStore().Delete(c.Request.Context(), ruleID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			RespondNotFound(c, "rule not found")
			return
		}
		logger.Errorf("DeleteRule: %v", err)
		RespondInternalError(c, "failed to delete rule")
		return
	}

	RespondSuccessWithMessage(c, "rule deleted")
}

// ── 重置内容 ──────────────────────────────────────────────────────────────────

// ResetRuleContent 清除自定义内容，恢复为系统默认
//
// POST /api/v1/user/clash/rules/:id/reset
func (h *ClashHandler) ResetRuleContent(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid rule id")
		return
	}

	rule, ok := ownerCheck(c, userID, ruleID)
	if !ok {
		return
	}

	rule.RuleContent = ""
	if err := database.Get().ClashRuleStore().Update(c.Request.Context(), rule); err != nil {
		logger.Errorf("ResetRuleContent: %v", err)
		RespondInternalError(c, "failed to reset rule content")
		return
	}

	RespondSuccess(c, toRuleResponse(c, rule))
}

// ── 重新生成令牌 ──────────────────────────────────────────────────────────────

// RegenerateToken 重新生成某条规则的订阅令牌（旧令牌立即失效）
//
// POST /api/v1/user/clash/rules/:id/token
func (h *ClashHandler) RegenerateToken(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		RespondBadRequest(c, "invalid rule id")
		return
	}

	rule, ok := ownerCheck(c, userID, ruleID)
	if !ok {
		return
	}

	newToken, err := generateToken()
	if err != nil {
		logger.Errorf("RegenerateToken: %v", err)
		RespondInternalError(c, "failed to generate token")
		return
	}

	rule.SubscribeToken = newToken
	if err := database.Get().ClashRuleStore().Update(c.Request.Context(), rule); err != nil {
		logger.Errorf("RegenerateToken: update: %v", err)
		RespondInternalError(c, "failed to save new token")
		return
	}

	RespondSuccess(c, gin.H{
		"subscribe_token": newToken,
		"subscribe_url":   subscribeURL(c, newToken),
	})
}

// ── 获取系统默认规则内容 ──────────────────────────────────────────────────────

// GetDefaultRule 返回系统默认 rule.yaml 的原始内容，供前端编辑器预填使用
//
// GET /api/v1/user/clash/default-rule
func (h *ClashHandler) GetDefaultRule(c *gin.Context) {
	content, err := h.readDefaultRule()
	if err != nil {
		logger.Errorf("GetDefaultRule: %v", err)
		RespondInternalError(c, "default rule file not available")
		return
	}
	RespondSuccess(c, gin.H{"content": content})
}

// ── 公开订阅下载（无需鉴权）──────────────────────────────────────────────────

// ServeSubscription 通过订阅令牌下载规则文件，供 Clash 客户端调用
//
// GET /clash/subscribe/:token
func (h *ClashHandler) ServeSubscription(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.String(http.StatusBadRequest, "missing token")
		return
	}

	rule, err := database.Get().ClashRuleStore().GetByToken(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			c.String(http.StatusNotFound, "subscription not found")
			return
		}
		logger.Errorf("ServeSubscription: db lookup: %v", err)
		c.String(http.StatusInternalServerError, "internal error")
		return
	}

	content := rule.RuleContent
	if content == "" {
		content, err = h.readDefaultRule()
		if err != nil {
			logger.Errorf("ServeSubscription: read default rule: %v", err)
			c.String(http.StatusInternalServerError, "default rule file not available")
			return
		}
	}

	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, rule.Name))
	c.String(http.StatusOK, content)
}

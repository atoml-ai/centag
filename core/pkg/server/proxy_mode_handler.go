package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"centag/core/pkg/proxymode"
	"centag/core/internal/session"
)

// ProxyModeHandler 代理模式 API 处理器
type ProxyModeHandler struct {
	modeMgr      *proxymode.ModeManager
	sessionStore *session.ProxyModeStore
}

// NewProxyModeHandler 创建新的处理器
func NewProxyModeHandler(modeMgr *proxymode.ModeManager, sessionStore *session.ProxyModeStore) *ProxyModeHandler {
	return &ProxyModeHandler{
		modeMgr:      modeMgr,
		sessionStore: sessionStore,
	}
}

// SetProxyMode 设置会话代理模式
// POST /api/v1/session/proxy-mode
func (h *ProxyModeHandler) SetProxyMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Mode    string `json:"mode"`
		Backend string `json:"backend,omitempty"`
		Model   string `json:"model,omitempty"`
		TTL     int    `json:"ttl,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Mode == "" {
		writeError(w, http.StatusBadRequest, "mode is required")
		return
	}

	// 验证模式是否存在且启用
	mode, exists := h.modeMgr.GetMode(req.Mode)
	if !exists || !mode.Enabled {
		writeError(w, http.StatusBadRequest, "invalid or disabled mode")
		return
	}

	if req.TTL <= 0 {
		req.TTL = 3600 // 默认 1 小时
	}
	if req.TTL > 86400 {
		req.TTL = 86400 // 最大 24 小时
	}

	clientID := extractClientID(r)
	sessionData := session.SessionProxyMode{
		UserID:    clientID,
		ModeKey:   req.Mode,
		BackendID: req.Backend,
		ModelName: req.Model,
		TTL:       req.TTL,
	}

	if err := h.sessionStore.Set(clientID, sessionData); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"mode":    req.Mode,
			"backend": req.Backend,
			"model":   req.Model,
			"ttl":     req.TTL,
		},
	})
}

// GetProxyMode 获取会话代理模式
// GET /api/v1/session/proxy-mode
func (h *ProxyModeHandler) GetProxyMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	clientID := extractClientID(r)
	sessionData, exists := h.sessionStore.Get(clientID)

	var data map[string]interface{}
	if exists {
		data = map[string]interface{}{
			"mode":    sessionData.ModeKey,
			"backend": sessionData.BackendID,
			"model":   sessionData.ModelName,
			"expires_at": sessionData.ExpiresAt,
		}
	} else {
		data = map[string]interface{}{
			"mode": nil,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// DeleteProxyMode 删除会话代理模式
// DELETE /api/v1/session/proxy-mode
func (h *ProxyModeHandler) DeleteProxyMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	clientID := extractClientID(r)
	h.sessionStore.Delete(clientID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "session mode cleared",
	})
}

// ListModes 列出所有代理模式
// GET /api/v1/proxy-modes
func (h *ProxyModeHandler) ListModes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	modes := h.modeMgr.ListModes()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"modes": modes,
		},
	})
}

// CreateMode 创建新代理模式
// POST /api/v1/proxy-modes
func (h *ProxyModeHandler) CreateMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req proxymode.ModeConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.modeMgr.AddMode(req); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    req,
	})
}

// UpdateMode 更新代理模式
// PUT /api/v1/proxy-modes/{key}
func (h *ProxyModeHandler) UpdateMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy-modes/")
	if key == "" {
		writeError(w, http.StatusBadRequest, "mode key required")
		return
	}

	var req struct {
		Name        string                 `json:"name,omitempty"`
		Type        string                 `json:"type,omitempty"`
		Description string                 `json:"description,omitempty"`
		Enabled     bool                   `json:"enabled"`
		Config      map[string]interface{} `json:"config,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing, exists := h.modeMgr.GetMode(key)
	if !exists {
		writeError(w, http.StatusNotFound, "mode not found")
		return
	}

	updated := proxymode.ModeConfig{
		Key:         key,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Enabled:     req.Enabled,
		Config:      req.Config,
	}

	// 保留原有值
	if updated.Name == "" {
		updated.Name = existing.Name
	}
	if updated.Type == "" {
		updated.Type = existing.Type
	}
	if updated.Description == "" {
		updated.Description = existing.Description
	}
	if updated.Config == nil {
		updated.Config = existing.Config
	}

	if err := h.modeMgr.UpdateMode(updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    updated,
	})
}

// DeleteMode 删除代理模式
// DELETE /api/v1/proxy-modes/{key}
func (h *ProxyModeHandler) DeleteMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy-modes/")
	if key == "" {
		writeError(w, http.StatusBadRequest, "mode key required")
		return
	}

	if err := h.modeMgr.DeleteMode(key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "mode deleted",
	})
}

// EnableMode 启用/禁用代理模式
// POST /api/v1/proxy-modes/{key}/enable
func (h *ProxyModeHandler) EnableMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy-modes/")
	key := strings.TrimSuffix(path, "/enable")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.modeMgr.EnableMode(key, req.Enabled); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"key":     key,
			"enabled": req.Enabled,
		},
	})
}

// extractClientID 从请求中提取客户端 ID
func extractClientID(r *http.Request) string {
	// 优先使用 X-Client-ID 头
	if clientID := r.Header.Get("X-Client-ID"); clientID != "" {
		return clientID
	}

	// 使用 X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// fallback 到 IP 地址
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

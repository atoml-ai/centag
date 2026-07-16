package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"centag/core/internal/session"
	"centag/core/pkg/proxymode"
)

func TestNewProxyModeHandler(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	
	handler := NewProxyModeHandler(modeMgr, sessionStore)
	if handler == nil {
		t.Fatal("NewProxyModeHandler() returned nil")
	}
}

func TestHandleSetProxyMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "valid request",
			body: map[string]interface{}{
				"mode":    "#d",
				"backend": "ollama-local",
				"model":   "qwen2.5:7b",
				"ttl":     3600,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "minimal request",
			body: map[string]interface{}{
				"mode": "#s",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid mode",
			body: map[string]interface{}{
				"mode": "#invalid",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing mode",
			body: map[string]interface{}{
				"backend": "ollama-local",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       map[string]interface{}{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid ttl uses default",
			body: map[string]interface{}{
				"mode": "#d",
				"ttl":  -100,
			},
			wantStatus: http.StatusOK, // 负数 TTL 会使用默认值 3600
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/v1/session/proxy-mode", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.SetProxyMode(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				json.Unmarshal(rr.Body.Bytes(), &resp)
				if resp["success"] != true {
					t.Error("Expected success=true in response")
				}
			}
		})
	}
}

func TestHandleGetProxyMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	// First set a session
	sessionData := session.SessionProxyMode{
		UserID:    "test-user",
		ModeKey:   "#d",
		BackendID: "ollama-local",
		ModelName: "qwen2.5:7b",
		TTL:       3600,
	}
	sessionStore.Set("test-user", sessionData)

	// Then get it
	req := httptest.NewRequest("GET", "/api/v1/session/proxy-mode", nil)
	req.Header.Set("X-Client-ID", "test-user")

	rr := httptest.NewRecorder()
	handler.GetProxyMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp["success"] != true {
		t.Error("Expected success=true")
	}
	
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}
	if data["mode"] != "#d" {
		t.Errorf("Expected mode #d, got %v", data["mode"])
	}
}

func TestHandleGetProxyMode_NoSession(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	req := httptest.NewRequest("GET", "/api/v1/session/proxy-mode", nil)
	req.Header.Set("X-Client-ID", "nonexistent-user")

	rr := httptest.NewRecorder()
	handler.GetProxyMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp["success"] != true {
		t.Error("Expected success=true")
	}
	
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}
	if data["mode"] != nil {
		t.Error("Expected mode to be null for nonexistent session")
	}
}

func TestHandleDeleteProxyMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	// First set a session
	sessionData := session.SessionProxyMode{
		UserID:  "test-user",
		ModeKey: "#d",
		TTL:     3600,
	}
	sessionStore.Set("test-user", sessionData)

	// Then delete it
	req := httptest.NewRequest("DELETE", "/api/v1/session/proxy-mode", nil)
	req.Header.Set("X-Client-ID", "test-user")

	rr := httptest.NewRecorder()
	handler.DeleteProxyMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Verify it's deleted
	_, exists := sessionStore.Get("test-user")
	if exists {
		t.Error("Expected session to be deleted")
	}
}

func TestHandleListModes(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	req := httptest.NewRequest("GET", "/api/v1/proxy-modes", nil)

	rr := httptest.NewRecorder()
	handler.ListModes(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp["success"] != true {
		t.Error("Expected success=true")
	}
	
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}
	
	modes, ok := data["modes"].([]interface{})
	if !ok {
		t.Fatal("Expected modes array")
	}
	
	// Should have at least 6 default modes
	if len(modes) < 6 {
		t.Errorf("Expected at least 6 modes, got %d", len(modes))
	}
}

func TestHandleCreateMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid custom mode",
			body: map[string]interface{}{
				"key":         "#x",
				"name":        "自定义模式",
				"type":        "custom",
				"description": "测试自定义模式",
				"enabled":     true,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "duplicate key",
			body: map[string]interface{}{
				"key":  "#d",
				"name": "重复模式",
				"type": "custom",
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "missing key",
			body: map[string]interface{}{
				"name": "无关键字模式",
				"type": "custom",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid key format",
			body: map[string]interface{}{
				"key":  "invalid",
				"name": "无效关键字",
				"type": "custom",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"key":  "#y",
				"type": "custom",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing type",
			body: map[string]interface{}{
				"key":  "#y",
				"name": "无类型模式",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/v1/proxy-modes", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateMode(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestHandleUpdateMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	// First create a custom mode
	createBody := map[string]interface{}{
		"key":         "#u",
		"name":        "更新测试模式",
		"type":        "custom",
		"description": "用于测试更新",
		"enabled":     true,
	}
	createBytes, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest("POST", "/api/v1/proxy-modes", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	handler.CreateMode(createRR, createReq)

	// Update it
	updateBody := map[string]interface{}{
		"name":        "已更新模式",
		"description": "已更新描述",
		"enabled":     false,
	}
	updateBytes, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest("PUT", "/api/v1/proxy-modes/#u", bytes.NewReader(updateBytes))
	updateReq.Header.Set("Content-Type", "application/json")

	updateRR := httptest.NewRecorder()
	handler.UpdateMode(updateRR, updateReq)

	if updateRR.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", updateRR.Code)
	}

	// Verify update
	mode, exists := modeMgr.GetMode("#u")
	if !exists {
		t.Fatal("Expected mode to exist")
	}
	if mode.Name != "已更新模式" {
		t.Errorf("Expected updated name, got %s", mode.Name)
	}
	if mode.Enabled {
		t.Error("Expected mode to be disabled")
	}
}

func TestHandleDeleteMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	// First create a custom mode
	createBody := map[string]interface{}{
		"key":         "#z",
		"name":        "待删除模式",
		"type":        "custom",
		"enabled":     true,
	}
	createBytes, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest("POST", "/api/v1/proxy-modes", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	handler.CreateMode(createRR, createReq)

	// Delete it
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/proxy-modes/#z", nil)
	deleteRR := httptest.NewRecorder()
	handler.DeleteMode(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", deleteRR.Code)
	}

	// Verify deletion
	_, exists := modeMgr.GetMode("#del")
	if exists {
		t.Error("Expected mode to be deleted")
	}
}

func TestHandleDeleteMode_Protected(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	// Try to delete a default mode
	req := httptest.NewRequest("DELETE", "/api/v1/proxy-modes/#d", nil)
	rr := httptest.NewRecorder()
	handler.DeleteMode(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleEnableMode(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()
	handler := NewProxyModeHandler(modeMgr, sessionStore)

	// Disable a mode
	body := map[string]interface{}{"enabled": false}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/proxy-modes/#s/enable", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.EnableMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	mode, exists := modeMgr.GetMode("#s")
	if !exists {
		t.Fatal("Expected mode to exist")
	}
	if mode.Enabled {
		t.Error("Expected mode to be disabled")
	}
}

func TestExtractClientID(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		wantID     string
	}{
		{
			name: "X-Client-ID header",
			headers: map[string]string{
				"X-Client-ID": "custom-client-id",
			},
			remoteAddr: "127.0.0.1:1234",
			wantID:     "custom-client-id",
		},
		{
			name:       "fallback to remote addr",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.100:5678",
			wantID:     "192.168.1.100",
		},
		{
			name: "X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "10.0.0.1",
			},
			remoteAddr: "127.0.0.1:1234",
			wantID:     "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			id := extractClientID(req)
			if id != tt.wantID {
				t.Errorf("Expected client ID %s, got %s", tt.wantID, id)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	
	data := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"key": "value",
		},
	}
	
	writeJSON(rr, http.StatusOK, data)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp["success"] != true {
		t.Error("Expected success=true")
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	
	writeError(rr, http.StatusBadRequest, "test error")
	
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp["success"] != false {
		t.Error("Expected success=false")
	}
	if resp["error"] != "test error" {
		t.Errorf("Expected error message 'test error', got %v", resp["error"])
	}
}

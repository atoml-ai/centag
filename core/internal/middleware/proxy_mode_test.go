package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/proxymode"
	"centag/core/internal/proxy"
	"centag/core/internal/session"
)

func TestParseProxyModeFromHeader(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		wantMode    string
		wantBackend string
		wantModel   string
		wantFound   bool
	}{
		{
			name: "valid mode header",
			headers: map[string]string{
				"X-Centag-Mode": "#d",
			},
			wantMode:  "#d",
			wantFound: true,
		},
		{
			name: "mode with backend",
			headers: map[string]string{
				"X-Centag-Mode":    "#d",
				"X-Centag-Backend": "ollama-local",
			},
			wantMode:    "#d",
			wantBackend: "ollama-local",
			wantFound:   true,
		},
		{
			name: "mode with backend and model",
			headers: map[string]string{
				"X-Centag-Mode":    "#m",
				"X-Centag-Backend": "openai-api",
				"X-Centag-Model":   "gpt-4",
			},
			wantMode:    "#m",
			wantBackend: "openai-api",
			wantModel:   "gpt-4",
			wantFound:   true,
		},
		{
			name:      "no headers",
			headers:   map[string]string{},
			wantFound: false,
		},
		{
			name: "only backend header",
			headers: map[string]string{
				"X-Centag-Backend": "ollama-local",
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			mode, backend, model, found := ParseProxyModeFromHeader(req)

			if found != tt.wantFound {
				t.Errorf("Expected found=%v, got %v", tt.wantFound, found)
			}
			if tt.wantFound {
				if mode != tt.wantMode {
					t.Errorf("Expected mode %s, got %s", tt.wantMode, mode)
				}
				if backend != tt.wantBackend {
					t.Errorf("Expected backend %s, got %s", tt.wantBackend, backend)
				}
				if model != tt.wantModel {
					t.Errorf("Expected model %s, got %s", tt.wantModel, model)
				}
			}
		})
	}
}

func TestParseProxyModeFromBody(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]interface{}
		wantMode    string
		wantBackend string
		wantModel   string
		wantFound   bool
	}{
		{
			name: "valid centag field",
			body: map[string]interface{}{
				"centag": map[string]interface{}{
					"mode": "#s",
				},
			},
			wantMode:  "#s",
			wantFound: true,
		},
		{
			name: "with backend and model",
			body: map[string]interface{}{
				"centag": map[string]interface{}{
					"mode":    "#d",
					"backend": "ollama-local",
					"model":   "qwen2.5:7b",
				},
			},
			wantMode:    "#d",
			wantBackend: "ollama-local",
			wantModel:   "qwen2.5:7b",
			wantFound:   true,
		},
		{
			name: "no centag field",
			body: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "hello"},
				},
			},
			wantFound: false,
		},
		{
			name: "empty centag field",
			body: map[string]interface{}{
				"centag": map[string]interface{}{},
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			mode, backend, model, found := ParseProxyModeFromBody(req)

			if found != tt.wantFound {
				t.Errorf("Expected found=%v, got %v", tt.wantFound, found)
			}
			if tt.wantFound {
				if mode != tt.wantMode {
					t.Errorf("Expected mode %s, got %s", tt.wantMode, mode)
				}
				if backend != tt.wantBackend {
					t.Errorf("Expected backend %s, got %s", tt.wantBackend, backend)
				}
				if model != tt.wantModel {
					t.Errorf("Expected model %s, got %s", tt.wantModel, model)
				}
			}
		})
	}
}

func TestParseProxyModeFromContent(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantMode       string
		wantBackend    string
		wantPipelineID string
		wantModel      string
		wantDeptTag    string
		wantContent    string
		wantFound      bool
		wantModified   bool
	}{
		{
			name:         "valid prefix",
			content:      "#d 你好，请帮我写代码",
			wantMode:     "#d",
			wantContent:  "你好，请帮我写代码",
			wantFound:    true,
			wantModified: true,
		},
		{
			name:         "prefix with cost center",
			content:      "#s /cost:finance 本月预算还剩多少",
			wantMode:     "#s",
			wantDeptTag:  "finance",
			wantContent:  "本月预算还剩多少",
			wantFound:    true,
			wantModified: true,
		},
		{
			name:         "prefix with backend",
			content:      "#d /backend:ollama-local 分析问题",
			wantMode:     "#d",
			wantBackend:  "ollama-local",
			wantContent:  "分析问题",
			wantFound:    true,
			wantModified: true,
		},
		{
			name:         "prefix with model",
			content:      "#m /model:gpt-4 翻译这段话",
			wantMode:     "#m",
			wantModel:    "gpt-4",
			wantContent:  "翻译这段话",
			wantFound:    true,
			wantModified: true,
		},
		{
			name:           "pipeline id short token",
			content:        "#p /p:smart-scheduling 执行任务",
			wantMode:       "#p",
			wantPipelineID: "smart-scheduling",
			wantContent:    "执行任务",
			wantFound:      true,
			wantModified:   true,
		},
		{
			name:           "pipeline and backend",
			content:        "#p /pipeline:direct-backend /backend:bigmodel 问",
			wantMode:       "#p",
			wantPipelineID: "direct-backend",
			wantBackend:    "bigmodel",
			wantContent:    "问",
			wantFound:      true,
			wantModified:   true,
		},
		{
			name:         "no prefix",
			content:      "你好，请帮我",
			wantFound:    false,
			wantModified: false,
		},
		{
			name:         "invalid prefix",
			content:      "/hello 你好",
			wantFound:    false,
			wantModified: false,
		},
		{
			name:         "empty content",
			content:      "",
			wantFound:    false,
			wantModified: false,
		},
		{
			name:         "prefix only",
			content:      "#d",
			wantMode:     "#d",
			wantContent:  "",
			wantFound:    true,
			wantModified: true,
		},
		{
			name:         "multiline content",
			content:      "#s 第一行\n第二行\n第三行",
			wantMode:     "#s",
			wantContent:  "第一行\n第二行\n第三行",
			wantFound:    true,
			wantModified: true,
		},
		{
			name:         "shortcut after agent preamble",
			content:      "<memory_context>\ncontext\n</memory_context>\n\n<session id=\"abc\" />\n\n#ch 最近的3次git提交分析一下",
			wantMode:     "#ch",
			wantContent:  "<memory_context>\ncontext\n</memory_context>\n\n<session id=\"abc\" />\n\n最近的3次git提交分析一下",
			wantFound:    true,
			wantModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, backend, pipelineID, model, deptTag, content, found, modified := ParseProxyModeFromContent(tt.content)

			if found != tt.wantFound {
				t.Errorf("Expected found=%v, got %v", tt.wantFound, found)
			}
			if modified != tt.wantModified {
				t.Errorf("Expected modified=%v, got %v", tt.wantModified, modified)
			}
			if tt.wantFound {
				if mode != tt.wantMode {
					t.Errorf("Expected mode %s, got %s", tt.wantMode, mode)
				}
				if backend != tt.wantBackend {
					t.Errorf("Expected backend %s, got %s", tt.wantBackend, backend)
				}
				if pipelineID != tt.wantPipelineID {
					t.Errorf("Expected pipelineID %s, got %s", tt.wantPipelineID, pipelineID)
				}
				if model != tt.wantModel {
					t.Errorf("Expected model %s, got %s", tt.wantModel, model)
				}
				if deptTag != tt.wantDeptTag {
					t.Errorf("Expected deptTag %s, got %s", tt.wantDeptTag, deptTag)
				}
				if content != tt.wantContent {
					t.Errorf("Expected content %q, got %q", tt.wantContent, content)
				}
			}
		})
	}
}

func TestExtractModeFromChatContent(t *testing.T) {
	tests := []struct {
		name         string
		messages     []interface{}
		wantMode     string
		wantFound    bool
		wantModified bool
	}{
		{
			name: "mode in last user message",
			messages: []interface{}{
				map[string]interface{}{"role": "system", "content": "You are helpful"},
				map[string]interface{}{"role": "user", "content": "#d 帮我写代码"},
			},
			wantMode:     "#d",
			wantFound:    true,
			wantModified: true,
		},
		{
			name: "no user messages",
			messages: []interface{}{
				map[string]interface{}{"role": "system", "content": "You are helpful"},
			},
			wantFound: false,
		},
		{
			name: "mode in last user message after history",
			messages: []interface{}{
				map[string]interface{}{"role": "user", "content": "Hello"},
				map[string]interface{}{"role": "assistant", "content": "Hi"},
				map[string]interface{}{"role": "user", "content": "#s 继续"},
			},
			wantMode:     "#s",
			wantFound:    true,
			wantModified: true,
		},
		{
			name: "ignore keyword in earlier turn when current turn has none",
			messages: []interface{}{
				map[string]interface{}{"role": "user", "content": "#a python的优缺点"},
				map[string]interface{}{"role": "assistant", "content": "..."},
				map[string]interface{}{"role": "user", "content": "再讲讲缺点"},
			},
			wantFound: false,
		},
		{
			name: "array content with shortcut after agent preamble",
			messages: []interface{}{
				map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "<memory_context>\ncontext\n</memory_context>\n\n<session id=\"abc\" />\n\n#ch 最近的3次git提交分析一下",
						},
					},
				},
			},
			wantMode:     "#ch",
			wantFound:    true,
			wantModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, _, _, _, _, found, modified := ExtractModeFromChatContent(tt.messages)

			if found != tt.wantFound {
				t.Errorf("Expected found=%v, got %v", tt.wantFound, found)
			}
			if modified != tt.wantModified {
				t.Errorf("Expected modified=%v, got %v", tt.wantModified, modified)
			}
			if tt.wantFound && mode != tt.wantMode {
				t.Errorf("Expected mode %s, got %s", tt.wantMode, mode)
			}
		})
	}
}

func TestProxyModeMiddleware(t *testing.T) {
	modeMgr := proxymode.NewManager()
	sessionStore := session.NewProxyModeStore()

	tests := []struct {
		name       string
		body       string
		headers    map[string]string
		wantStatus int
		wantMode   string
	}{
		{
			name: "content shortcut sets resolved mode",
			body: `{"model": "gpt-4", "messages": [{"role": "user", "content": "#d hello"}]}`,
			wantStatus: http.StatusOK,
			wantMode:   "#d",
		},
		{
			name: "model pipeline prefix sets resolved mode",
			body: `{"model": "pipeline.direct-backend glm-4", "messages": [{"role": "user", "content": "hello"}]}`,
			wantStatus: http.StatusOK,
			wantMode:   "#d",
		},
		{
			name: "model pipeline prefix only id sets resolved mode",
			body: `{"model": "pipeline.direct-backend", "messages": [{"role": "user", "content": "hello"}]}`,
			wantStatus: http.StatusOK,
			wantMode:   "#d",
		},
		{
			name: "header ignored without allow_header_override",
			body: `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`,
			headers: map[string]string{
				"X-Centag-Mode": "#d",
			},
			wantStatus: http.StatusOK,
			wantMode:   "",
		},
		{
			name: "invalid content shortcut",
			body: `{"model": "gpt-4", "messages": [{"role": "user", "content": "#invalid hello"}]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "plain openai request",
			body:       `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`,
			headers:    map[string]string{},
			wantStatus: http.StatusOK,
			wantMode:   "",
		},
		{
			name: "openai array content shortcut",
			body: `{"model":"test","messages":[{"role":"user","content":[{"type":"text","text":"<memory_context>\nctx\n</memory_context>\n\n#ch 请分析硬件"}]}]}`,
			wantStatus: http.StatusOK,
			wantMode:   "#ch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			var capturedMode string
			h := ProxyModeMiddleware(modeMgr, sessionStore)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedMode = r.Header.Get(proxy.HeaderCentagResolvedMode)
				w.WriteHeader(http.StatusOK)
			}))

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
			if tt.wantStatus == http.StatusOK && capturedMode != tt.wantMode {
				t.Errorf("Expected resolved mode %q, got %q", tt.wantMode, capturedMode)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		remote  string
		wantIP  string
	}{
		{
			name: "X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			remote: "127.0.0.1:1234",
			wantIP: "192.168.1.100",
		},
		{
			name: "X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "10.0.0.1",
			},
			remote: "127.0.0.1:1234",
			wantIP: "10.0.0.1",
		},
		{
			name:    "remote addr fallback",
			headers: map[string]string{},
			remote:  "192.168.1.50:1234",
			wantIP:  "192.168.1.50",
		},
		{
			name: "X-Forwarded-For with multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 172.16.0.1",
			},
			remote: "127.0.0.1:1234",
			wantIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := GetClientIP(req)
			if ip != tt.wantIP {
				t.Errorf("Expected IP %s, got %s", tt.wantIP, ip)
			}
		})
	}
}

func TestStripProxyModeFromBody(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		wantMode    string
		wantBackend string
		wantFound   bool
	}{
		{
			name: "with centag field",
			input: map[string]interface{}{
				"model": "gpt-4",
				"centag": map[string]interface{}{
					"mode":    "#d",
					"backend": "ollama-local",
				},
			},
			wantMode:    "#d",
			wantBackend: "ollama-local",
			wantFound:   true,
		},
		{
			name: "without centag field",
			input: map[string]interface{}{
				"model":    "gpt-4",
				"messages": []interface{}{},
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, backend, found := StripProxyModeFromBody(tt.input)
			if found != tt.wantFound {
				t.Errorf("Expected found=%v, got %v", tt.wantFound, found)
			}
			if tt.wantFound {
				if mode != tt.wantMode {
					t.Errorf("Expected mode %s, got %s", tt.wantMode, mode)
				}
				if backend != tt.wantBackend {
					t.Errorf("Expected backend %s, got %s", tt.wantBackend, backend)
				}
				// Verify centag field is removed
				if _, exists := tt.input["centag"]; exists {
					t.Error("Expected centag field to be stripped")
				}
			}
		})
	}
}

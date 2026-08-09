package proxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"centag/core/pkg/config"
)

func TestDetectProxyModeStandardOpenAIUsesDefault(t *testing.T) {
	t.Cleanup(func() { config.Set(nil) })

	req := newChatRequest(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	req.Header.Set("X-Proxy-Mode", "smart-scheduling")

	mode, source := detectProxyModeWithConfig(req, &config.Config{
		Proxy: config.ProxyConfig{
			DefaultMode:         "direct-backend",
			AllowHeaderOverride: false,
		},
	})

	if mode != ModeDefault {
		t.Fatalf("mode=%q, want %q (header ignored when override disabled)", mode, ModeDefault)
	}
	if source != "default" {
		t.Fatalf("source=%q, want default", source)
	}
}

func TestDetectProxyModeHeaderOverrideEnabled(t *testing.T) {
	req := newChatRequest(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	req.Header.Set("X-Proxy-Mode", "smart-scheduling")

	mode, source := detectProxyModeWithConfig(req, &config.Config{
		Proxy: config.ProxyConfig{AllowHeaderOverride: true},
	})

	if mode != ModeSmartScheduling {
		t.Fatalf("mode=%q, want smart-scheduling", mode)
	}
	if source != "proxy-mode-header" {
		t.Fatalf("source=%q, want proxy-mode-header", source)
	}
}

func TestDetectProxyModeContentShortcutOverridesHeader(t *testing.T) {
	req := newChatRequest(`{"model":"gpt-4","messages":[{"role":"user","content":"#d hi"}]}`)
	req.Header.Set("X-Proxy-Mode", "smart-scheduling")

	mode, source := detectProxyModeWithConfig(req, &config.Config{
		Proxy: config.ProxyConfig{AllowHeaderOverride: true},
	})

	if mode != ModeDirectBackend {
		t.Fatalf("mode=%q, want direct-backend", mode)
	}
	if source != "content-prefix" {
		t.Fatalf("source=%q, want content-prefix", source)
	}
}

func TestDetectProxyModeModelPrefix(t *testing.T) {
	req := newChatRequest(`{"model":"pipeline.direct-backend glm-4-flash","messages":[{"role":"user","content":"hi"}]}`)

	mode, source := detectProxyModeWithConfig(req, &config.Config{
		Proxy: config.ProxyConfig{DefaultMode: "smart-scheduling"},
	})

	if mode != ModeDirectBackend {
		t.Fatalf("mode=%q, want direct-backend", mode)
	}
	if source != "model-prefix" {
		t.Fatalf("source=%q, want model-prefix", source)
	}
}

func TestDetectProxyModeModelPrefixPipelineOnly(t *testing.T) {
	req := newChatRequest(`{"model":"pipeline.direct-backend","messages":[{"role":"user","content":"hi"}]}`)

	mode, source := detectProxyModeWithConfig(req, &config.Config{
		Proxy: config.ProxyConfig{DefaultMode: "smart-scheduling"},
	})

	if mode != ModeDirectBackend {
		t.Fatalf("mode=%q, want direct-backend", mode)
	}
	if source != "model-prefix" {
		t.Fatalf("source=%q, want model-prefix", source)
	}
}

func TestDetectProxyModeResolvedHeader(t *testing.T) {
	req := newChatRequest(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	req.Header.Set(HeaderCentagResolvedMode, "#d")

	mode, source := detectProxyModeWithConfig(req, nil)
	if mode != ModeDirectBackend {
		t.Fatalf("mode=%q, want direct-backend", mode)
	}
	if source != "shortcut" {
		t.Fatalf("source=%q, want shortcut", source)
	}
}

func TestParseModelPipelinePrefix(t *testing.T) {
	tests := []struct {
		model           string
		wantPipeline    string
		wantActualModel string
		wantOK          bool
	}{
		{"centag/direct-backend", "direct-backend", "", true},
		{"centag/direct-backend glm-4-flash", "direct-backend", "glm-4-flash", true},
		{"pipeline.direct-backend", "direct-backend", "", true},
		{"pipeline.direct-backend glm-4-flash", "direct-backend", "glm-4-flash", true},
		{"pipeline_smart-scheduling qwen2.5", "smart-scheduling", "qwen2.5", true},
		{"pipeline.coding-agent.auto", "coding-agent", "", true},
		{"pipeline.pipeline.coding-agent.auto", "coding-agent", "", true},
		{"gpt-4", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pipelineID, actualModel, ok := parseModelPipelinePrefix(tt.model)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tt.wantOK)
			}
			if pipelineID != tt.wantPipeline {
				t.Fatalf("pipelineID=%q, want %q", pipelineID, tt.wantPipeline)
			}
			if actualModel != tt.wantActualModel {
				t.Fatalf("actualModel=%q, want %q", actualModel, tt.wantActualModel)
			}
		})
	}
}

func TestFindShortcutTokenInContentRejectsMarkdownHeading(t *testing.T) {
	content := "## 当前 USER.md 内容\n```markdown\n# 用户信息\n```\n\n## 本轮对话\n### 用户\n#ch 请分析本地系统硬件信息"
	if got := findShortcutTokenInContent(content); got != "" {
		t.Fatalf("findShortcutTokenInContent() = %q, want empty (## is not a valid shortcut)", got)
	}
}

func TestDetectProxyModeMarkdownHeadingFallsBackToDefault(t *testing.T) {
	body := `{"model":"test","messages":[{"role":"user","content":"## 当前 USER.md 内容\n\n## 本轮对话\n### 用户\n#ch 请分析本地系统硬件信息"}]}`
	req := newChatRequest(body)

	mode, source := detectProxyModeWithConfig(req, nil)
	if mode != ModeDefault {
		t.Fatalf("mode=%q, want %q (## must not be treated as shortcut)", mode, ModeDefault)
	}
	if source != "default" {
		t.Fatalf("source=%q, want default", source)
	}
}

func TestDetectProxyModeArrayContentShortcutAfterPreamble(t *testing.T) {
	body := `{"model":"test","messages":[{"role":"user","content":[{"type":"text","text":"<memory_context>\nctx\n</memory_context>\n\n#ch 请分析硬件"}]}]}`
	req := newChatRequest(body)

	mode, source := detectProxyModeWithConfig(req, nil)
	if mode != ModeCacheHit {
		t.Fatalf("mode=%q, want cache-hit", mode)
	}
	if source != "content-prefix" {
		t.Fatalf("source=%q, want content-prefix", source)
	}
}

func TestApplyModelPipelinePrefixToBody(t *testing.T) {
	body := map[string]interface{}{"model": "pipeline.direct-backend glm-4-flash"}
	pipelineID, applied := ApplyModelPipelinePrefixToBody(body)
	if !applied || pipelineID != "direct-backend" {
		t.Fatalf("pipelineID=%q applied=%v", pipelineID, applied)
	}
	if body["model"] != "glm-4-flash" {
		t.Fatalf("model=%v, want glm-4-flash", body["model"])
	}
}

func TestApplyModelPipelinePrefixToBodyPipelineOnly(t *testing.T) {
	t.Cleanup(func() { config.Set(nil) })
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{DefaultModel: "mimo-v2.5-free"},
	})

	body := map[string]interface{}{"model": "pipeline.transparent-proxy"}
	pipelineID, applied := ApplyModelPipelinePrefixToBody(body)
	if !applied || pipelineID != "transparent-proxy" {
		t.Fatalf("pipelineID=%q applied=%v", pipelineID, applied)
	}
	if body["model"] != "pipeline.transparent-proxy" {
		t.Fatalf("model=%v, want pipeline.transparent-proxy kept (no system default injection)", body["model"])
	}
}

func TestApplyModelPipelinePrefixToBodyPipelineOnlyNoDefaultKeepsVirtual(t *testing.T) {
	t.Cleanup(func() { config.Set(nil) })
	config.Set(&config.Config{Proxy: config.ProxyConfig{}})

	body := map[string]interface{}{"model": "pipeline.direct-backend"}
	pipelineID, applied := ApplyModelPipelinePrefixToBody(body)
	if !applied || pipelineID != "direct-backend" {
		t.Fatalf("pipelineID=%q applied=%v", pipelineID, applied)
	}
	if body["model"] != "pipeline.direct-backend" {
		t.Fatalf("model=%v, want pipeline.direct-backend when no default_model", body["model"])
	}
}

func newChatRequest(body string) *http.Request {
	return &http.Request{
		Method: http.MethodPost,
		Header: make(http.Header),
		URL:    &url.URL{},
		Body:   io.NopCloser(strings.NewReader(body)),
	}
}

func TestExtractCentagSceneBytes(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "valid scene",
			body:     `{"centag":{"scene":"problem_solving"}}`,
			expected: "problem_solving",
		},
		{
			name:     "empty body",
			body:     `{}`,
			expected: "",
		},
		{
			name:     "missing centag",
			body:     `{"model":"gpt-4"}`,
			expected: "",
		},
		{
			name:     "missing scene",
			body:     `{"centag":{"mode":"education-scene"}}`,
			expected: "",
		},
		{
			name:     "empty scene",
			body:     `{"centag":{"scene":""}}`,
			expected: "",
		},
		{
			name:     "whitespace scene",
			body:     `{"centag":{"scene":"  "}}`,
			expected: "",
		},
		{
			name:     "chinese scene",
			body:     `{"centag":{"scene":"解题"}}`,
			expected: "解题",
		},
		{
			name:     "scene with other fields",
			body:     `{"centag":{"mode":"education-scene","scene":"essay_review"},"model":"gpt-4"}`,
			expected: "essay_review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes := []byte(tt.body)
			result := extractCentagSceneBytes(bodyBytes)
			if result != tt.expected {
				t.Fatalf("extractCentagSceneBytes(%q) = %q, want %q", tt.body, result, tt.expected)
			}
		})
	}
}

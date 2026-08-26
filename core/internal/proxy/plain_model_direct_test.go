package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
	openai "centag/plugins/protocol/openai"

	"github.com/gin-gonic/gin"
)

// newPlainModelTestManager 构造带两个后端的测试 Manager：
//   - backend-a 声明 glm-4-plus（别名 glm4p → 实际 glm-4-plus）
//   - backend-b 声明 deepseek-v4-flash
func newPlainModelTestManager(t *testing.T) {
	t.Helper()
	mgr := backend.GetManager()
	if mgr == nil {
		t.Skip("backend manager not initialized")
	}
	upsert := func(cfg *backend.BackendConfig) {
		t.Helper()
		if err := mgr.Update(cfg); err != nil {
			if err := mgr.Add(cfg); err != nil {
				t.Fatalf("upsert backend %s: %v", cfg.ID, err)
			}
		}
	}
	upsert(&backend.BackendConfig{
		ID:      "backend-a",
		Name:    "backend-a",
		Type:    "openai",
		BaseURL: "http://127.0.0.1:9",
		APIKey:  "test-key",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "glm-4-plus", ActualModel: "glm-4-plus"},
			{RequestedModel: "glm4p", ActualModel: "glm-4-plus"},
		},
	})
	upsert(&backend.BackendConfig{
		ID:      "backend-b",
		Name:    "backend-b",
		Type:    "openai",
		BaseURL: "http://127.0.0.1:9",
		APIKey:  "test-key",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "deepseek-v4-flash", ActualModel: "deepseek-v4-flash"},
		},
	})
}

func TestMatchPlainModelBackend(t *testing.T) {
	newPlainModelTestManager(t)

	cases := []struct {
		name      string
		model     string
		wantHit   bool
		wantBID   string
	}{
		{"exact declared model", "glm-4-plus", true, "backend-a"},
		{"alias maps to same backend", "glm4p", true, "backend-a"},
		{"other backend model", "deepseek-v4-flash", true, "backend-b"},
		{"undeclared model misses", "no-such-model", false, ""},
		{"empty model misses", "", false, ""},
		{"auto misses (not explicit)", "auto", false, ""},
		{"pipeline prefixed misses", "centag/direct-backend", false, ""},
		{"builtin pipeline name misses", "direct-backend", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bid, ok := matchPlainModelBackend(tc.model)
			if ok != tc.wantHit {
				t.Fatalf("model=%q hit=%v, want %v", tc.model, ok, tc.wantHit)
			}
			if ok && bid != tc.wantBID {
				t.Fatalf("model=%q backend=%q, want %q", tc.model, bid, tc.wantBID)
			}
		})
	}
}

func TestMatchPlainModelBackendSkipsDisabled(t *testing.T) {
	newPlainModelTestManager(t)
	mgr := backend.GetManager()
	if mgr == nil {
		t.Skip("backend manager not initialized")
	}

	// 禁用声明方后端后不应再命中
	if err := mgr.Update(&backend.BackendConfig{ID: "backend-a", Enabled: false}); err != nil {
		t.Fatalf("disable backend-a: %v", err)
	}
	defer func() {
		if err := mgr.Update(&backend.BackendConfig{ID: "backend-a", Enabled: true}); err != nil {
			t.Fatalf("re-enable backend-a: %v", err)
		}
	}()

	if _, ok := matchPlainModelBackend("glm-4-plus"); ok {
		t.Fatal("disabled backend should not be matched")
	}
}

// capturingPipelineEngine 记录最近一次执行的流水线 ID 与输入，供 handler 集成断言。
type capturingPipelineEngine struct {
	ids       map[string]bool
	lastID    string
	lastInput *pipeline.PipelineInput
}

func (s *capturingPipelineEngine) Execute(_ context.Context, id string, in *pipeline.PipelineInput) (*pipeline.PipelineOutput, error) {
	s.lastID = id
	s.lastInput = in
	return &pipeline.PipelineOutput{Content: "ok"}, nil
}

func (s *capturingPipelineEngine) HasPipeline(id string) bool { return s.ids[id] }

func (s *capturingPipelineEngine) RegisterPipeline(*pipeline.AgentPatternPipeline) error { return nil }

func (s *capturingPipelineEngine) ExecuteStream(_ context.Context, id string, in *pipeline.PipelineInput) (<-chan pipeline.PipelineStreamResult, error) {
	s.lastID = id
	s.lastInput = in
	ch := make(chan pipeline.PipelineStreamResult)
	close(ch)
	return ch, nil
}

// newPlainModelDirectEnv 构造可跑通 HandleChatCompletions 的最小环境（config + 协议插件 + 默认解析器）。
// 默认流水线固定为 smart-scheduling，用于区分「直连透明」与「默认解析」两条路径。
// 注意：engine 由用例自行 SetPipelineEngine 注入（该调用会重建 ModeDispatcher）。
func newPlainModelDirectEnv(t *testing.T, plainModelEnabled bool) *Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	prevCfg := config.Get()
	cfg := &config.Config{}
	cfg.Proxy.PlainModelDirectPipeline = &plainModelEnabled
	config.Set(cfg)
	t.Cleanup(func() { config.Set(prevCfg) })

	pluginMgr := plugin.NewManager()
	proto, err := openai.NewProtocol()
	if err != nil {
		t.Fatalf("NewProtocol: %v", err)
	}
	if err := pluginMgr.Register(proto); err != nil {
		t.Fatalf("register openai-protocol: %v", err)
	}

	h := NewHandler(nil, pluginMgr, nil)
	h.SetDefaultPipelineResolver(NewDefaultPipelineResolver(&config.Config{
		Proxy: config.ProxyConfig{
			PipelineConfig: &config.PipelineConfig{DefaultPipeline: "smart-scheduling"},
		},
	}))
	return h
}

// attachEngine 注入捕获引擎并关闭 stream fake（须在 SetPipelineEngine 之后执行，
// 因为该调用内部会以 DefaultStreamFakeConfig 重建 dispatcher）。
func attachEngine(h *Handler) *capturingPipelineEngine {
	engine := &capturingPipelineEngine{ids: map[string]bool{"transparent": true, "smart-scheduling": true}}
	h.SetPipelineEngine(engine)
	h.modeDispatcher.SetStreamFakeConfig(StreamFakeConfig{Enabled: false})
	return engine
}

// runChat 向 handler 发起一条 model=<model> 的 chat/completions 请求。
func runChat(h *Handler, model string) *httptest.ResponseRecorder {
	body := `{"model":"` + model + `","messages":[{"role":"user","content":"hi"}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.HandleChatCompletions(c)
	return w
}

func TestPlainModelDirect_HandlerPinsBackendAndTransparent(t *testing.T) {
	newPlainModelTestManager(t)
	h := newPlainModelDirectEnv(t, true)
	engine := attachEngine(h)

	w := runChat(h, "glm-4-plus")

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if engine.lastID != "transparent" {
		t.Fatalf("pipeline=%q, want transparent", engine.lastID)
	}
	if got := w.Header().Get("X-Proxy-Mode"); got != "transparent" {
		t.Fatalf("X-Proxy-Mode=%q, want transparent", got)
	}
	if got := w.Header().Get("X-Pipeline-ID"); got != "transparent" {
		t.Fatalf("X-Pipeline-ID=%q, want transparent", got)
	}
	if engine.lastInput == nil {
		t.Fatal("pipeline input not captured")
	}
	if got, _ := engine.lastInput.Metadata["backend_id"].(string); got != "backend-a" {
		t.Fatalf("metadata backend_id=%q, want backend-a (pinned)", got)
	}
}

func TestPlainModelDirect_DisabledKeepsDefaultPipeline(t *testing.T) {
	newPlainModelTestManager(t)
	h := newPlainModelDirectEnv(t, false)
	engine := attachEngine(h)

	w := runChat(h, "glm-4-plus")

	if engine.lastID != "smart-scheduling" {
		t.Fatalf("pipeline=%q, want smart-scheduling (default)", engine.lastID)
	}
	if engine.lastInput != nil {
		if _, pinned := engine.lastInput.Metadata["backend_id"]; pinned {
			t.Fatal("backend_id should not be pinned when feature disabled")
		}
	}
	_ = w
}

func TestPlainModelDirect_UnknownModelFallsBackToDefault(t *testing.T) {
	newPlainModelTestManager(t)
	h := newPlainModelDirectEnv(t, true)
	engine := attachEngine(h)

	w := runChat(h, "no-such-model")

	if engine.lastID != "smart-scheduling" {
		t.Fatalf("pipeline=%q, want smart-scheduling (no direct hit)", engine.lastID)
	}
	if engine.lastInput != nil {
		if _, pinned := engine.lastInput.Metadata["backend_id"]; pinned {
			t.Fatal("backend_id should not be pinned for unknown model")
		}
	}
	_ = w
}

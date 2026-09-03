package pipeline

import (
	"context"
	"strings"
	"testing"
)

const verdictFakeErrBody = `{"error":{"type":"server_error","message":"Error from provider (Console): Upstream request failed: Model is unavailable."}}`

// verdictStubNode 测试节点：mode=fake 返回「假成功」（显式错误体 + status_code=400，
// err=nil）；其余返回正常输出。用于触发引擎假成功安全网（TC-EN-*）。
type verdictStubNode struct {
	id   string
	mode string
}

func (n *verdictStubNode) Type() NodeType               { return NodeTypeProcessor }
func (n *verdictStubNode) ID() string                   { return n.id }
func (n *verdictStubNode) Name() string                 { return n.id }
func (n *verdictStubNode) GetConfig() NodeConfig        { return NodeConfig{} }
func (n *verdictStubNode) Validate() error              { return nil }
func (n *verdictStubNode) GetTimeout() int              { return 30 }
func (n *verdictStubNode) GetRetryConfig() *RetryConfig { return nil }
func (n *verdictStubNode) SetTimeout(int)               {}
func (n *verdictStubNode) SetRetryConfig(*RetryConfig)  {}

func (n *verdictStubNode) Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error) {
	if n.mode == "fake" {
		return &NodeOutput{
			Content: verdictFakeErrBody,
			Metadata: map[string]interface{}{
				"status_code": 400,
				"backend_id":  "fake-be",
				"model":       "m",
			},
		}, nil
	}
	if n.mode == "fake401" {
		// TC-EN-007：401 鉴权错误体（R08-① 豁免面）——安全网不得接管。
		return &NodeOutput{
			Content: `{"error":"invalid_api_key"}`,
			Metadata: map[string]interface{}{
				"status_code": 401,
				"backend_id":  "fake-be",
				"model":       "m",
			},
		}, nil
	}
	return &NodeOutput{Content: "ok-fallback"}, nil
}

func registerVerdictStub(t *testing.T, registry *NodeRegistry) {
	t.Helper()
	// NodeType 是闭集枚举（IsValid 白名单），测试通过覆盖测试专用 registry 中的
	// processor factory 并配合 Implementation: "verdict-stub"（绕开 builtin 插件路径）
	// 注入可控行为节点。
	if err := registry.Register(NodeTypeProcessor, func(cfg NodeConfig) (PipelineNode, error) {
		mode, _ := cfg.CustomConfig["mode"].(string)
		return &verdictStubNode{id: "verdict-stub", mode: mode}, nil
	}); err != nil {
		t.Fatalf("register verdict_stub: %v", err)
	}
}

func newVerdictEngine(t *testing.T) (*PipelineEngine, *PipelineRegistry) {
	t.Helper()
	nodeRegistry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(nodeRegistry); err != nil {
		t.Fatalf("RegisterBuiltinNodes: %v", err)
	}
	registerVerdictStub(t, nodeRegistry)
	pipelineRegistry := NewPipelineRegistry()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, &testCapabilityBroker{}, NewPipelineLogger(), nil)
	return engine, pipelineRegistry
}

// TestExecuteLayerNodeFakeSuccess_RescuedByFallbackGroup 覆盖 TC-EN-001：
// 假成功主节点被安全网转为失败后，FallbackGroups 执行备用节点并恢复。
func TestExecuteLayerNodeFakeSuccess_RescuedByFallbackGroup(t *testing.T) {
	engine, registry := newVerdictEngine(t)

	pipeline := &AgentPatternPipeline{
		ID:   "fake-success-rescued",
		Name: "Fake Success Rescued",
		Nodes: []PipelineNodeConfig{
			{
				ID:             "forward",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "fake"}},
				NextNodes:      []string{"forward_fallback"},
			},
			{
				ID:             "forward_fallback",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "ok"}},
				DependsOn:      []string{"forward"},
			},
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 1,
			FallbackGroups: []FallbackGroup{
				{PrimaryNodeID: "forward", FallbackNodes: []string{"forward_fallback"}, MaxAttempts: 2},
			},
		},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	out, err := engine.Execute(context.Background(), "fake-success-rescued", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Content, "ok-fallback") {
		t.Fatalf("content=%q, want fallback output ok-fallback", out.Content)
	}
}

// TestExecuteLayerNodeFakeSuccess_NoFallbackFails 覆盖 TC-EN-002：
// 假成功且无降级组时，管线必须失败（此前误报 success=true）。
func TestExecuteLayerNodeFakeSuccess_NoFallbackFails(t *testing.T) {
	engine, registry := newVerdictEngine(t)

	pipeline := &AgentPatternPipeline{
		ID:   "fake-success-no-fb",
		Name: "Fake Success No Fallback",
		Nodes: []PipelineNodeConfig{
			{
				ID:             "forward",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "fake"}},
			},
		},
		GlobalConfig: GlobalPipelineConfig{ParallelLimit: 1},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ch, err := engine.ExecuteStream(context.Background(), "fake-success-no-fb", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	sawErr := false
	for res := range ch {
		if res.Chunk != nil && res.Chunk.Error != nil {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatal("pipeline must fail on unrescued fake success (was false success before safety net)")
	}
}

// TestExecuteLayerNodeFakeSuccess_GeneratorPlainTextUnaffected 覆盖 TC-EN-003（R03）：
// generator 纯文本/JSON 输出（无 status_code 元数据）不受安全网影响。
func TestExecuteLayerNodeFakeSuccess_GeneratorPlainTextUnaffected(t *testing.T) {
	nodeRegistry := NewNodeRegistry()
	if err := RegisterBuiltinNodes(nodeRegistry); err != nil {
		t.Fatalf("RegisterBuiltinNodes: %v", err)
	}
	// LLM 返回错误形 JSON 文本但无 status_code 元数据：安全网双条件不满足，零回归。
	mockBroker := &testCapabilityBroker{llmClient: &testBackendClient{response: verdictFakeErrBody}}
	pipelineRegistry := NewPipelineRegistry()
	engine := NewPipelineEngine(nodeRegistry, pipelineRegistry, mockBroker, NewPipelineLogger(), nil)

	pipeline := &AgentPatternPipeline{
		ID:   "generator-plain-json",
		Name: "Generator Plain JSON",
		Nodes: []PipelineNodeConfig{
			{
				ID:      "gen",
				Type:    NodeTypeGenerator,
				Kind:    "llm.generate",
				Backend: "ok-backend",
				Model:   "m",
				Config:  NodeConfig{Backend: "ok-backend", Model: "m", PromptTemplate: "{{input}}"},
			},
		},
		GlobalConfig: GlobalPipelineConfig{ParallelLimit: 1},
	}
	if err := pipelineRegistry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	out, err := engine.Execute(context.Background(), "generator-plain-json", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Content, "Model is unavailable") {
		t.Fatalf("content=%q, want generator output passed through unchanged", out.Content)
	}
}

// TestExecuteLayerNodeFakeSuccess_ExecuteEntryExempt 覆盖 TC-EN-006（R08）：
// 调用方注入 raw_error_body_passthrough 标记时，引擎安全网跳过，假成功保持原样。
func TestExecuteLayerNodeFakeSuccess_ExecuteEntryExempt(t *testing.T) {
	engine, registry := newVerdictEngine(t)

	pipeline := &AgentPatternPipeline{
		ID:   "fake-success-exempt",
		Name: "Fake Success Exempt",
		Nodes: []PipelineNodeConfig{
			{
				ID:             "forward",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "fake"}},
			},
		},
		GlobalConfig: GlobalPipelineConfig{ParallelLimit: 1},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	input := &PipelineInput{
		Content: "hello",
		Metadata: map[string]interface{}{
			"raw_error_body_passthrough": true,
		},
	}
	out, err := engine.Execute(context.Background(), "fake-success-exempt", input)
	if err != nil {
		t.Fatalf("Execute: %v, want exempted fake success passthrough", err)
	}
	if !strings.Contains(out.Content, "Model is unavailable") {
		t.Fatalf("content=%q, want raw error body passthrough", out.Content)
	}
}

// TestExecuteLayerNodeFakeSuccess_Auth401CarveOut 覆盖 TC-EN-007（R08-①）：
// 401 鉴权错误体在引擎安全网处同样豁免，保持透传契约。
func TestExecuteLayerNodeFakeSuccess_Auth401CarveOut(t *testing.T) {
	engine, registry := newVerdictEngine(t)

	pipeline := &AgentPatternPipeline{
		ID:   "fake-success-401",
		Name: "Fake Success 401",
		Nodes: []PipelineNodeConfig{
			{
				ID:             "forward",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "fake401"}},
			},
		},
		GlobalConfig: GlobalPipelineConfig{ParallelLimit: 1},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	out, err := engine.Execute(context.Background(), "fake-success-401", &PipelineInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Execute: %v, want 401 auth error body passthrough (carve-out)", err)
	}
	if !strings.Contains(out.Content, "invalid_api_key") {
		t.Fatalf("content=%q, want raw 401 body passthrough", out.Content)
	}
}

// TestUsableFallbackNodeOutputVerdict 覆盖 TC-EN-004（R06）：收口后的四态判定。
func TestUsableFallbackNodeOutputVerdict(t *testing.T) {
	tests := []struct {
		id   string
		out  *NodeOutput
		want bool
	}{
		{
			id:   "TC-EN-004a-openai-error-body",
			out:  &NodeOutput{Content: `{"error":{"type":"server_error"}}`},
			want: false, // 旧实现漏判（无 "type":"error" 字面量），修复后拒绝
		},
		{
			id:   "TC-EN-004b-type-error-substring-in-content",
			out:  &NodeOutput{Content: `{"choices":[{"message":{"content":"see \"type\":\"error\" docs"}}]}`},
			want: true, // 旧实现子串误拒，修复后放行
		},
		{
			id:   "TC-EN-004c-plain-text-not-supported",
			out:  &NodeOutput{Content: "model is not supported here"},
			want: false, // 非 JSON 错误文本防线保留
		},
		{
			id:   "TC-EN-004d-normal",
			out:  &NodeOutput{Content: "hello"},
			want: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			if got := isUsableFallbackNodeOutput(tc.out); got != tc.want {
				t.Fatalf("isUsableFallbackNodeOutput = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestExecuteStreamFakeSuccess_NodeLogRecordsFailure 覆盖 TC-EN-005：
// 假成功经降级恢复后，主节点首次执行日志必须为失败（统计归位，不再记成功）。
func TestExecuteStreamFakeSuccess_NodeLogRecordsFailure(t *testing.T) {
	engine, registry := newVerdictEngine(t)

	pipeline := &AgentPatternPipeline{
		ID:   "fake-success-stream-log",
		Name: "Fake Success Stream Log",
		Nodes: []PipelineNodeConfig{
			{
				ID:             "forward",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "fake"}},
				NextNodes:      []string{"forward_fallback"},
			},
			{
				ID:             "forward_fallback",
				Type:           NodeTypeProcessor,
				Implementation: "verdict-stub",
				Config:         NodeConfig{CustomConfig: map[string]interface{}{"mode": "ok"}},
				DependsOn:      []string{"forward"},
			},
		},
		GlobalConfig: GlobalPipelineConfig{
			ParallelLimit: 1,
			FallbackGroups: []FallbackGroup{
				{PrimaryNodeID: "forward", FallbackNodes: []string{"forward_fallback"}, MaxAttempts: 2},
			},
		},
	}
	if err := registry.Register(pipeline); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ch, err := engine.ExecuteStream(context.Background(), "fake-success-stream-log", &PipelineInput{Content: "hello", Stream: true})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	var execLog *ExecutionLog
	for res := range ch {
		if res.Chunk != nil && res.Chunk.Error != nil {
			t.Fatalf("stream error: %v", res.Chunk.Error)
		}
		if res.Output != nil && res.Output.ExecutionLog != nil {
			execLog = res.Output.ExecutionLog
		}
	}
	if execLog == nil {
		t.Fatal("execution log missing")
	}
	// 假成功被安全网转换后，主节点必须存在失败记录（retry 包装层会先记成功，
	// 安全网补记失败；旧实现只有成功记录）。
	primaryFailedLogged := false
	for i := range execLog.NodeLogs {
		if execLog.NodeLogs[i].NodeID == "forward" && !execLog.NodeLogs[i].Success {
			primaryFailedLogged = true
			break
		}
	}
	if !primaryFailedLogged {
		t.Fatal("primary node must have a failure record in node logs (fake success converted), got success only")
	}
}

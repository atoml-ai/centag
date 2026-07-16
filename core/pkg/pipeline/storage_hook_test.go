package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"centag/core/pkg/storage"
)

// mockKVStore 模拟 KV 存储（基于内存 map）
type mockKVStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newMockKVStore() *mockKVStore {
	return &mockKVStore{data: make(map[string][]byte)}
}

func (m *mockKVStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}
	m.data[key] = data
	return nil
}

func (m *mockKVStore) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return string(data), nil
}

func (m *mockKVStore) GetBytes(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (m *mockKVStore) GetString(ctx context.Context, key string) (string, error) {
	data, err := m.GetBytes(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *mockKVStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockKVStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockKVStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (m *mockKVStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

func (m *mockKVStore) SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	for k, v := range items {
		if err := m.Set(ctx, k, v, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockKVStore) GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, k := range keys {
		if v, err := m.Get(ctx, k); err == nil {
			result[k] = v
		}
	}
	return result, nil
}

func (m *mockKVStore) DeleteBatch(ctx context.Context, keys []string) error {
	for _, k := range keys {
		m.Delete(ctx, k)
	}
	return nil
}

func (m *mockKVStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockKVStore) Count(ctx context.Context, pattern string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.data)), nil
}

func (m *mockKVStore) GetAll(ctx context.Context, pattern string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

func (m *mockKVStore) FlushDB(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	return nil
}

func (m *mockKVStore) Close() error {
	return nil
}

func (m *mockKVStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{}
}

// newStorageHookWithKVStore 创建使用 mock KVStore 的 StorageHook（测试专用）
func newStorageHookWithKVStore(kv storage.KVStore, cfg StorageHookConfig, hookCfg HookBehaviorConfig, namespace string, pipelineID string) *StorageHook {
	h := &StorageHook{
		manager:    nil, // 使用独立 KV 路径
		cfg:        cfg,
		hookCfg:    hookCfg,
		namespace:  namespace,
		pipelineID: pipelineID,
		logger:     nil,
	}
	// 直接设置 kvStore provider，绕过 manager
	h.kvStoreOverride = kv
	return h
}

func TestNewStorageHook_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *AgentPatternPipeline
		enabled  bool
		ns       string
	}{
		{
			name:     "nil pipeline",
			pipeline: nil,
			enabled:  false,
		},
		{
			name: "no storage config",
			pipeline: &AgentPatternPipeline{
				ID: "test",
				GlobalConfig: GlobalPipelineConfig{
					StorageConfig: nil,
				},
			},
			enabled: false,
		},
		{
			name: "storage disabled",
			pipeline: &AgentPatternPipeline{
				ID: "test",
				GlobalConfig: GlobalPipelineConfig{
					StorageConfig: &StorageHookConfig{Enabled: false},
				},
			},
			enabled: false,
		},
		{
			name: "storage enabled with hooks",
			pipeline: &AgentPatternPipeline{
				ID: "test",
				GlobalConfig: GlobalPipelineConfig{
					StorageConfig: &StorageHookConfig{
						Enabled:       true,
						Namespace:     "education-scene",
						AutoSave:      true,
						SaveInterval:  300,
						RetentionDays: 30,
					},
					Hooks: []HookConfig{
						{
							Type: "storage",
							On:   []string{"node_complete", "pipeline_complete"},
							Config: map[string]interface{}{
								"save_user_progress":        true,
								"save_conversation_history": true,
								"save_scene_context":        true,
							},
						},
					},
				},
			},
			enabled: true,
			ns:      "education-scene",
		},
		{
			name: "storage enabled but no storage hook type",
			pipeline: &AgentPatternPipeline{
				ID: "test",
				GlobalConfig: GlobalPipelineConfig{
					StorageConfig: &StorageHookConfig{Enabled: true},
					Hooks: []HookConfig{
						{Type: "other", On: []string{"node_complete"}},
					},
				},
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 使用 nil manager：测试配置解析而不依赖真实存储
			hook := NewStorageHook(nil, tt.pipeline, nil)

			if hook == nil {
				t.Fatal("NewStorageHook returned nil")
			}

			if tt.ns != "" && hook.namespace != tt.ns {
				t.Errorf("namespace = %q, want %q", hook.namespace, tt.ns)
			}
		})
	}
}

func TestStorageHook_IsEnabled(t *testing.T) {
	// 无 manager 时即使配置启用，IsEnabled 也应为 false
	pipeline := &AgentPatternPipeline{
		ID: "test",
		GlobalConfig: GlobalPipelineConfig{
			StorageConfig: &StorageHookConfig{
				Enabled: true,
			},
			Hooks: []HookConfig{
				{
					Type: "storage",
					On:   []string{"node_complete"},
				},
			},
		},
	}

	hook := NewStorageHook(nil, pipeline, nil)
	if hook.IsEnabled() {
		t.Error("IsEnabled should be false when manager is nil")
	}

	// nil StorageHook
	var nilHook *StorageHook
	if nilHook.IsEnabled() {
		t.Error("IsEnabled should be false for nil hook")
	}
}

func TestStorageHook_HookBehaviorConfig(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID: "coding-agent",
		GlobalConfig: GlobalPipelineConfig{
			StorageConfig: &StorageHookConfig{
				Enabled:       true,
				Namespace:     "coding-agent",
				RetentionDays: 30,
			},
			Hooks: []HookConfig{
				{
					Type: "storage",
					On:   []string{"node_complete", "pipeline_complete"},
					Config: map[string]interface{}{
						"save_code_snippets": true,
						"save_solutions":     true,
						"track_file_changes": true,
					},
				},
			},
		},
	}

	hook := NewStorageHook(nil, pipeline, nil)

	if hook.hookCfg.SaveCodeSnippets != true {
		t.Error("SaveCodeSnippets should be true")
	}
	if hook.hookCfg.SaveSolutions != true {
		t.Error("SaveSolutions should be true")
	}
	if hook.hookCfg.TrackFileChanges != true {
		t.Error("TrackFileChanges should be true")
	}
	if hook.hookCfg.SaveUserProgress != false {
		t.Error("SaveUserProgress should be false (not configured)")
	}
	if hook.hookCfg.SaveConversationHistory != false {
		t.Error("SaveConversationHistory should be false (not configured)")
	}
}

func TestStorageHook_Namespace(t *testing.T) {
	pipeline := &AgentPatternPipeline{
		ID: "education-scene",
		GlobalConfig: GlobalPipelineConfig{
			StorageConfig: &StorageHookConfig{
				Enabled:   true,
				Namespace: "",
			},
			Hooks: []HookConfig{
				{
					Type: "storage",
					On:   []string{"node_complete"},
				},
			},
		},
	}

	hook := NewStorageHook(nil, pipeline, nil)
	if hook.namespace != "education-scene" {
		t.Errorf("namespace should fallback to pipeline ID, got %q", hook.namespace)
	}
}

func TestStorageHook_KeyGeneration(t *testing.T) {
	hook := &StorageHook{
		namespace:  "coding-agent",
		pipelineID: "coding-agent",
		cfg:        StorageHookConfig{RetentionDays: 30},
	}

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"history key", hook.key("history"), "pipeline:coding-agent:history"},
		{"scene_context key", hook.key("scene_context"), "pipeline:coding-agent:scene_context"},
		{"code_snippets key", hook.key("code_snippets"), "pipeline:coding-agent:code_snippets"},
		{"solutions key", hook.key("solutions"), "pipeline:coding-agent:solutions"},
		{"node output key", hook.nodeKey("generate", "output"), "pipeline:coding-agent:node:generate:output"},
		{"node output key 2", hook.nodeKey("scene_router", "output"), "pipeline:coding-agent:node:scene_router:output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("key = %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

func TestStorageHook_OnNodeStart_Disabled(t *testing.T) {
	// 禁用时不报错
	hook := &StorageHook{
		cfg:        StorageHookConfig{Enabled: false},
		pipelineID: "test",
		namespace:  "test",
	}
	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})

	// 不应 panic
	hook.OnNodeStart(context.Background(), "node1", execCtx)
	hook.OnNodeComplete(context.Background(), "node1", &NodeOutput{Content: "test"}, execCtx)
	hook.OnPipelineComplete(context.Background(), execCtx)
}

func TestStorageHook_OnNodeStart_WithMockStorage(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true}, HookBehaviorConfig{
		SaveSceneContext: true,
		SaveUserProgress: true,
	}, "education-scene", "education-scene")

	// 预先写入场景上下文
	sceneCtx := map[string]interface{}{"scene": "problem_solving", "difficulty": "medium"}
	sceneData, _ := json.Marshal(sceneCtx)
	kv.Set(context.Background(), "pipeline:education-scene:scene_context", sceneData, 0)

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "education-scene"})
	execCtx.SetVariable("user_id", "user123")

	hook.OnNodeStart(context.Background(), "problem-solver", execCtx)

	// 验证场景上下文已加载
	if scene, ok := execCtx.GetVariable("scene"); !ok || scene != "problem_solving" {
		t.Errorf("scene should be loaded, got %v", scene)
	}
	if diff, ok := execCtx.GetVariable("difficulty"); !ok || diff != "medium" {
		t.Errorf("difficulty should be loaded, got %v", diff)
	}
}

func TestStorageHook_OnNodeComplete_SaveOutput(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
	}, "coding-agent", "coding-agent")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "coding-agent"})
	execCtx.SetVariable("input", "write a sort function")

	output := &NodeOutput{Content: "Here is the sort function:\n```go\nfunc sort(arr []int) {}\n```"}

	hook.OnNodeComplete(context.Background(), "generate", output, execCtx)

	// 验证节点输出已保存
	outputKey := "pipeline:coding-agent:node:generate:output"
	savedOutput, err := kv.GetString(context.Background(), outputKey)
	if err != nil {
		t.Fatalf("output not saved: %v", err)
	}
	if savedOutput != output.Content {
		t.Errorf("saved output = %q, want %q", savedOutput, output.Content)
	}

	// 验证对话历史已保存
	historyKey := "pipeline:coding-agent:history"
	historyData, err := kv.GetBytes(context.Background(), historyKey)
	if err != nil {
		t.Fatalf("history not saved: %v", err)
	}
	var history []ConversationTurn
	if err := json.Unmarshal(historyData, &history); err != nil {
		t.Fatalf("failed to unmarshal history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Input != "write a sort function" {
		t.Errorf("history input = %q, want %q", history[0].Input, "write a sort function")
	}
	if history[0].NodeID != "generate" {
		t.Errorf("history nodeID = %q, want %q", history[0].NodeID, "generate")
	}
}

func TestStorageHook_OnNodeComplete_SaveCodeSnippets(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveCodeSnippets: true,
	}, "coding-agent", "coding-agent")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "coding-agent"})
	execCtx.SetVariable("input", "write quicksort")

	output := &NodeOutput{
		Content: "Here is quicksort:\n```go\nfunc quicksort(arr []int) {\n    // impl\n}\n```\nAnd here is a test:\n```go\nfunc TestSort(t *testing.T) {\n    // test\n}\n```",
	}

	hook.OnNodeComplete(context.Background(), "generate", output, execCtx)

	// 验证代码片段已保存
	snippetsKey := "pipeline:coding-agent:code_snippets"
	snippetsData, err := kv.GetBytes(context.Background(), snippetsKey)
	if err != nil {
		t.Fatalf("code snippets not saved: %v", err)
	}
	var snippets []CodeSnippet
	if err := json.Unmarshal(snippetsData, &snippets); err != nil {
		t.Fatalf("failed to unmarshal snippets: %v", err)
	}
	if len(snippets) != 2 {
		t.Fatalf("expected 2 code snippets, got %d", len(snippets))
	}
	if snippets[0].Language != "go" {
		t.Errorf("snippet language = %q, want %q", snippets[0].Language, "go")
	}
	if !stringContains(snippets[0].Code, "quicksort") {
		t.Errorf("snippet should contain quicksort, got %q", snippets[0].Code)
	}
}

func TestStorageHook_OnPipelineComplete(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveUserProgress: true,
		SaveSceneContext: true,
	}, "education-scene", "education-scene")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "education-scene"})
	execCtx.SetVariable("user_id", "user123")
	execCtx.SetVariable("scene", "problem_solving")
	execCtx.SetResult("problem-solver", &NodeOutput{Content: "solution here"})

	hook.OnPipelineComplete(context.Background(), execCtx)

	// 验证执行日志已保存
	keys, _ := kv.Keys(context.Background(), "")
	foundExecLog := false
	for _, k := range keys {
		if len(k) > len("pipeline:education-scene:execution:") &&
			k[:len("pipeline:education-scene:execution:")] == "pipeline:education-scene:execution:" {
			foundExecLog = true
			break
		}
	}
	if !foundExecLog {
		t.Error("execution log should have been saved")
	}

	// 验证用户进度已保存
	progressKey := "pipeline:education-scene:user:user123:progress"
	progressData, err := kv.GetBytes(context.Background(), progressKey)
	if err != nil {
		t.Fatalf("user progress not saved: %v", err)
	}
	var progress map[string]interface{}
	if err := json.Unmarshal(progressData, &progress); err != nil {
		t.Fatalf("failed to unmarshal progress: %v", err)
	}
	if progress["pipeline_id"] != "education-scene" {
		t.Errorf("progress pipeline_id = %v, want education-scene", progress["pipeline_id"])
	}

	// 验证场景上下文已保存
	sceneKey := "pipeline:education-scene:scene_context"
	sceneData, err := kv.GetBytes(context.Background(), sceneKey)
	if err != nil {
		t.Fatalf("scene context not saved: %v", err)
	}
	var sceneCtx map[string]interface{}
	if err := json.Unmarshal(sceneData, &sceneCtx); err != nil {
		t.Fatalf("failed to unmarshal scene context: %v", err)
	}
	if sceneCtx["scene"] != "problem_solving" {
		t.Errorf("scene = %v, want problem_solving", sceneCtx["scene"])
	}
}

func TestExtractCodeSnippets(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		languages []string
	}{
		{
			name:     "single go snippet",
			content:  "```go\npackage main\nfunc main() {}\n```",
			expected: 1,
			languages: []string{"go"},
		},
		{
			name:     "no code blocks",
			content:  "Just some text without code.",
			expected: 0,
		},
		{
			name:     "multiple snippets",
			content:  "```python\nprint('hello')\n```\n```javascript\nconsole.log('hi');\n```",
			expected: 2,
			languages: []string{"python", "javascript"},
		},
		{
			name:     "no language specified",
			content:  "```\nplain text\n```",
			expected: 1,
			languages: []string{""},
		},
		{
			name:     "incomplete code block",
			content:  "```go\nunclosed block",
			expected: 0,
		},
		{
			name:     "malformed blocks - backticks inside code block",
			content:  "```\n```\n```\n```go\ncode\n```",
			expected: 1,
			languages: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippets := extractCodeSnippets(tt.content)
			if len(snippets) != tt.expected {
				t.Errorf("got %d snippets, expected %d", len(snippets), tt.expected)
			}
			for i, lang := range tt.languages {
				if i < len(snippets) && snippets[i].Language != lang {
					t.Errorf("snippet[%d] language = %q, want %q", i, snippets[i].Language, lang)
				}
			}
		})
	}
}

func TestGlobalPipelineConfig_HasStorageHook(t *testing.T) {
	tests := []struct {
		name   string
		config GlobalPipelineConfig
		want   bool
	}{
		{
			name:   "no storage config",
			config: GlobalPipelineConfig{},
			want:   false,
		},
		{
			name: "storage enabled but no hooks",
			config: GlobalPipelineConfig{
				StorageConfig: &StorageHookConfig{Enabled: true},
			},
			want: false,
		},
		{
			name: "storage enabled with storage hook",
			config: GlobalPipelineConfig{
				StorageConfig: &StorageHookConfig{Enabled: true},
				Hooks: []HookConfig{
					{Type: "storage", On: []string{"node_complete"}},
				},
			},
			want: true,
		},
		{
			name: "storage disabled with hooks",
			config: GlobalPipelineConfig{
				StorageConfig: &StorageHookConfig{Enabled: false},
				Hooks: []HookConfig{
					{Type: "storage", On: []string{"node_complete"}},
				},
			},
			want: false,
		},
		{
			name: "storage enabled with other hooks only",
			config: GlobalPipelineConfig{
				StorageConfig: &StorageHookConfig{Enabled: true},
				Hooks: []HookConfig{
					{Type: "monitoring", On: []string{"pipeline_complete"}},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.HasStorageHook(); got != tt.want {
				t.Errorf("HasStorageHook() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobalPipelineConfig_StorageNamespace(t *testing.T) {
	config := GlobalPipelineConfig{
		StorageConfig: &StorageHookConfig{
			Enabled:   true,
			Namespace: "custom-ns",
		},
	}
	if got := config.StorageNamespace("fallback-id"); got != "custom-ns" {
		t.Errorf("StorageNamespace() = %q, want %q", got, "custom-ns")
	}

	config2 := GlobalPipelineConfig{
		StorageConfig: &StorageHookConfig{
			Enabled:   true,
			Namespace: "",
		},
	}
	if got := config2.StorageNamespace("fallback-id"); got != "fallback-id" {
		t.Errorf("StorageNamespace() with empty ns = %q, want %q", got, "fallback-id")
	}

	config3 := GlobalPipelineConfig{}
	if got := config3.StorageNamespace("fallback-id"); got != "fallback-id" {
		t.Errorf("StorageNamespace() with nil config = %q, want %q", got, "fallback-id")
	}
}

func TestStorageHook_ConversationHistoryLimit(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
	}, "test", "test")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})

	// 写入 110 轮对话（超过 100 限制）
	for i := 0; i < 110; i++ {
		execCtx.SetVariable("input", fmt.Sprintf("question %d", i))
		output := &NodeOutput{Content: fmt.Sprintf("answer %d", i)}
		hook.OnNodeComplete(context.Background(), fmt.Sprintf("node_%d", i), output, execCtx)
	}

	// 验证历史被限制在 100 条
	historyKey := "pipeline:test:history"
	historyData, err := kv.GetBytes(context.Background(), historyKey)
	if err != nil {
		t.Fatalf("history not saved: %v", err)
	}
	var history []ConversationTurn
	if err := json.Unmarshal(historyData, &history); err != nil {
		t.Fatalf("failed to unmarshal history: %v", err)
	}
	if len(history) > 100 {
		t.Errorf("history should be limited to 100, got %d", len(history))
	}
	// 应保留最新的 100 条
	if history[len(history)-1].Input != "question 109" {
		t.Errorf("last entry should be question 109, got %v", history[len(history)-1].Input)
	}
}

func TestStorageHook_SaveSolutions(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveSolutions: true,
	}, "coding-agent", "coding-agent")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "coding-agent"})
	execCtx.SetVariable("input", "create a REST API")

	output := &NodeOutput{Content: "Here is your REST API code"}

	hook.OnNodeComplete(context.Background(), "generate", output, execCtx)

	solutionsKey := "pipeline:coding-agent:solutions"
	solutionsData, err := kv.GetBytes(context.Background(), solutionsKey)
	if err != nil {
		t.Fatalf("solutions not saved: %v", err)
	}
	var solutions []CodingSolution
	if err := json.Unmarshal(solutionsData, &solutions); err != nil {
		t.Fatalf("failed to unmarshal solutions: %v", err)
	}
	if len(solutions) != 1 {
		t.Fatalf("expected 1 solution, got %d", len(solutions))
	}
	if solutions[0].TaskDescription != "create a REST API" {
		t.Errorf("task description = %q, want %q", solutions[0].TaskDescription, "create a REST API")
	}
	if !solutions[0].Success {
		t.Error("solution should be marked as success")
	}
}

func TestStorageHook_NilOutput(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
	}, "test", "test")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "test"})

	// nil output 不应 panic
	hook.OnNodeComplete(context.Background(), "node1", nil, execCtx)

	// 验证没有错误存储
	historyKey := "pipeline:test:history"
	_, err := kv.GetBytes(context.Background(), historyKey)
	if err == nil {
		t.Error("history should not be saved for nil output")
	}
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstr(s, substr)
}

func indexOfSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// 并发安全性测试 (Task 3.1)
// ============================================================================

func TestStorageHook_ConcurrentAccess(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
		SaveCodeSnippets:        true,
		SaveSolutions:           true,
	}, "concurrent-test", "concurrent-test")

	workers := 10
	opsPerWorker := 20
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "concurrent-test"})
				execCtx.SetVariable("input", fmt.Sprintf("query-%d-%d", id, j))
				hook.OnNodeStart(context.Background(), fmt.Sprintf("node_%d", j), execCtx)
				hook.OnNodeComplete(context.Background(), fmt.Sprintf("node_%d", j),
					&NodeOutput{Content: fmt.Sprintf("```go\nfunc f%d_%d() {}\n```", id, j)}, execCtx)
			}
		}(i)
	}
	wg.Wait()

	// 验证历史记录完整性
	historyKey := "pipeline:concurrent-test:history"
	historyData, err := kv.GetBytes(context.Background(), historyKey)
	if err != nil {
		t.Fatalf("history not saved: %v", err)
	}
	var history []ConversationTurn
	if err := json.Unmarshal(historyData, &history); err != nil {
		t.Fatalf("unmarshal history: %v", err)
	}
	if len(history) == 0 {
		t.Error("concurrent operations should produce history entries")
	}
	t.Logf("concurrent test: %d workers × %d ops → %d history entries",
		workers, opsPerWorker, len(history))
}

// ============================================================================
// 边界条件和异常场景测试 (Task 3.1)
// ============================================================================

func TestStorageHook_ZeroRetentionDays(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 0}, HookBehaviorConfig{
		SaveConversationHistory: true,
	}, "zero-ttl", "zero-ttl")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "zero-ttl"})
	execCtx.SetVariable("input", "test input")
	hook.OnNodeComplete(context.Background(), "node1", &NodeOutput{Content: "output"}, execCtx)

	// 0 意味着永不过期，数据应正常保存
	historyKey := "pipeline:zero-ttl:history"
	_, err := kv.GetBytes(context.Background(), historyKey)
	if err != nil {
		t.Fatalf("zero TTL should still save data: %v", err)
	}
}

func TestStorageHook_NegativeRetentionDays(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: -1}, HookBehaviorConfig{
		SaveConversationHistory: true,
	}, "neg-ttl", "neg-ttl")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "neg-ttl"})
	execCtx.SetVariable("input", "test")
	hook.OnNodeComplete(context.Background(), "n1", &NodeOutput{Content: "out"}, execCtx)

	// 负值也按永不过期处理
	historyKey := "pipeline:neg-ttl:history"
	_, err := kv.GetBytes(context.Background(), historyKey)
	if err != nil {
		t.Fatalf("negative TTL should still save: %v", err)
	}
}

func TestStorageHook_EmptyOutputContent(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
		SaveCodeSnippets:        true,
	}, "empty-out", "empty-out")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "empty-out"})
	execCtx.SetVariable("input", "query")
	// 空内容的输出
	hook.OnNodeComplete(context.Background(), "node1", &NodeOutput{Content: ""}, execCtx)

	// 空内容不应保存节点输出
	outputKey := "pipeline:empty-out:node:node1:output"
	_, err := kv.GetBytes(context.Background(), outputKey)
	if err == nil {
		t.Error("empty output content should not be saved")
	}

	// 但对话历史应该保存（因为配置了 SaveConversationHistory）
	historyKey := "pipeline:empty-out:history"
	historyData, err := kv.GetBytes(context.Background(), historyKey)
	if err != nil {
		t.Fatalf("history should be saved even with empty output: %v", err)
	}
	var history []ConversationTurn
	json.Unmarshal(historyData, &history)
	if len(history) == 0 {
		t.Error("history should have entry for empty output")
	}
}

func TestStorageHook_VeryLargeOutput(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
	}, "large-out", "large-out")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "large-out"})
	execCtx.SetVariable("input", "generate large output")

	// 10KB 输出
	largeContent := strings.Repeat("x", 10240)
	output := &NodeOutput{Content: largeContent}

	hook.OnNodeComplete(context.Background(), "node1", output, execCtx)

	outputKey := "pipeline:large-out:node:node1:output"
	saved, err := kv.GetString(context.Background(), outputKey)
	if err != nil {
		t.Fatalf("large output not saved: %v", err)
	}
	if len(saved) != len(largeContent) {
		t.Errorf("saved length = %d, want %d", len(saved), len(largeContent))
	}
}

func TestStorageHook_SaveUserProgress_NoUserID(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveUserProgress: true,
	}, "no-user", "no-user")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "no-user"})
	// 不设置 user_id
	hook.OnPipelineComplete(context.Background(), execCtx)

	// 无 user_id 时不保存进度
	keys, _ := kv.Keys(context.Background(), "")
	for _, k := range keys {
		if strings.Contains(k, "/user:") || strings.HasPrefix(k, "pipeline:no-user:user:") {
			t.Errorf("should not save progress without user_id, found key: %s", k)
		}
	}
}

func TestStorageHook_MultipleHookConfigs(t *testing.T) {
	// 测试 pipeline 配置多个 hook（非 storage 类型）时正确提取 storage hook
	pipeline := &AgentPatternPipeline{
		ID: "multi-hook",
		GlobalConfig: GlobalPipelineConfig{
			StorageConfig: &StorageHookConfig{Enabled: true},
			Hooks: []HookConfig{
				{Type: "monitoring", On: []string{"pipeline_complete"}},
				{Type: "storage", On: []string{"node_complete"}, Config: map[string]interface{}{
					"save_user_progress": true,
					"save_solutions":     true,
				}},
				{Type: "logging", On: []string{"node_start"}},
			},
		},
	}

	hook := NewStorageHook(nil, pipeline, nil)
	if !hook.hookCfg.SaveUserProgress {
		t.Error("SaveUserProgress should be true from storage hook config")
	}
	if !hook.hookCfg.SaveSolutions {
		t.Error("SaveSolutions should be true")
	}
	// 未配置的应为 false
	if hook.hookCfg.SaveConversationHistory {
		t.Error("SaveConversationHistory should be false")
	}
}

func TestStorageHook_OnPipelineComplete_EmptyResults(t *testing.T) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveUserProgress:        true,
		SaveConversationHistory: true,
	}, "empty-results", "empty-results")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "empty-results"})
	execCtx.SetVariable("user_id", "user1")
	// 不设置任何结果
	hook.OnPipelineComplete(context.Background(), execCtx)

	// 应该正常完成，不 panic
	progressKey := "pipeline:empty-results:user:user1:progress"
	_, err := kv.GetBytes(context.Background(), progressKey)
	if err != nil {
		t.Fatalf("progress should be saved even without results: %v", err)
	}
}

func TestExtractCodeSnippets_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "nested backticks",
			content:  "```go\ncode with `backtick` inside\n```",
			expected: 1,
		},
		{
			name:     "indented code block",
			content:  "  ```python\n  print('hello')\n  ```",
			expected: 1,
		},
		{
			name:     "only opening fence",
			content:  "```go\nfunc main() {",
			expected: 0,
		},
		{
			name:     "multiple languages in one block",
			content:  "```go\ncode\n```\n```js\nmore code\n```",
			expected: 2,
		},
		{
			name:     "empty code block",
			content:  "```python\n```",
			expected: 0,
		},
		{
			name:     "code block with only whitespace",
			content:  "```go\n   \n```",
			expected: 0,
		},
		{
			name:     "code block right after text",
			content:  "text\n```go\ncode\n```\nmore text",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippets := extractCodeSnippets(tt.content)
			if len(snippets) != tt.expected {
				t.Errorf("extractCodeSnippets(%q) = %d snippets, want %d",
					tt.content, len(snippets), tt.expected)
			}
		})
	}
}

// ============================================================================
// Benchmark (Task 3.1 性能测试)
// ============================================================================

func BenchmarkStorageHook_OnNodeComplete(b *testing.B) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveConversationHistory: true,
		SaveCodeSnippets:        true,
		SaveSolutions:           true,
	}, "bench", "bench")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "bench"})
	execCtx.SetVariable("input", "write a function")
	output := &NodeOutput{Content: "```go\nfunc bench() {}\n```"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hook.OnNodeComplete(context.Background(), "node1", output, execCtx)
	}
}

func BenchmarkStorageHook_OnPipelineComplete(b *testing.B) {
	kv := newMockKVStore()
	hook := newStorageHookWithKVStore(kv, StorageHookConfig{Enabled: true, RetentionDays: 30}, HookBehaviorConfig{
		SaveUserProgress: true,
		SaveSceneContext: true,
	}, "bench", "bench")

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "bench"})
	execCtx.SetVariable("user_id", "user-bench")
	execCtx.SetVariable("scene", "coding")
	execCtx.SetResult("n1", &NodeOutput{Content: "result"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hook.OnPipelineComplete(context.Background(), execCtx)
	}
}

func BenchmarkExtractCodeSnippets(b *testing.B) {
	content := "Here is code:\n```go\npackage main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n"
	for i := 0; i < b.N; i++ {
		extractCodeSnippets(content)
	}
}

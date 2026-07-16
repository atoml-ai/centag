package loader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"centag/core/pkg/pipeline"
)

// testPlugin 模拟插件实例
type testPlugin struct{}

func (p *testPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:        "test-plugin",
		Kind:        "test",
		Version:     "1.0.0",
		Permissions: []string{},
	}
}

func (p *testPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	return nil
}

func (p *testPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: "test output",
		},
	}, nil
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from  PluginState
		to    PluginState
		valid bool
	}{
		{StateUnknown, StateLoading, true},
		{StateUnknown, StateRunning, false},
		{StateLoading, StateLoaded, true},
		{StateLoading, StateError, true},
		{StateLoaded, StateValidating, true},
		{StateLoaded, StateUnloading, true},
		{StateValidating, StateValidated, true},
		{StateValidating, StateError, true},
		{StateValidated, StateStarting, true},
		{StateValidated, StateUnloading, true},
		{StateStarting, StateRunning, true},
		{StateStarting, StateError, true},
		{StateRunning, StateStopping, true},
		{StateRunning, StateUnloading, true},
		{StateStopping, StateStopped, true},
		{StateStopping, StateError, true},
		{StateStopped, StateStarting, true},
		{StateStopped, StateUnloading, true},
		{StateUnloading, StateUnloaded, true},
		{StateUnloading, StateError, true},
		{StateError, StateLoading, true},
		{StateError, StateUnloading, true},
	}

	for _, tt := range tests {
		result := IsValidTransition(tt.from, tt.to)
		if result != tt.valid {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.valid)
		}
	}
}

func TestDefaultLoader_Load(t *testing.T) {
	loader := NewDefaultLoader("/tmp/plugins", nil)
	ctx := context.Background()

	// 测试无效来源
	req := &LoadRequest{
		Source: "invalid",
	}

	_, err := loader.Load(ctx, req)
	if err == nil {
		t.Error("Expected error for invalid source")
	}
}

func TestDefaultLoader_GetList(t *testing.T) {
	loader := NewDefaultLoader("/tmp/plugins", nil)

	// 初始为空
	list := loader.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d items", len(list))
	}

	// 获取不存在的插件
	_, err := loader.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
}

func TestDefaultLoader_StateTransitions(t *testing.T) {
	loader := NewDefaultLoader("/tmp/plugins", nil)

	// 创建模拟插件
	managed := &ManagedPlugin{
		ID:      "test-plugin",
		State:   StateUnknown,
		Version: "1.0.0",
	}

	loader.mu.Lock()
	loader.plugins[managed.ID] = managed
	loader.mu.Unlock()

	// 测试状态转换
	tests := []struct {
		from PluginState
		to   PluginState
		ok   bool
	}{
		{StateUnknown, StateLoading, true},
		{StateLoading, StateLoaded, true},
		{StateLoaded, StateValidating, true},
		{StateValidating, StateValidated, true},
		{StateValidated, StateStarting, true},
		{StateStarting, StateRunning, true},
	}

	for _, tt := range tests {
		managed.State = tt.from
		err := loader.transitionState(managed, tt.to)

		if tt.ok && err != nil {
			t.Errorf("Expected successful transition from %s to %s, got error: %v", tt.from, tt.to, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("Expected error for transition from %s to %s", tt.from, tt.to)
		}
	}
}

func TestDefaultLoader_Listeners(t *testing.T) {
	loader := NewDefaultLoader("/tmp/plugins", nil)

	// 创建监听器
	listener := &testListener{
		events: make([]*LifecycleEvent, 0),
	}

	// 添加监听器
	loader.AddListener(listener)

	// 创建插件并触发状态转换
	managed := &ManagedPlugin{
		ID:      "test-plugin",
		State:   StateUnknown,
		Version: "1.0.0",
	}

	loader.mu.Lock()
	loader.plugins[managed.ID] = managed
	loader.mu.Unlock()

	// 触发状态转换
	loader.transitionState(managed, StateLoading)

	// 等待事件处理
	time.Sleep(100 * time.Millisecond)

	// 验证监听器收到事件
	if len(listener.events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(listener.events))
	}

	// 移除监听器
	loader.RemoveListener(listener)
}

type testListener struct {
	events []*LifecycleEvent
	mu     sync.Mutex
}

func (l *testListener) OnStateChanged(event *LifecycleEvent) {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
}

func TestDefaultLoader_StartStop(t *testing.T) {
	loader := NewDefaultLoader("/tmp/plugins", nil)

	// 创建插件 - 内置插件类型，不需要实际启动
	managed := &ManagedPlugin{
		ID:      "test-plugin",
		State:   StateValidated,
		Version: "1.0.0",
		Manifest: &PluginManifest{
			ID:         "test-plugin",
			Name:       "Test Plugin",
			Version:    "1.0.0",
			Runtime:    "builtin", // 内置插件，不需要实际启动
			Entrypoint: "",
		},
		Instance: &testPlugin{}, // 模拟已加载的实例
	}

	loader.mu.Lock()
	loader.plugins[managed.ID] = managed
	loader.mu.Unlock()

	// 启动
	err := loader.Start(managed.ID)
	if err != nil {
		t.Errorf("Failed to start plugin: %v", err)
	}

	if managed.State != StateRunning {
		t.Errorf("Expected state Running, got %s", managed.State)
	}

	// 停止
	err = loader.Stop(managed.ID)
	if err != nil {
		t.Errorf("Failed to stop plugin: %v", err)
	}

	if managed.State != StateStopped {
		t.Errorf("Expected state Stopped, got %s", managed.State)
	}
}

func TestDefaultLoader_StartUnsupportedRuntimeDoesNotMarkRunning(t *testing.T) {
	pluginDir := t.TempDir()
	pluginID := "go-plugin"
	pluginPath := filepath.Join(pluginDir, pluginID, "plugin.so")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.WriteFile(pluginPath, []byte("not-a-real-plugin"), 0644); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}

	loader := NewDefaultLoader(pluginDir, nil)
	managed := &ManagedPlugin{
		ID:      pluginID,
		State:   StateValidated,
		Version: "1.0.0",
		Manifest: &PluginManifest{
			ID:         pluginID,
			Name:       "Go Plugin",
			Version:    "1.0.0",
			Runtime:    "go",
			Entrypoint: "plugin.so",
		},
	}

	loader.mu.Lock()
	loader.plugins[managed.ID] = managed
	loader.mu.Unlock()

	err := loader.Start(managed.ID)
	if err == nil {
		t.Fatal("expected unsupported runtime error")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error, got %v", err)
	}
	if managed.State == StateRunning {
		t.Fatal("unsupported runtime must not be marked running")
	}
	if managed.State != StateError {
		t.Fatalf("expected state error, got %s", managed.State)
	}
	if managed.LastError == "" {
		t.Fatal("expected LastError to be recorded")
	}
}

func TestDefaultLoader_Update(t *testing.T) {
	loader := NewDefaultLoader("/tmp/plugins", nil)
	ctx := context.Background()

	// 创建插件
	managed := &ManagedPlugin{
		ID:      "test-plugin",
		State:   StateRunning,
		Version: "1.0.0",
	}

	loader.mu.Lock()
	loader.plugins[managed.ID] = managed
	loader.mu.Unlock()

	// 更新请求
	req := &UpdateRequest{
		PluginID:        "test-plugin",
		NewVersion:      "2.0.0",
		Strategy:        StrategyBlueGreen,
		RollbackOnError: true,
	}

	status, err := loader.Update(ctx, req)
	if err != nil {
		t.Errorf("Failed to update plugin: %v", err)
	}

	if status.PluginID != "test-plugin" {
		t.Errorf("Expected plugin ID test-plugin, got %s", status.PluginID)
	}

	if status.FromVersion != "1.0.0" {
		t.Errorf("Expected from version 1.0.0, got %s", status.FromVersion)
	}

	if status.ToVersion != "2.0.0" {
		t.Errorf("Expected to version 2.0.0, got %s", status.ToVersion)
	}

	if status.Strategy != StrategyBlueGreen {
		t.Errorf("Expected strategy blue-green, got %s", status.Strategy)
	}
}

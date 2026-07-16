package proxymode

import (
	"testing"

	"centag/core/pkg/pipeline"
)

func TestModeFromPipeline(t *testing.T) {
	p := &pipeline.AgentPatternPipeline{
		ID:           "cache-hit",
		Name:         "缓存优先",
		Description:  "先读缓存",
		ShortcutCode: "#ch",
		Metadata: map[string]interface{}{
			"aligned_proxy_mode": "cache-hit",
		},
		Nodes: []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
	}

	mode, ok := ModeFromPipeline(p)
	if !ok {
		t.Fatal("expected ModeFromPipeline ok")
	}
	if mode.Key != "#ch" {
		t.Fatalf("key = %q, want #ch", mode.Key)
	}
	if mode.Type != "cache-hit" {
		t.Fatalf("type = %q, want cache-hit", mode.Type)
	}
	if pid, _ := mode.Config["pipeline_id"].(string); pid != "cache-hit" {
		t.Fatalf("pipeline_id = %q", pid)
	}
}

func TestModeFromPipeline_NoShortcut(t *testing.T) {
	p := &pipeline.AgentPatternPipeline{
		ID:   "no-shortcut",
		Name: "Test",
		Nodes: []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
	}
	_, ok := ModeFromPipeline(p)
	if ok {
		t.Fatal("expected false without shortcut_code")
	}
}

func TestSyncFromPipelines(t *testing.T) {
	mgr := NewManager()

	pipelines := []*pipeline.AgentPatternPipeline{
		{
			ID:           "cache-hit",
			Name:         "缓存优先",
			Description:  "先读缓存",
			ShortcutCode: "#ch",
			Metadata:     map[string]interface{}{"aligned_proxy_mode": "cache-hit"},
			Nodes:        []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
		},
		{
			ID:           "custom-flow",
			Name:         "自定义",
			Description:  "用户自定义流水线",
			ShortcutCode: "#custom",
			Nodes:        []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
		},
	}

	n := mgr.SyncFromPipelines(pipelines)
	if n != 2 {
		t.Fatalf("synced = %d, want 2", n)
	}

	mode, exists := mgr.GetMode("#custom")
	if !exists {
		t.Fatal("#custom should be registered")
	}
	if mode.Name != "自定义" {
		t.Fatalf("name = %q", mode.Name)
	}
	if mgr.PipelineIDForShortcut("#custom") != "custom-flow" {
		t.Fatalf("pipeline_id = %q", mgr.PipelineIDForShortcut("#custom"))
	}

	// 受保护内置模式应被流水线数据更新
	ch, exists := mgr.GetMode("#ch")
	if !exists {
		t.Fatal("#ch should exist")
	}
	if ch.Name != "缓存优先" {
		t.Fatalf("#ch name = %q, want 缓存优先", ch.Name)
	}
}

func TestSyncFromPipelines_RemovesStaleCustom(t *testing.T) {
	mgr := NewManager()

	initial := []*pipeline.AgentPatternPipeline{
		{
			ID:           "temp",
			Name:         "临时",
			ShortcutCode: "#tmp",
			Nodes:        []pipeline.PipelineNodeConfig{{ID: "n1", Type: pipeline.NodeTypeGenerator, Backend: "b", Model: "m"}},
		},
	}
	mgr.SyncFromPipelines(initial)
	if _, ok := mgr.GetMode("#tmp"); !ok {
		t.Fatal("#tmp should exist after first sync")
	}

	mgr.SyncFromPipelines(nil)
	if _, ok := mgr.GetMode("#tmp"); ok {
		t.Fatal("#tmp should be removed after empty sync")
	}
	if _, ok := mgr.GetMode("#d"); !ok {
		t.Fatal("protected #d should remain")
	}
}

func TestRemovePipelineShortcut_Protected(t *testing.T) {
	mgr := NewManager()
	mgr.RemovePipelineShortcut("#d")
	if _, ok := mgr.GetMode("#d"); !ok {
		t.Fatal("protected #d should not be removed")
	}
}
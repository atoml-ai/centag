package pipeline

import (
	"testing"

	"centag/core/pkg/bootstrap"
)

// TestEducationPipelineStorageConfig 验证教育流水线模板中的存储配置
func TestEducationPipelineStorageConfig(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadEducationSceneTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)
	if p == nil {
		t.Fatal("CreatePipelineFromTemplate returned nil")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	gc := p.GlobalConfig

	// 1. 验证 HasStorageHook
	if !gc.HasStorageHook() {
		t.Fatal("HasStorageHook should return true for education-scene")
	}

	// 2. 验证 StorageConfig 基本字段
	sc := gc.StorageConfig
	if sc == nil {
		t.Fatal("StorageConfig is nil")
	}
	if !sc.Enabled {
		t.Error("StorageConfig.Enabled should be true")
	}
	if sc.AutoSave != true {
		t.Errorf("AutoSave = %v, want true", sc.AutoSave)
	}
	if sc.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", sc.RetentionDays)
	}

	// 3. 验证 StorageNamespace（config 设了 namespace）
	ns := gc.StorageNamespace(p.ID)
	if ns != "education-scene" {
		t.Errorf("StorageNamespace = %q, want %q", ns, "education-scene")
	}

	// 4. 验证 Hooks 配置
	if len(gc.Hooks) == 0 {
		t.Fatal("Hooks should not be empty")
	}
	storageHook := gc.Hooks[0]
	if storageHook.Type != "storage" {
		t.Errorf("hook type = %q, want %q", storageHook.Type, "storage")
	}

	// 验证钩子事件
	foundComplete := false
	for _, evt := range storageHook.On {
		if evt == "node_complete" || evt == "pipeline_complete" {
			foundComplete = true
		}
	}
	if !foundComplete {
		t.Errorf("hook should handle node_complete or pipeline_complete events, got %v", storageHook.On)
	}

	// 验证钩子行为配置
	if v, ok := storageHook.Config["save_user_progress"].(bool); !ok || !v {
		t.Error("save_user_progress should be true")
	}
	if v, ok := storageHook.Config["save_conversation_history"].(bool); !ok || !v {
		t.Error("save_conversation_history should be true")
	}
	if v, ok := storageHook.Config["save_scene_context"].(bool); !ok || !v {
		t.Error("save_scene_context should be true")
	}
}

// TestEducationPipelineCreateStorageHook 验证从教育流水线创建 StorageHook 实例
func TestEducationPipelineCreateStorageHook(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadEducationSceneTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)

	// 通过 NewStorageHook 创建（manager 为 nil → IsEnabled 为 false，但配置正确）
	hook := NewStorageHook(nil, p, nil)
	if hook == nil {
		t.Fatal("NewStorageHook returned nil")
	}

	// 验证配置已正确传递
	if hook.cfg.Enabled != true {
		t.Error("hook cfg.Enabled should be true")
	}
	if hook.namespace != "education-scene" {
		t.Errorf("hook namespace = %q, want %q", hook.namespace, "education-scene")
	}
	if hook.pipelineID != "education-scene" {
		t.Errorf("hook pipelineID = %q, want %q", hook.pipelineID, "education-scene")
	}

	// 验证教育场景专用的行为配置
	if !hook.hookCfg.SaveUserProgress {
		t.Error("SaveUserProgress should be true for education-scene")
	}
	if !hook.hookCfg.SaveConversationHistory {
		t.Error("SaveConversationHistory should be true for education-scene")
	}
	if !hook.hookCfg.SaveSceneContext {
		t.Error("SaveSceneContext should be true for education-scene")
	}
	if hook.hookCfg.SaveCodeSnippets {
		t.Error("SaveCodeSnippets should be false for education-scene")
	}
	if hook.hookCfg.SaveSolutions {
		t.Error("SaveSolutions should be false for education-scene")
	}
}

// TestCodingPipelineStorageConfig 验证编程流水线模板中的存储配置
func TestCodingPipelineStorageConfig(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadPipelineTemplate(t, "coding-agent")
	p := CreatePipelineFromTemplate(tmpl, nil)
	if p == nil {
		t.Fatal("CreatePipelineFromTemplate returned nil")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	gc := p.GlobalConfig

	// 1. 验证 HasStorageHook
	if !gc.HasStorageHook() {
		t.Fatal("HasStorageHook should return true for coding-agent")
	}

	// 2. 验证 StorageConfig 基本字段
	sc := gc.StorageConfig
	if sc == nil {
		t.Fatal("StorageConfig is nil")
	}
	if !sc.Enabled {
		t.Error("StorageConfig.Enabled should be true")
	}
	if sc.AutoSave != true {
		t.Errorf("AutoSave = %v, want true", sc.AutoSave)
	}
	if sc.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", sc.RetentionDays)
	}

	// 3. 验证 StorageNamespace
	ns := gc.StorageNamespace(p.ID)
	if ns != "coding-agent" {
		t.Errorf("StorageNamespace = %q, want %q", ns, "coding-agent")
	}

	// 4. 验证 Hooks 配置
	if len(gc.Hooks) == 0 {
		t.Fatal("Hooks should not be empty")
	}
	storageHook := gc.Hooks[0]
	if storageHook.Type != "storage" {
		t.Errorf("hook type = %q, want %q", storageHook.Type, "storage")
	}

	// 5. 验证钩子事件
	foundComplete := false
	for _, evt := range storageHook.On {
		if evt == "node_complete" || evt == "pipeline_complete" {
			foundComplete = true
		}
	}
	if !foundComplete {
		t.Errorf("hook should handle node_complete or pipeline_complete events, got %v", storageHook.On)
	}

	// 6. 验证钩子行为配置
	if v, ok := storageHook.Config["save_code_snippets"].(bool); !ok || !v {
		t.Error("save_code_snippets should be true")
	}
	if v, ok := storageHook.Config["save_solutions"].(bool); !ok || !v {
		t.Error("save_solutions should be true")
	}
	if v, ok := storageHook.Config["track_file_changes"].(bool); !ok || !v {
		t.Error("track_file_changes should be true")
	}
}

// TestCodingPipelineCreateStorageHook 验证从编程流水线创建 StorageHook 实例
func TestCodingPipelineCreateStorageHook(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadPipelineTemplate(t, "coding-agent")
	p := CreatePipelineFromTemplate(tmpl, nil)

	hook := NewStorageHook(nil, p, nil)
	if hook == nil {
		t.Fatal("NewStorageHook returned nil")
	}

	// 验证配置
	if hook.cfg.Enabled != true {
		t.Error("hook cfg.Enabled should be true")
	}
	if hook.namespace != "coding-agent" {
		t.Errorf("hook namespace = %q, want %q", hook.namespace, "coding-agent")
	}
	if hook.pipelineID != "coding-agent" {
		t.Errorf("hook pipelineID = %q, want %q", hook.pipelineID, "coding-agent")
	}

	// 验证编程场景专用的行为配置
	if !hook.hookCfg.SaveCodeSnippets {
		t.Error("SaveCodeSnippets should be true for coding-agent")
	}
	if !hook.hookCfg.SaveSolutions {
		t.Error("SaveSolutions should be true for coding-agent")
	}
	if !hook.hookCfg.TrackFileChanges {
		t.Error("TrackFileChanges should be true for coding-agent")
	}
	if hook.hookCfg.SaveUserProgress {
		t.Error("SaveUserProgress should be false for coding-agent")
	}
	if hook.hookCfg.SaveConversationHistory {
		t.Error("SaveConversationHistory should be false for coding-agent")
	}
	if hook.hookCfg.SaveSceneContext {
		t.Error("SaveSceneContext should be false for coding-agent")
	}
}

// TestPipelineTemplateLoading_AllTemplatesWithStorageConfig 扫描所有模板，
// 验证带有 storage 配置的模板可正确解析
func TestPipelineTemplateLoading_AllTemplatesWithStorageConfig(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))

	allTemplates := bootstrap.LoadInitialPipelineTemplatesFromFiles()
	if len(allTemplates) == 0 {
		t.Fatal("no pipeline templates loaded")
	}

	var storageEnabled []string
	for _, raw := range allTemplates {
		tmpl := convertBootstrapTemplate(raw)
		if tmpl.GlobalConfig != nil && tmpl.GlobalConfig.HasStorageHook() {
			storageEnabled = append(storageEnabled, raw.ID)
		}
	}

	// Builtin set may only ship coding-agent with storage; education-scene lives in external business repos.
	if len(storageEnabled) < 1 {
		t.Errorf("expected at least 1 template with storage enabled, got %d: %v", len(storageEnabled), storageEnabled)
	}

	t.Logf("Templates with storage hooks enabled (%d): %v", len(storageEnabled), storageEnabled)
}

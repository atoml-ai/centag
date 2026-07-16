package scheduler

import (
	"os"
	"path/filepath"
	"testing"

	"centag/core/pkg/logger"
)

func init() {
	// 初始化日志
	logger.Init(logger.Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})
}

func TestFastMatcherConfigManager_DefaultConfig(t *testing.T) {
	manager := NewFastMatcherConfigManager("")

	config := manager.GetConfig()
	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}

	// 验证默认配置
	if len(config.CategoryToTask) == 0 {
		t.Error("CategoryToTask is empty")
	}

	if len(config.TaskToBackends) == 0 {
		t.Error("TaskToBackends is empty")
	}

	// 验证特定映射
	if taskType, ok := config.CategoryToTask["code"]; !ok || taskType != TaskCodeGeneration {
		t.Errorf("CategoryToTask['code'] = %v, want TaskCodeGeneration", taskType)
	}
}

func TestFastMatcherConfigManager_LoadSave(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "fast_matcher_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 创建管理器并保存配置
	manager := NewFastMatcherConfigManager(configPath)
	config := manager.GetConfig()

	// 添加自定义映射
	config.CategoryToTask["custom-task"] = TaskCodeGeneration
	config.TaskToBackends[TaskCodeGeneration] = append(config.TaskToBackends[TaskCodeGeneration],
		BackendRecommendationConfig{
			BackendID: "custom-backend",
			Model:     "custom-model",
			Priority:  99,
			Reason:    "自定义后端",
		},
	)

	// 保存配置
	if err := manager.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file not created: %s", configPath)
	}

	// 创建新管理器并加载配置
	newManager := NewFastMatcherConfigManager(configPath)
	if err := newManager.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	newConfig := newManager.GetConfig()

	// 验证加载的配置
	if taskType, ok := newConfig.CategoryToTask["custom-task"]; !ok || taskType != TaskCodeGeneration {
		t.Errorf("Loaded CategoryToTask['custom-task'] = %v, want TaskCodeGeneration", taskType)
	}

	found := false
	for _, rec := range newConfig.TaskToBackends[TaskCodeGeneration] {
		if rec.BackendID == "custom-backend" && rec.Model == "custom-model" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Custom backend recommendation not found in loaded config")
	}
}

func TestFastMatcherConfigManager_UpdateConfig(t *testing.T) {
	manager := NewFastMatcherConfigManager("")

	// 更新配置
	newConfig := &FastMatcherConfig{
		CategoryToTask: map[string]TaskType{
			"test": TaskTranslation,
		},
		TaskToBackends: map[TaskType][]BackendRecommendationConfig{
			TaskTranslation: {
				{BackendID: "test-backend", Model: "test-model", Priority: 1, Reason: "测试"},
			},
		},
	}

	manager.UpdateConfig(newConfig)

	// 验证更新
	config := manager.GetConfig()
	if len(config.CategoryToTask) != 1 {
		t.Errorf("CategoryToTask length = %d, want 1", len(config.CategoryToTask))
	}

	if taskType, ok := config.CategoryToTask["test"]; !ok || taskType != TaskTranslation {
		t.Errorf("CategoryToTask['test'] = %v, want TaskTranslation", taskType)
	}
}

func TestFastMatcherConfigManager_AddRemoveCategoryMapping(t *testing.T) {
	manager := NewFastMatcherConfigManager("")

	// 添加映射
	manager.AddCategoryMapping("new-category", TaskCodeGeneration)

	config := manager.GetConfig()
	if taskType, ok := config.CategoryToTask["new-category"]; !ok || taskType != TaskCodeGeneration {
		t.Errorf("AddCategoryMapping failed: CategoryToTask['new-category'] = %v", taskType)
	}

	// 移除映射
	manager.RemoveCategoryMapping("new-category")

	config = manager.GetConfig()
	if _, ok := config.CategoryToTask["new-category"]; ok {
		t.Error("RemoveCategoryMapping failed: category still exists")
	}
}

func TestFastMatcherConfigManager_AddRemoveBackendRecommendation(t *testing.T) {
	manager := NewFastMatcherConfigManager("")

	// 添加推荐
	manager.AddBackendRecommendation(TaskCodeGeneration, BackendRecommendationConfig{
		BackendID: "new-backend",
		Model:     "new-model",
		Priority:  10,
		Reason:    "新后端",
	})

	config := manager.GetConfig()
	found := false
	for _, rec := range config.TaskToBackends[TaskCodeGeneration] {
		if rec.BackendID == "new-backend" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AddBackendRecommendation failed: recommendation not found")
	}

	// 移除推荐
	manager.RemoveBackendRecommendation(TaskCodeGeneration, "new-backend")

	config = manager.GetConfig()
	for _, rec := range config.TaskToBackends[TaskCodeGeneration] {
		if rec.BackendID == "new-backend" {
			t.Error("RemoveBackendRecommendation failed: recommendation still exists")
			break
		}
	}
}

func TestFastMatcherWithConfig(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "fast_matcher_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 创建配置管理器
	configManager := NewFastMatcherConfigManager(configPath)

	// 添加自定义映射
	config := configManager.GetConfig()
	config.CategoryToTask["my-custom-category"] = TaskTranslation
	config.TaskToBackends[TaskTranslation] = []BackendRecommendationConfig{
		{BackendID: "my-custom-backend", Model: "my-custom-model", Priority: 1, Reason: "自定义"},
	}

	// 保存配置
	if err := configManager.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 创建快速匹配器
	matcher := NewFastMatcherWithConfig(configManager)

	// 验证自定义映射
	taskType := matcher.MatchCategory("my-custom-category")
	if taskType != TaskTranslation {
		t.Errorf("MatchCategory('my-custom-category') = %v, want TaskTranslation", taskType)
	}

	// 重新加载配置
	if err := matcher.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() error = %v", err)
	}

	// 验证重新加载后仍然有效
	taskType = matcher.MatchCategory("my-custom-category")
	if taskType != TaskTranslation {
		t.Errorf("After ReloadConfig, MatchCategory('my-custom-category') = %v, want TaskTranslation", taskType)
	}
}

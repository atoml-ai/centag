package scheduler

import (
	"testing"

	"centag/core/pkg/backend"
)

// mockBackend 创建测试用的后端配置
func mockBackend(id string, enabled bool) *backend.BackendConfig {
	return &backend.BackendConfig{
		ID:      id,
		Name:    id + "-name",
		Enabled: enabled,
		SupportedModels: []backend.ModelMapping{
			{ActualModel: id + "-model"},
		},
	}
}

func TestTaskTypeSelector_Select(t *testing.T) {
	selector := NewTaskTypeSelector()

	backends := []*backend.BackendConfig{
		mockBackend("bigmodel", true),
		mockBackend("ppinfra", true),
		mockBackend("ollama-local", true),
	}

	tests := []struct {
		name     string
		taskType TaskType
		wantID   string
	}{
		{"代码生成优先bigmodel", TaskCodeGeneration, "bigmodel"},
		{"简单对话优先ollama-local", TaskSimpleChat, "ollama-local"},
		{"复杂推理优先bigmodel", TaskComplexReasoning, "bigmodel"},
		{"长文本优先ppinfra", TaskLongText, "ppinfra"},
		{"向量嵌入优先ollama-local", TaskEmbedding, "ollama-local"},
		{"翻译优先ppinfra", TaskTranslation, "ppinfra"},
		{"创意写作优先bigmodel", TaskCreative, "bigmodel"},
		{"数据分析优先bigmodel", TaskAnalysis, "bigmodel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := selector.Select(tt.taskType, backends)
			if got == nil {
				t.Errorf("Select() returned nil, want backend ID %s", tt.wantID)
				return
			}
			if got.ID != tt.wantID {
				t.Errorf("Select() got ID = %s, want %s, reason: %s", got.ID, tt.wantID, reason)
			}
		})
	}
}

func TestTaskTypeSelector_Select_DisabledBackend(t *testing.T) {
	selector := NewTaskTypeSelector()

	backends := []*backend.BackendConfig{
		mockBackend("bigmodel", false), // 禁用
		mockBackend("ppinfra", true),
	}

	got, reason := selector.Select(TaskCodeGeneration, backends)
	if got == nil {
		t.Error("Select() returned nil, expected ppinfra")
		return
	}
	if got.ID != "ppinfra" {
		t.Errorf("Select() got ID = %s, want ppinfra, reason: %s", got.ID, reason)
	}
}

func TestTaskTypeSelector_Select_NoBackends(t *testing.T) {
	selector := NewTaskTypeSelector()

	got, reason := selector.Select(TaskCodeGeneration, []*backend.BackendConfig{})
	if got != nil {
		t.Errorf("Select() got %v, want nil", got)
	}
	t.Logf("reason: %s", reason)
}

func TestTaskTypeSelector_CustomPriorities(t *testing.T) {
	selector := NewTaskTypeSelector()

	// 自定义优先级：代码生成优先 ppinfra
	selector.TaskPriorities[TaskCodeGeneration] = []string{"ppinfra", "bigmodel"}

	backends := []*backend.BackendConfig{
		mockBackend("bigmodel", true),
		mockBackend("ppinfra", true),
	}

	got, _ := selector.Select(TaskCodeGeneration, backends)
	if got == nil {
		t.Error("Select() returned nil")
		return
	}
	if got.ID != "ppinfra" {
		t.Errorf("Select() got ID = %s, want ppinfra (custom priority)", got.ID)
	}
}

func TestConfigDrivenSelector_LoadPrioritiesFromConfig(t *testing.T) {
	selector := NewConfigDrivenSelector(nil)

	config := map[string][]string{
		"code_generation": {"ppinfra", "bigmodel"},
		"simple_chat":     {"ollama-local", "bigmodel"},
	}

	selector.LoadPrioritiesFromConfig(config)

	if len(selector.TaskPriorities[TaskCodeGeneration]) != 2 {
		t.Errorf("TaskPriorities[code_generation] has %d items, want 2", len(selector.TaskPriorities[TaskCodeGeneration]))
	}
	if selector.TaskPriorities[TaskCodeGeneration][0] != "ppinfra" {
		t.Errorf("TaskPriorities[code_generation][0] = %s, want ppinfra", selector.TaskPriorities[TaskCodeGeneration][0])
	}
}

func TestTaskTypeSelector_GetPriority(t *testing.T) {
	selector := NewTaskTypeSelector()
	if selector.GetPriority() != 100 {
		t.Errorf("GetPriority() = %d, want 100", selector.GetPriority())
	}
}

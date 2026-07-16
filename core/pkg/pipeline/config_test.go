package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLConfigLoaderLoadFromFile(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建测试YAML文件
	yamlContent := `id: test-pipeline
name: Test Pipeline
description: A test pipeline
version: "1.0"
nodes:
  - id: generator
    type: generator
    name: Generator Node
    backend: test-backend
    model: gpt-4
    timeout: 60
global_config:
  timeout: 120
  max_retries: 3
  bypass_on_error: true
`
	yamlFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := NewPipelineRegistry()
	loader := NewYAMLConfigLoader(registry)

	pipeline, err := loader.LoadFromFile(yamlFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if pipeline.ID != "test-pipeline" {
		t.Errorf("Expected ID 'test-pipeline', got '%s'", pipeline.ID)
	}
	if pipeline.Name != "Test Pipeline" {
		t.Errorf("Expected name 'Test Pipeline', got '%s'", pipeline.Name)
	}
	if len(pipeline.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(pipeline.Nodes))
	}
}

func TestYAMLConfigLoaderLoadFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建多个YAML文件
	yaml1 := `id: pipeline-1
name: Pipeline 1
version: "1.0"
nodes:
  - id: node1
    type: generator
    backend: b
    model: m
global_config:
  timeout: 60
`
	yaml2 := `id: pipeline-2
name: Pipeline 2
version: "1.0"
nodes:
  - id: node1
    type: processor
    backend: b
    model: m
global_config:
  timeout: 60
`

	os.WriteFile(filepath.Join(tmpDir, "pipeline1.yaml"), []byte(yaml1), 0644)
	os.WriteFile(filepath.Join(tmpDir, "pipeline2.yml"), []byte(yaml2), 0644)

	registry := NewPipelineRegistry()
	loader := NewYAMLConfigLoader(registry)

	pipelines, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	if len(pipelines) != 2 {
		t.Errorf("Expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestYAMLConfigLoaderInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建无效YAML文件
	invalidYAML := `invalid: yaml: content: [`
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")
	os.WriteFile(yamlFile, []byte(invalidYAML), 0644)

	registry := NewPipelineRegistry()
	loader := NewYAMLConfigLoader(registry)

	_, err := loader.LoadFromFile(yamlFile)
	if err == nil {
		t.Error("Should fail for invalid YAML")
	}
}

func TestYAMLConfigLoaderNonexistentFile(t *testing.T) {
	registry := NewPipelineRegistry()
	loader := NewYAMLConfigLoader(registry)

	_, err := loader.LoadFromFile("/nonexistent/file.yaml")
	if err == nil {
		t.Error("Should fail for nonexistent file")
	}
}

func TestConfigValidator(t *testing.T) {
	validator := NewConfigValidator()

	if validator.HasErrors() {
		t.Error("New validator should not have errors")
	}

	validator.AddError("error 1")
	validator.AddError("error 2")

	if !validator.HasErrors() {
		t.Error("Validator should have errors after adding")
	}

	errors := validator.GetErrors()
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}

	errStr := validator.Error()
	if errStr == "" {
		t.Error("Error string should not be empty")
	}
}

func TestValidatePipelineConfig(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *AgentPatternPipeline
		wantErrs int
	}{
		{
			name: "valid pipeline",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				},
			},
			wantErrs: 0,
		},
		{
			name: "missing all required fields",
			pipeline: &AgentPatternPipeline{
				ID:    "",
				Name:  "",
				Nodes: []PipelineNodeConfig{},
			},
			wantErrs: 3, // id, name, version
		},
		{
			name: "self dependency",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", DependsOn: []string{"node1"}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "self next node",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m", NextNodes: []string{"node1"}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid global config",
			pipeline: &AgentPatternPipeline{
				ID:      "test",
				Name:    "Test",
				Version: "1.0",
				Nodes: []PipelineNodeConfig{
					{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
				},
				GlobalConfig: GlobalPipelineConfig{
					Timeout:       -1,
					MaxRetries:    -1,
					ParallelLimit: -1,
				},
			},
			wantErrs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidatePipelineConfig(tt.pipeline)
			if len(validator.GetErrors()) != tt.wantErrs {
				t.Errorf("Expected %d errors, got %d: %v", tt.wantErrs, len(validator.GetErrors()), validator.GetErrors())
			}
		})
	}
}


func TestCreatePipelineFromTemplate(t *testing.T) {
	template := PatternTemplate{
		ID:          "test-template",
		Name:        "Test Template",
		Description: "A test template",
		Nodes: []PipelineNodeConfig{
			{ID: "node1", Type: NodeTypeGenerator, Backend: "b", Model: "m"},
		},
	}

	pipeline := CreatePipelineFromTemplate(template, nil)

	if pipeline.ID != "test-template" {
		t.Errorf("Expected ID 'test-template', got '%s'", pipeline.ID)
	}
	if pipeline.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", pipeline.Version)
	}
	if len(pipeline.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(pipeline.Nodes))
	}

	// 测试覆盖
	overrides := map[string]interface{}{
		"id":   "overridden-id",
		"name": "Overridden Name",
	}
	pipeline = CreatePipelineFromTemplate(template, overrides)
	if pipeline.ID != "overridden-id" {
		t.Errorf("Expected overridden ID, got '%s'", pipeline.ID)
	}
	if pipeline.Name != "Overridden Name" {
		t.Errorf("Expected overridden name, got '%s'", pipeline.Name)
	}
}

func TestYAMLConfigLoaderSaveToFile(t *testing.T) {
	tmpDir := t.TempDir()

	pipeline := &AgentPatternPipeline{
		ID:          "test-save",
		Name:        "Test Save",
		Description: "Test saving to YAML",
		Version:     "1.0",
		Nodes: []PipelineNodeConfig{
			{
				ID:      "node1",
				Type:    NodeTypeGenerator,
				Name:    "Generator",
				Backend: "test-backend",
				Model:   "gpt-4",
			},
		},
		GlobalConfig: DefaultGlobalConfig(),
	}

	registry := NewPipelineRegistry()
	loader := NewYAMLConfigLoader(registry)

	savePath := filepath.Join(tmpDir, "saved.yaml")
	err := loader.SaveToFile(pipeline, savePath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Error("Saved file should exist")
	}

	// 验证可以重新加载
	loaded, err := loader.LoadFromFile(savePath)
	if err != nil {
		t.Fatalf("Failed to reload saved file: %v", err)
	}

	if loaded.ID != pipeline.ID {
		t.Errorf("Expected ID '%s', got '%s'", pipeline.ID, loaded.ID)
	}
}

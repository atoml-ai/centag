package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLConfigLoader YAML配置加载器
type YAMLConfigLoader struct {
	registry *PipelineRegistry
}

func NewYAMLConfigLoader(registry *PipelineRegistry) *YAMLConfigLoader {
	return &YAMLConfigLoader{registry: registry}
}

// LoadFromFile 从单个YAML文件加载流水线配置
func (l *YAMLConfigLoader) LoadFromFile(filePath string) (*AgentPatternPipeline, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var pipeline AgentPatternPipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse YAML %s: %w", filePath, err)
	}

	// 设置默认值
	if pipeline.GlobalConfig.Timeout == 0 {
		pipeline.GlobalConfig = DefaultGlobalConfig()
	}

	// 验证配置
	if err := pipeline.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pipeline config in %s: %w", filePath, err)
	}

	// 注册到注册表
	if err := l.registry.Register(&pipeline); err != nil {
		return nil, fmt.Errorf("failed to register pipeline from %s: %w", filePath, err)
	}

	return &pipeline, nil
}

// LoadFromDirectory 从目录加载所有YAML配置
func (l *YAMLConfigLoader) LoadFromDirectory(dirPath string) ([]*AgentPatternPipeline, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var pipelines []*AgentPatternPipeline
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			filePath := filepath.Join(dirPath, name)
			pipeline, err := l.LoadFromFile(filePath)
			if err != nil {
				// 记录错误但继续加载其他文件
				fmt.Printf("Warning: failed to load %s: %v\n", filePath, err)
				continue
			}
			pipelines = append(pipelines, pipeline)
		}
	}

	return pipelines, nil
}

// SaveToFile 将流水线配置保存到YAML文件
func (l *YAMLConfigLoader) SaveToFile(pipeline *AgentPatternPipeline, filePath string) error {
	data, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("failed to marshal pipeline: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// ConfigValidator 配置验证器
type ConfigValidator struct {
	errors []string
}

func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{errors: make([]string, 0)}
}

func (v *ConfigValidator) AddError(msg string) {
	v.errors = append(v.errors, msg)
}

func (v *ConfigValidator) HasErrors() bool {
	return len(v.errors) > 0
}

func (v *ConfigValidator) GetErrors() []string {
	return v.errors
}

func (v *ConfigValidator) Error() string {
	return fmt.Sprintf("config validation failed: %s", strings.Join(v.errors, "; "))
}

// ValidatePipelineConfig 验证流水线配置的完整性
func ValidatePipelineConfig(pipeline *AgentPatternPipeline) *ConfigValidator {
	validator := NewConfigValidator()

	// 验证基本信息
	if pipeline.ID == "" {
		validator.AddError("pipeline id is required")
	}
	if pipeline.Name == "" {
		validator.AddError("pipeline name is required")
	}
	if pipeline.Version == "" {
		validator.AddError("pipeline version is required")
	}

	// 空节点草稿流水线仅校验基础信息
	if len(pipeline.Nodes) == 0 {
		return validator
	}

	nodeIDs := make(map[string]bool)
	for i, node := range pipeline.Nodes {
		prefix := fmt.Sprintf("node[%d](%s)", i, node.ID)

		if node.ID == "" {
			validator.AddError(fmt.Sprintf("%s: id is required", prefix))
		} else if nodeIDs[node.ID] {
			validator.AddError(fmt.Sprintf("%s: duplicate id '%s'", prefix, node.ID))
		} else {
			nodeIDs[node.ID] = true
		}

		pluginNode := node.Implementation != "" || node.Kind != ""
		if node.Type == "" && !pluginNode {
			validator.AddError(fmt.Sprintf("%s: type or implementation is required", prefix))
		}
		if node.Type != "" && !node.Type.IsValid() && !pluginNode {
			validator.AddError(fmt.Sprintf("%s: invalid type '%s'", prefix, node.Type))
		}

		// 草稿节点允许暂不配置 backend/model

		// 验证依赖关系
		for _, dep := range node.DependsOn {
			if dep == node.ID {
				validator.AddError(fmt.Sprintf("%s: cannot depend on itself", prefix))
			}
		}

		for _, next := range node.NextNodes {
			if next == node.ID {
				validator.AddError(fmt.Sprintf("%s: cannot point to itself", prefix))
			}
		}
	}

	// 验证依赖的节点是否存在
	for _, node := range pipeline.Nodes {
		for _, dep := range node.DependsOn {
			if !nodeIDs[dep] {
				validator.AddError(fmt.Sprintf("node '%s': dependency '%s' not found", node.ID, dep))
			}
		}
		for _, next := range node.NextNodes {
			if !nodeIDs[next] {
				validator.AddError(fmt.Sprintf("node '%s': next node '%s' not found", node.ID, next))
			}
		}
	}

	// 验证全局配置
	if pipeline.GlobalConfig.Timeout < 0 {
		validator.AddError("global_config.timeout must be non-negative")
	}
	if pipeline.GlobalConfig.MaxRetries < 0 {
		validator.AddError("global_config.max_retries must be non-negative")
	}
	if pipeline.GlobalConfig.ParallelLimit < 1 && pipeline.GlobalConfig.ParallelLimit != 0 {
		validator.AddError("global_config.parallel_limit must be at least 1")
	}

	return validator
}

// PatternTemplate 模式模板 - 用于快速创建常见模式
type PatternTemplate struct {
	ID            string
	SchemaVersion string
	Name          string
	Description   string
	ShortcutCode  string  // 快捷码，如 "#d", "#s", "#mem0"
	Nodes         []PipelineNodeConfig
	GlobalConfig  *GlobalPipelineConfig
	Metadata      map[string]interface{}
}

// CreatePipelineFromTemplate 从模板创建流水线
func CreatePipelineFromTemplate(template PatternTemplate, overrides map[string]interface{}) *AgentPatternPipeline {
	globalConfig := GlobalPipelineConfig{
		Timeout:       120,
		MaxRetries:    3,
		BypassOnError: true,
		ParallelLimit: 4,
	}
	if template.GlobalConfig != nil {
		globalConfig = *template.GlobalConfig
	}

	var metadata map[string]interface{}
	if len(template.Metadata) > 0 {
		metadata = make(map[string]interface{}, len(template.Metadata))
		for k, v := range template.Metadata {
			metadata[k] = v
		}
	}

	// 归一化所有节点：将顶层 Backend/Model 归入 Config
	for i := range template.Nodes {
		template.Nodes[i].Normalize()
	}

	pipeline := &AgentPatternPipeline{
		SchemaVersion: template.SchemaVersion,
		ID:            template.ID,
		Name:          template.Name,
		Description:   template.Description,
		ShortcutCode:  template.ShortcutCode,
		Version:       "1.0",
		Nodes:         template.Nodes,
		GlobalConfig:  globalConfig,
		Metadata:      metadata,
	}

	// 应用覆盖配置
	if overrides != nil {
		if id, ok := overrides["id"].(string); ok {
			pipeline.ID = id
		}
		if name, ok := overrides["name"].(string); ok {
			pipeline.Name = name
		}
		if desc, ok := overrides["description"].(string); ok {
			pipeline.Description = desc
		}
	}

	return pipeline
}

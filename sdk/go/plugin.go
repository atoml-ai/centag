package sdk

import (
	"context"
	"encoding/json"
	"fmt"
)

// Plugin 插件接口
type Plugin interface {
	// GetDescriptor 获取插件描述
	GetDescriptor() *PluginDescriptor
	
	// ValidateConfig 验证配置
	ValidateConfig(config map[string]interface{}) error
	
	// Execute 执行
	Execute(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error)
}

// PluginDescriptor 插件描述
type PluginDescriptor struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Kind        string                 `json:"kind"` // generator, processor, reviewer, router
	Permissions []string               `json:"permissions"`
	ConfigSchema map[string]interface{} `json:"config_schema"`
}

// ExecuteInput 执行输入
type ExecuteInput struct {
	Content  string                 `json:"content"`
	Messages []Message              `json:"messages,omitempty"`
	Config   map[string]interface{} `json:"config"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// ExecuteOutput 执行输出
type ExecuteOutput struct {
	Content string                 `json:"content"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BasePlugin 基础插件
type BasePlugin struct {
	Descriptor *PluginDescriptor
}

// GetDescriptor 获取描述
func (p *BasePlugin) GetDescriptor() *PluginDescriptor {
	return p.Descriptor
}

// ValidateConfig 验证配置 (默认实现)
func (p *BasePlugin) ValidateConfig(config map[string]interface{}) error {
	// 基础验证：检查必需字段
	if p.Descriptor.ConfigSchema != nil {
		required, ok := p.Descriptor.ConfigSchema["required"].([]interface{})
		if ok {
			for _, field := range required {
				fieldName := field.(string)
				if _, exists := config[fieldName]; !exists {
					return fmt.Errorf("required config field missing: %s", fieldName)
				}
			}
		}
	}
	return nil
}

// PluginBuilder 插件构建器
type PluginBuilder struct {
	descriptor *PluginDescriptor
	handler    func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error)
}

// NewPluginBuilder 创建插件构建器
func NewPluginBuilder() *PluginBuilder {
	return &PluginBuilder{
		descriptor: &PluginDescriptor{
			ConfigSchema: make(map[string]interface{}),
		},
	}
}

// WithID 设置 ID
func (b *PluginBuilder) WithID(id string) *PluginBuilder {
	b.descriptor.ID = id
	return b
}

// WithName 设置名称
func (b *PluginBuilder) WithName(name string) *PluginBuilder {
	b.descriptor.Name = name
	return b
}

// WithVersion 设置版本
func (b *PluginBuilder) WithVersion(version string) *PluginBuilder {
	b.descriptor.Version = version
	return b
}

// WithDescription 设置描述
func (b *PluginBuilder) WithDescription(desc string) *PluginBuilder {
	b.descriptor.Description = desc
	return b
}

// WithAuthor 设置作者
func (b *PluginBuilder) WithAuthor(author string) *PluginBuilder {
	b.descriptor.Author = author
	return b
}

// WithKind 设置类型
func (b *PluginBuilder) WithKind(kind string) *PluginBuilder {
	b.descriptor.Kind = kind
	return b
}

// WithPermissions 设置权限
func (b *PluginBuilder) WithPermissions(permissions []string) *PluginBuilder {
	b.descriptor.Permissions = permissions
	return b
}

// WithConfigSchema 设置配置 Schema
func (b *PluginBuilder) WithConfigSchema(schema map[string]interface{}) *PluginBuilder {
	b.descriptor.ConfigSchema = schema
	return b
}

// WithHandler 设置处理器
func (b *PluginBuilder) WithHandler(handler func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error)) *PluginBuilder {
	b.handler = handler
	return b
}

// Build 构建插件
func (b *PluginBuilder) Build() Plugin {
	return &builtInPlugin{
		descriptor: b.descriptor,
		handler:    b.handler,
	}
}

// builtInPlugin 内置插件
type builtInPlugin struct {
	descriptor *PluginDescriptor
	handler    func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error)
}

// GetDescriptor 获取描述
func (p *builtInPlugin) GetDescriptor() *PluginDescriptor {
	return p.descriptor
}

// ValidateConfig 验证配置
func (p *builtInPlugin) ValidateConfig(config map[string]interface{}) error {
	// 基础验证
	if p.descriptor.ConfigSchema != nil {
		required, ok := p.descriptor.ConfigSchema["required"].([]interface{})
		if ok {
			for _, field := range required {
				fieldName := field.(string)
				if _, exists := config[fieldName]; !exists {
					return fmt.Errorf("required config field missing: %s", fieldName)
				}
			}
		}
	}
	return nil
}

// Execute 执行
func (p *builtInPlugin) Execute(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
	if p.handler == nil {
		return nil, fmt.Errorf("handler not implemented")
	}
	return p.handler(ctx, input)
}

// TestHelper 测试辅助函数

// NewTestInput 创建测试输入
func NewTestInput(content string) *ExecuteInput {
	return &ExecuteInput{
		Content: content,
		Config:  make(map[string]interface{}),
	}
}

// NewTestOutput 创建测试输出
func NewTestOutput(content string) *ExecuteOutput {
	return &ExecuteOutput{
		Content: content,
		Data:    make(map[string]interface{}),
	}
}

// AssertOutput 断言输出
func AssertOutput(expected, actual *ExecuteOutput) error {
	if expected.Content != actual.Content {
		return fmt.Errorf("content mismatch: expected '%s', got '%s'", expected.Content, actual.Content)
	}
	return nil
}

// JSONMarshal 序列化为 JSON
func JSONMarshal(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}

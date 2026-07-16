# Go 插件开发模板

本模板演示如何为 ProxyClaw 流水线开发一个标准的 Go 节点插件。

## 目录结构

```
plugins/
  └── myplugin/
      ├── plugin.go        // 插件实现
      └── register.go      // 注册函数
```

## plugin.go

```go
package myplugin

import (
	"context"
	"fmt"

	"centag/internal/pipeline"
)

// MyPlugin 自定义节点插件
type MyPlugin struct{}

// Descriptor 返回插件描述符
func (p *MyPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "My Plugin",
		Implementation: "example.my-plugin",
		Kind:           "custom.my-category",
		Version:        "1.0.0",
		Description:    "插件功能描述",
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"my_param": map[string]interface{}{
					"type":        "string",
					"description": "参数说明",
					"default":     "default_value",
				},
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "输入内容",
				},
			},
			"required": []string{"content"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "输出内容",
				},
			},
		},
		Permissions: []string{"llm.call"}, // 需要的权限
		SupportsStream: false,
	}
}

// ValidateConfig 校验插件配置
func (p *MyPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	// 校验配置参数
	return nil
}

// Execute 执行插件逻辑
func (p *MyPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}

	// 获取 CapabilityBroker（如果插件需要调用 LLM、存储等服务）
	// broker := req.CapabilityBroker

	// 获取密钥（如果配置了 secrets_ref）
	// if req.Secrets != nil {
	//     apiKey := req.Secrets["my_api_key"]
	// }

	// 业务逻辑
	input := req.Input.Content
	output := "Processed: " + input

	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: output,
			Metadata: map[string]interface{}{
				"processed": true,
			},
		},
	}, nil
}
```

## register.go

```go
package myplugin

import "centag/internal/pipeline"

// RegisterMyPlugin 注册插件到节点注册表
func RegisterMyPlugin(registry *pipeline.NodeRegistry) {
	registry.RegisterPlugin(&MyPlugin{})
}
```

## 使用方式

在 `internal/server/server.go` 或初始化代码中：

```go
import "centag/internal/pipeline/archive/deprecated/examples/myplugin"

func init() {
    // 假设有一个全局的 NodeRegistry 实例
    // myplugin.RegisterMyPlugin(globalNodeRegistry)
}
```

## 权限说明

在 `Descriptor()` 中声明插件需要的权限，例如：

- `llm.call` — 调用 LLM 服务
- `storage.read` / `storage.write` — 读写存储
- `memory.read` / `memory.write` — 读写记忆
- `network.outbound` — 发起外部 HTTP 请求
- `secrets.read` — 读取密钥

## 配置 Schema

使用 [JSON Schema](https://json-schema.org/) 定义 `ConfigSchema`，WebUI 会根据该 Schema 自动生成配置表单。

## 输入/输出 Schema

定义 `InputSchema` 和 `OutputSchema`，便于流水线编排和类型检查。

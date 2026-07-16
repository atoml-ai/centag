# 插件兼容测试套件

本测试套件用于验证第三方插件是否符合 ProxyClaw 流水线节点插件契约。

## 测试项目

### 1. Descriptor 验证

- [ ] `implementation` 字段必填
- [ ] `kind` 字段必填
- [ ] `version` 字段必填
- [ ] `api_version` 应为 `centag.pipeline.node/v1alpha1` 或更高
- [ ] 如果 `remote` 不为空，`base_url` 和 `manifest_url` 应为有效的 URL

### 2. ValidateConfig 验证

- [ ] 调用 `POST /validate` 或 `ValidateConfig()`
- [ ] 有效配置应返回 `{"valid": true}`
- [ ] 无效配置应返回 `{"valid": false, "errors": [...]}`
- [ ] HTTP 500 应被视为"插件未实现该端点"

### 3. Execute 验证

- [ ] 调用 `POST /execute` 或 `Execute()`
- [ ] 请求应包含 `schema_version`、`implementation`、`config`、`input`
- [ ] 成功响应应包含 `output.content`
- [ ] 失败时应返回错误（非 200 状态码或 error message）

### 4. Schema 验证

- [ ] `config_schema` 应符合 JSON Schema draft-07
- [ ] `input_schema` 应符合 JSON Schema draft-07
- [ ] `output_schema` 应符合 JSON Schema draft-07
- [ ] 有效输入应通过 schema 校验
- [ ] 无效输入应被拒绝

### 5. 权限验证

- [ ] `permissions` 字段应包含所需权限
- [ ] 无权限时应返回错误
- [ ] `secrets.read` 权限应允许访问密钥

### 6. 性能验证

- [ ] 执行时间应在合理范围内（默认 30s）
- [ ] 超时时应返回错误
- [ ] 并发执行应不超过 semaphore 限制

## 测试工具

### Go 测试辅助函数

```go
package plugin_test

import (
	"testing"
	"centag/internal/pipeline"
)

// TestPluginDescriptor 测试插件描述符
func TestPluginDescriptor(t *testing.T, plugin pipeline.NodePlugin) {
	desc := plugin.Descriptor()
	if desc.Implementation == "" {
		t.Error("implementation is required")
	}
	if desc.Kind == "" {
		t.Error("kind is required")
	}
	if desc.Version == "" {
		t.Error("version is required")
	}
}

// TestPluginValidate 测试配置校验
func TestPluginValidate(t *testing.T, plugin pipeline.NodePlugin) {
	// 有效配置
	err := plugin.ValidateConfig(pipeline.NodeConfig{
		Model: "test-model",
	})
	if err != nil {
		t.Errorf("valid config failed: %v", err)
	}
}

// TestPluginExecute 测试执行
func TestPluginExecute(t *testing.T, plugin pipeline.NodePlugin) {
	req := &pipeline.NodeExecutionRequest{
		SchemaVersion:  pipeline.PipelinePluginSchemaVersion,
		Implementation: plugin.Descriptor().Implementation,
		Config:        pipeline.NodeConfig{},
		Input: &pipeline.NodeInput{
			Content: "test input",
		},
	}
	resp, err := plugin.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resp == nil || resp.Output == nil {
		t.Fatal("output is nil")
	}
}
```

### curl 测试示例

```bash
# 获取描述符
curl http://localhost:3000/.well-known/centag-node-plugin.json

# 校验配置
curl -X POST http://localhost:3000/validate \
  -H "Content-Type: application/json" \
  -d '{"schema_version": "centag.pipeline.node/v1alpha1", "config": {}}'

# 执行
curl -X POST http://localhost:3000/execute \
  -H "Content-Type: application/json" \
  -d '{"schema_version": "centag.pipeline.node/v1alpha1", "input": {"content": "test"}}'
```

## 自动化测试脚本

TODO: 创建 `scripts/plugin-test.sh`，自动运行上述测试项目。

## 迁移规则（v1alpha1 → v1beta1）

TODO: 定义迁移规则，包括：
- 字段重命名
- 字段弃用
- 新字段添加
- 行为变更

## 官方 allowlist 示例

TODO: 创建 `docs/guides/plugin-allowlist-example.md`，包含：
- 受信任的插件来源
- 允许的 URL 模式
- 推荐的权限配置

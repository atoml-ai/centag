# 插件上架流程

> 面向 AI 编码智能体：定义插件从开发到上架的完整流程。

---

## 1. 上架流程概览

```
开发者                 审查者                 系统
  │                      │                     │
  ├─ 1. 开发插件        │                     │
  │                      │                     │
  ├─ 2. 自测            │                     │
  │                      │                     │
  ├─ 3. 提交描述符      │                     │
  │                      │                     │
  │                      ├─ 4. 准入检查       │
  │                      │                     │
  │                      ├─ 5. 安全扫描       │
  │                      │                     │
  │                      ├─ 6. 上架决定       │
  │                      │                     │
  │                      │                     ├─ 7. 注册插件
  │                      │                     │
  │                      │                     ├─ 8. 健康检查
  │                      │                     │
  │                      │                     ├─ 9. 上线通知
  │                      │                     │
  └──────────────────────┴─────────────────────┴
```

---

## 2. 详细步骤

### 步骤 1-2：开发与自测

**开发清单**：
- [ ] 实现 `NodePlugin` 接口
- [ ] 填写 `NodePluginDescriptor`（name, implementation, kind, version, permissions）
- [ ] 提供 `config_schema`, `input_schema`, `output_schema`
- [ ] 完成单元测试（覆盖率 > 80%）
- [ ] 运行 `go test -race ./...` 无竞态
- [ ] 提供 `examples/` 目录的示例调用（旧示例已归档至 `../../archive/deprecated/examples/`）

**自测命令**：
```bash
# 契约测试
go test -v ./internal/pipeline/... -run "Test.*Contract|Test.*Descriptor|Test.*Validate"

# 竞态检测
go test -race ./internal/pipeline/... -run "TestRemoteNode"

# E2E 测试（需要真实 HTTP 服务）
go test -v ./internal/pipeline/... -run "TestRemotePluginE2E"
```

### 步骤 3：提交描述符

**描述符示例**：
```json
{
  "name": "My Custom Plugin",
  "implementation": "https://my-plugin.example.com",
  "kind": "processor",
  "version": "1.0.0",
  "description": "My custom processor plugin",
  "config_schema": { ... },
  "input_schema": { ... },
  "output_schema": { ... },
  "permissions": ["llm.call"],
  "supports_stream": false,
  "min_centag_version": "1.0.0",
  "tags": ["custom", "processor"],
  "expected_hash": "sha256-hex-string"
}
```

**提交方式**：
```bash
# 方式1：通过 API 注册
curl -X POST http://localhost:20060/api/v1/pipelines/node-plugins \
  -H "Content-Type: application/json" \
  -d @manifest.json

# 方式2：通过 CLI 注册（如果提供）
centag plugin register manifest.json
```

### 步骤 4：准入检查

**自动检查项**：
- [ ] 权限最小化（不超过 10 个）
- [ ] 版本号已设置（不为 "unknown"）
- [ ] 实现标识不为空
- [ ] 超时设置合理（5s ~ 300s）
- [ ] 重试策略已声明
- [ ] 指标暴露（可选）

**检查命令**：
```bash
# 手动检查
go run ./scripts/tools/admission-checker.go \
  --manifest .well-known/centag-node-plugin.json \
  --config ../../archive/deprecated/configs/plugin-security.yaml
```

**通过标准**：总分 >= 70 分

### 步骤 5：安全扫描

**扫描内容**：
1. 权限过度申请（如 `*`、`sudo`、`root`）
2. 硬编码密钥（`api_key`、`secret`、`password`）
3. 敏感操作（文件删除、系统命令执行）
4. 网络请求（是否声明 `network.outbound` 权限）

**扫描报告示例**：
```
Admission Result for my-plugin:
  Passed: false
  Score: 65/100
  Issues:
    - [high] error_handling: Plugin implementation identifier is missing
  Warnings:
    - Plugin does not specify retry or fallback strategy
    - Plugin version is not specified
    - Plugin does not expose metrics
```

### 步骤 6：上架决定

**审查清单**：
- [ ] 准入检查通过（分数 >= 70）
- [ ] 安全扫描无高危问题
- [ ] 文档完整（README + 示例）
- [ ] 性能测试通过（延迟 P95 < 2s，错误率 < 1%）

**上架选项**：
1. **直接上架**：分数 >= 90，无警告
2. **条件上架**：分数 70-89，有警告，需监控
3. **拒绝上架**：分数 < 70，有高危问题，需修复后重审

### 步骤 7-9：注册与健康检查

**注册后自动触发**：
1. 写入 `plugin_registry` 表
2. 启动健康检查协程（每 60 秒）
3. 初始状态：`enabled=false`，等待手动审批
4. 审批通过后：`enabled=true`，开始接收流量

**审批命令**：
```bash
# 审批插件
curl -X PUT http://localhost:20060/api/v1/pipelines/plugin-registry/my-plugin \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

---

## 3. 回滚与下架

### 3.1 紧急回滚

```bash
# 禁用插件（停止接收新请求）
curl -X PUT http://localhost:20060/api/v1/pipelines/plugin-registry/my-plugin \
  -d '{"enabled": false}'

# 从流水线中移除
# 编辑流水线配置，移除该插件节点（旧版 YAML 配置已归档至 ../../archive/deprecated/configs/pipelines/）
# 重新加载配置
curl -X POST http://localhost:20060/api/v1/config/reload
```

### 3.2 下架流程

1. 将插件状态设为 `enabled=false`
2. 等待现有请求完成（优雅关闭）
3. 从 `plugin_registry` 表删除记录
4. 归档插件描述符和配置

---

## 4. 监控与审计

### 4.1 监控指标

| 指标 | 说明 |
|------|------|
| `plugin_requests_total` | 插件调用总次数 |
| `plugin_errors_total` | 插件调用失败次数 |
| `plugin_latency_seconds` | 插件调用延迟分布 |
| `plugin_health_status` | 插件健康状态（0=不健康，1=健康） |

### 4.2 审计日志

```json
{
  "timestamp": "2026-05-06T12:00:00Z",
  "event_type": "plugin_onboarded",
  "plugin_id": "my-plugin",
  "implementation": "https://my-plugin.example.com",
  "score": 85,
  "approved_by": "admin",
  "reason": "Passes admission check"
}
```

---

## 5. 模板：上架检查单

```markdown
# 插件上架检查单

## 基本信息
- 插件名称：_____________
- 实现地址：_____________
- 版本号：_____________
- 提交者：_____________
- 审查者：_____________

## 准入检查（自动）
- [ ] 权限最小化（≤10 个）：_____/10 分
- [ ] 版本号已设置：_____/10 分
- [ ] 实现标识不为空：_____/20 分
- [ ] 超时设置合理：_____/20 分
- [ ] 重试策略已声明：_____/15 分
- [ ] 指标暴露：_____/5 分
- **总分**：_____/100 分（通过线：70 分）

## 安全扫描（自动）
- [ ] 无过高权限：通过 / 失败
- [ ] 无硬编码密钥：通过 / 失败
- [ ] 已声明网络权限：通过 / 失败

## 审查决定
- [ ] 直接上架（分数 ≥ 90）
- [ ] 条件上架（分数 70-89，需监控）
- [ ] 拒绝上架（分数 < 70，需修复）

## 审查意见
_________________________________________________
_________________________________________________
_________________________________________________

签名：___________
日期：___________
```

---

*最后更新：2026-05-06*
*维护者：见 ../../docs/harness/AGENTS.md*

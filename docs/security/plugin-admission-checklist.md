# 插件准入检查单

本文档定义了插件上架前的安全检查清单与评分标准。

## 1. 检查清单

### 1.1 权限检查 (Permissions Check)

| 检查项 | 说明 | 严重程度 |
|--------|------|----------|
| 无过高权限 | 权限列表不包含 `*`, `sudo`, `root`, `admin`, `shell`, `exec` | 高 |
| 权限最小化 | 权限数量不超过 10 个 | 中 |
| 权限明确 | 每个权限都有明确用途 | 中 |

**检查示例**：
```go
// Good
Permissions: []string{"llm.call", "memory.read"}

// Bad
Permissions: []string{"*"}
```

### 1.2 超时检查 (Timeout Check)

| 检查项 | 说明 | 严重程度 |
|--------|------|----------|
| 超时上限 | 不超过配置的 MaxTimeoutSeconds (默认 300s) | 高 |
| 超时下限 | 不低于配置的 MinTimeoutSeconds (默认 5s) | 低 |

### 1.3 错误处理检查 (Error Handling Check)

| 检查项 | 说明 | 严重程度 |
|--------|------|----------|
| 版本已设置 | Version 不为空且不为 "unknown" | 中 |
| 实现标识 | Implementation 不为空 | 高 |
| 重试策略 | 声明了 retry 或 fallback | 低 |

### 1.4 可观测性检查 (Observability Check)

| 检查项 | 说明 | 严重程度 |
|--------|------|----------|
| 指标暴露 | metadata 中包含 metrics | 低 |
| 日志配置 | metadata 中包含 logging | 低 |

## 2. 评分标准

### 2.1 评分规则

| 类别 | 基础分 | 扣分项 | 扣分值 |
|------|--------|--------|--------|
| 权限检查 | 100 | 过高权限 | -30 |
| 权限检查 | 100 | 权限过多 (>10) | -10 |
| 超时检查 | 100 | 超时超上限 | -25 |
| 超时检查 | 100 | 超时低于下限 | -10 |
| 错误处理 | 100 | 无实现标识 | -20 |
| 错误处理 | 100 | 无重试策略 | -15 |
| 错误处理 | 100 | 无版本号 | -10 |
| 可观测性 | 100 | 无 metrics | -10 |
| 可观测性 | 100 | 无 logging | -5 |

### 2.2 通过条件

- **总分 >= 70 分**: 通过
- **总分 < 70 分**: 拒绝，需修复后重检

### 2.3 评分计算示例

```go
// 示例: 一个权限过多但无严重问题的插件
result := &AdmissionResult{
    Score: 100 - 10 - 15 - 10 = 65, // 权限过多 -10, 无重试 -15, 无版本 -10
    Passed: false,
}
```

## 3. 检查流程

### 3.1 自动检查

准入检查在插件注册时自动执行：

```go
func (r *NodeRegistry) RegisterPlugin(plugin NodePlugin) error {
    // ...
    if r.admissionChecker != nil && r.admissionChecker.cfg.Enabled {
        result := r.admissionChecker.CheckAll(descriptor, 30)
        if !result.Passed {
            return fmt.Errorf("admission check failed: %s", result.Summary())
        }
    }
    // ...
}
```

### 3.2 手动检查

使用 `AdmissionChecker` 手动检查插件：

```go
checker := pipeline.NewAdmissionChecker(config.DefaultPluginSecurityConfig().AdmissionCheck)
result := checker.CheckAll(descriptor, 30)

fmt.Println(result.Summary())
```

### 3.3 检查输出示例

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

## 4. 修复指南

### 4.1 权限问题修复

**问题**: 权限过高
```go
// 修复前
Permissions: []string{"*"}

// 修复后
Permissions: []string{"llm.call", "memory.read"}
```

### 4.2 超时问题修复

**问题**: 超时过长
```yaml
# 修复前
timeout: 600

# 修复后
timeout: 120
```

### 4.3 版本问题修复

**问题**: 版本未设置
```go
// 修复前
Version: "unknown"

// 修复后
Version: "1.0.0"
```

### 4.4 可观测性修复

**问题**: 无指标暴露
```go
// 修复
Metadata: map[string]string{
    "metrics": "true",
    "logging": "json",
}
```

## 5. 审查记录

| 日期 | 插件 | 分数 | 结果 | 审查人 |
|------|------|------|------|--------|
| - | - | - | - | - |

---

*最后更新：2026-05-04*
*版本：1.0.0*
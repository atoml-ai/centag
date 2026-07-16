---
name: step4-test
description: "工作流 Step 4：单元测试补全 — 为新增代码编写完整单元测试，覆盖边界和异常场景。触发场景：补测试、step4-test、测试补全"
---

# Step 4: 单元测试补全

> 执行指南：`docs/harness/workflow/phase-3-implement.md`（Step 4 部分）

## 触发词

`step4-test` / 补测试 / 测试补全 / step4

## 前置门禁

无。Step 4 与 Step 3 同属 Phase 3，不触发门禁检查。

## 相关上下文

- `docs/harness/CONVENTIONS.md` §3 测试规范
- `docs/harness/workflow/phase-3-implement.md` Step 4 部分

## 执行流程

### 第一步：确定测试范围

1. 读取任务计划，确认哪些任务新增了代码
2. 通过 `git diff` 或阅读新增文件确定待测方法
3. 优先覆盖：Service 层、Handler 层、核心工具函数

### 第二步：编写测试

按 `docs/harness/CONVENTIONS.md` 测试规范编写：

| 规范 | 说明 |
|------|------|
| 文件位置 | 与被测文件同目录，`*_test.go` |
| 函数命名 | `Test<函数名>_<场景描述>` |
| 测试风格 | 表驱动测试（Table-Driven Tests） |
| Mock | 基于 interface 的 Mock，不依赖外部环境 |
| 覆盖场景 | 正常路径、参数校验失败、依赖异常/超时、边界值 |

### 第三步：运行验证

- 运行新增测试：`go test -v ./<模块>/... -run Test<函数名>` ✅ 通过
- 运行全量测试确保无回归：`go test ./...`
- 检查覆盖率：`go test -coverprofile=coverage.out ./internal/...`

## 产出

| 产物 | 路径 |
|------|------|
| 测试文件 | 与被测文件同目录 `*_test.go` |

## 完成后

- 更新 `workflow_state.md`：Step 4 状态 → ✅ 已完成
- 提示用户可执行 `step5-review` 进入质量交付阶段（将自动检查 Gate 3）。

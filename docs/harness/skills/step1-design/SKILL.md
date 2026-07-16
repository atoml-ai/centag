---
name: step1-design
description: "工作流 Step 1：方案设计与确认 — 技术方案设计、AI 内审、人工确认。触发场景：方案设计、step1-design、出方案"
---

# Step 1: 方案设计与确认

> 执行指南：`docs/harness/workflow/phase-1-design.md`

## 触发词

`step1-design` / 方案设计 / step1 / 出方案

## 前置门禁

无。Step 1 是工作流的第一步，可直接开始。

## 相关上下文

开始前确认以下文档是否就绪：
- 相关 `docs/guide/` 文档（了解产品背景）
- 相关 `docs/api/` 文档（了解现有接口）
- `docs/harness/ARCHITECTURE.md`（架构约束）
- `docs/harness/PATTERNS.md`（设计模式参考）

## 执行流程

### 第一步：确认上下文

1. 确认任务/需求名称和版本号
2. 确定方案产出的路径：`docs/versions/<版本>/<需求>/`
3. 确认需要阅读的背景文档
4. **初始化 workflow_state**：检查 `docs/versions/<版本>/<需求>/workflow_state.md` 是否存在，不存在则按 `docs/harness/templates/workflow_state模板.md` 创建

### 第二步：阅读上下文

- 阅读 `docs/harness/ARCHITECTURE.md` 了解模块边界和分层约束
- 阅读 `docs/harness/PATTERNS.md` 了解项目中使用的设计模式
- 阅读 `docs/harness/ANTI-PATTERNS.md` 了解禁止行为
- 阅读相关领域文档（如 `docs/cache/`、`docs/guide/proxy-modes.md` 等）

### 第三步：方案设计

按 `docs/harness/workflow/phase-1-design.md` 执行清单完成方案设计：

1. 设计技术方案，包含：
   - 背景与目标（含非目标，防止范围膨胀）
   - 方案选型与对比（如有多种方案）
   - 接口设计（API 路径、请求/响应结构）
   - 数据模型变更
   - 模块影响范围
   - 风险与应对
2. 按 `docs/harness/templates/技术方案文档模板.md` 结构编排方案
3. 写入 `docs/versions/<版本>/<需求>/技术方案.md`
3. API 或配置行为变更时同步 `docs/api/` 或 `docs/guide/`

### 第四步：AI 内审

- 逐章节检查方案一致性
- 检查接口设计与现有架构一致性
- 识别 Critical 问题并修复

### 第五步：人工确认

- 用户逐章节审核
- 用户确认方案可行

## 产出

| 产物 | 路径 |
|------|------|
| 技术方案 | `docs/versions/<版本>/<需求>/技术方案.md` |

## 完成后

- 更新 `workflow_state.md`：Step 1 状态 → ✅ 已完成，记录完成时间
- 提示用户可执行 `step2-plan` 进入任务规划阶段（将自动检查 Gate 1）。

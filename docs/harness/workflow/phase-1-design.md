# Phase 1: 方案设计（Step 1）

> 快捷别名：`step1-design` / 方案设计 / step1

## 目标

技术方案多轮推敲 + AI 内审 + 人工确认。

## 前置条件

无。Step 1 是工作流的第一步，可直接开始。

## 执行清单

### 1. 确认上下文

- [ ] 确认当前任务/需求名称和版本号
- [ ] 确认需求背景（阅读相关 `docs/guide/`、`docs/api/` 或已有执行计划）
- [ ] 确认涉及的技术栈和模块范围
- [ ] **初始化 workflow_state**：检查 `docs/versions/<版本>/<需求>/workflow_state.md` 是否存在，不存在则创建

### 2. 方案设计

- [ ] 阅读相关架构文档（`docs/harness/ARCHITECTURE.md`）了解模块边界
- [ ] 阅读相关设计模式（`docs/harness/PATTERNS.md`）了解代码模板
- [ ] 设计技术方案，包含：
  - 背景与目标
  - 方案选型与对比
  - 接口设计（API / 配置项变更）
  - 数据模型变更
  - 模块影响范围
  - 风险与应对
- [ ] 按 `docs/harness/templates/技术方案文档模板.md` 结构编写方案
- [ ] 方案写入 `docs/versions/<版本>/<需求>/技术方案.md`
- [ ] API 或配置行为变更时同步更新 `docs/api/` 或 `docs/guide/` 对应文档

### 3. AI 内审

- [ ] 对方案逐章节做一致性检查
- [ ] 检查接口设计与现有架构的一致性
- [ ] 识别 Critical 问题并修复（目标：Critical = 0）

### 4. 人工确认

- [ ] 用户逐章节审核方案
- [ ] 用户确认方案可行

## 产出

| 产物 | 路径 |
|------|------|
| 技术方案 | `docs/versions/<版本>/<需求>/技术方案.md` |

## 完成后

- 更新 `workflow_state.md`：Step 1 → ✅，记录完成时间
- 提示用户可执行 `step2-plan` 进入任务规划阶段（将自动检查 Gate 1）

---
name: centag-wizard-test
description: "Centag 向导测试 OpenCode 交互入口（触发：向导测试/wizard test/一键测试）。用 question 收参后执行 docs/harness/skills/centag-wizard-test.md。分层见 AGENT.md。"
---

# Centag Wizard Test — OpenCode 交互入口

> **本文件只做交互。** 业务正本：`docs/harness/skills/centag-wizard-test.md`  
> 分层原则：仓库根目录 `AGENT.md`

## 硬性规则

1. 全部选择通过 OpenCode **`question`** 工具（多问题表单）。
2. 禁止在聊天正文列 A/B/C 让用户打字。
3. 确认后加载正本执行，不在此发明业务步骤。

## 触发词

`向导测试` / `wizard test` / `centag 向导` / `一键测试` / `测试向导` / `开始向导`

## 收参步骤（question）

1. **基础配置**：`deploy_type`（personal/team/minimal）、`base_url`、Admin 凭据（.env / 自定义）、`test_type`（pipeline/admin/all）
2. **若 pipeline|all**：`backend_source`（real/mock）、`backend_id`、`test_scope`（快速/标准）
3. **确认**：开始测试 / 修改配置

默认值与后端映射以**正本**为准；入口只负责收集并导出环境变量。

## 交接

导出 `CENTAG_DEPLOY_TYPE`、`TEST_*`、`ADMIN_*` 后执行：

**`docs/harness/skills/centag-wizard-test.md`**

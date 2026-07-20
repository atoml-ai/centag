# UI 测试流程 — 普通用户 WebUI 能力矩阵（v0.2.6）

> 版本：v0.2.6 | 日期：2026-07-20 | 分支：`feature/v0.2.6`  
> **执行正本（强制 Browser MCP）**：[`docs/harness/skills/centag-ui-browser-test.md`](../../../harness/skills/centag-ui-browser-test.md)  
> Cursor 入口：`.cursor/rules/centag-ui-browser-test.mdc`（触发词：`UI浏览器测试`）

## 1. 为何单独规定

本次改动信息架构较大（侧栏收缩、对话改挂流水线测试、存储/记忆分角色）。  
**HTTP admin-e2e 无法证明导航与抽屉行为**；完工验收必须以 **浏览器自动化** 为准。

## 2. 何时执行

| 节点 | 要求 |
|------|------|
| Step 3 编码中 | 可按角色冒烟 U12（测试对话） |
| **Gate 3 → Step 5 / 准出前** | **必须**完整跑本流程（适用角色无 P0 fail、无 browser blocked） |
| 仅改文档未改 UI | 可 SKIP，并在自测记录注明 |

## 3. 推荐命令与触发

1. 启动对应 Profile（personal 或 team）并确认：

```bash
curl -s http://localhost:20060/api/v1/status | jq .edition
```

2. 在 Cursor 对话触发：`UI浏览器测试`（或让 Agent 直接加载正本）。  
3. Agent **必须**调用 Browser MCP 按正本 §3 用例操作；产出：

- `/tmp/centag_ui_browser_results.json`
- 自测记录中粘贴 summary（或链接该 JSON）

配套 HTTP 回归（不替代 UI）：

```bash
TEST_DEPLOY_TYPE=personal bash docs/harness/skills/admin-e2e-test.sh   # 或 team
```

## 4. 本版本必跑矩阵

| 部署 | 角色 | 必跑用例（P0 加粗） |
|------|------|---------------------|
| personal | personal | **U01 U02 U12 U14 U15**，U04 U06 U08 U09 U10 U11 U13 U18 U19 |
| team | team_admin | **U01 U03 U12 U14 U15**，U05 U06 U18 U19 U20 |
| team | team_user | **U01 U02 U12 U14 U15**，U04 U07 U08 U09 U16 U17 U19 |
| minimal | minimal | **U01 U12**，U10 U11（若环境有后端）；无代理/存储侧栏 |

> team 交付时 **admin + user 两套都要绿**；只验 admin 不得宣称 v0.2.6 UI 完成。

## 5. P0 失败即阻断准出

| ID | 失败含义 |
|----|----------|
| U01 | 独立「对话」导航回潮 |
| U02 | 普通用户侧栏又挂后端/策略列表 |
| U03 | Admin 丢失共用资源入口 |
| U12 | 流水线测试打不开对话（含 Admin） |
| U14 | `/chat` 独立页仍可当产品入口停留 |
| U15 | 深链编辑被误伤 |

## 6. 证据要求

每条用例在 JSON 中保留：

- 操作前后 snapshot 要点（侧栏可见项、抽屉是否出现、URL）
- `pass|fail|skip|blocked`
- Browser MCP 不可用 → 全套 `blocked`，**Gate 4 不得通过**

## 7. 变更记录

| 日期 | 变更 |
|------|------|
| 2026-07-20 | Step 1 补充：提前锁定完工 UI 浏览器验收流程 |

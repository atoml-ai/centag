# Centag UI 浏览器自动化测试 — 正本

> **业务正本**（`docs/harness/skills/`）。交互入口：`.cursor/rules/centag-ui-browser-test.mdc`。  
> 分层见根目录 **`AGENT.md`**。  
> 与 `centag-admin-e2e.md`（**仅 HTTP**）互补：本正本验证 **WebUI 导航与操作**，必须通过浏览器自动化完成。

---

## 0. 硬性规则

1. **必须调用浏览器能力**（Cursor **Browser MCP** / `cursor-ide-browser`：导航、快照、点击、填表、等待）。  
   - **禁止**仅用 `curl` / 读源码断言后宣称「UI 通过」。  
   - Browser MCP **不可用** → 本技能 **失败退出**，在报告中记 `BLOCKED: browser_mcp_unavailable`，不得改判为通过。
2. 每个用例执行前 `browser_snapshot`（或等价无障碍树），操作后再次快照，记录可见文案/角色名作为证据。
3. 失败记录并继续（同 admin-e2e）；全部跑完输出汇总报告。
4. **不**为某一 edition 单独维护平行测试页；用例按 **角色 × capability** 裁剪（与 v0.2.6 技术方案矩阵一致）。

---

## 1. 前置条件

| 参数 | 环境变量 | 必需 | 说明 |
|------|----------|------|------|
| 服务地址 | `TEST_BASE_URL` | ✅ | 默认 `http://localhost:20060` |
| Web 根路径 | `TEST_WEB_BASE` | 可选 | 默认 `$TEST_BASE_URL/static/`（Vue history base） |
| 部署类型 | `TEST_DEPLOY_TYPE` | ✅ | `personal` / `team` / `minimal` |
| Admin 用户 | `ADMIN_USERNAME` / `ADMIN_PASSWORD` | ✅ | 可读 `config/secrets/.env` |
| 普通用户（team） | `TEST_USER_USERNAME` / `TEST_USER_PASSWORD` | team 必测 user 时 ✅ | 无则先用 admin API 创建临时用户再测，测完删除 |
| 角色子集 | `TEST_UI_ROLES` | 可选 | 逗号分隔：`personal,team_admin,team_user,minimal`；默认按 `TEST_DEPLOY_TYPE` 推断 |

### 1.1 服务探测

```bash
curl -sf "$TEST_BASE_URL/health" >/dev/null
curl -sf "$TEST_BASE_URL/api/v1/status" | jq -r .edition
```

`edition` 必须与 `TEST_DEPLOY_TYPE` 语义一致（personal / team / minimal）。

### 1.2 浏览器会话

1. 调用 Browser MCP：打开 `$TEST_WEB_BASE`（或 `$TEST_BASE_URL/static/`）。  
2. 若已登录其它账号：先走退出，再按用例角色登录。  
3. 登录页：填写用户名/密码 → 提交 → 等待进入 `/dashboard`（或概览）。

---

## 2. 角色与默认套件

| `TEST_DEPLOY_TYPE` | 默认执行角色 | 说明 |
|--------------------|--------------|------|
| `personal` | `personal` | 单用户即 admin 语义；测干活面 + 存储配置 + 记忆 |
| `team` | `team_admin` + `team_user` | **两套登录各跑一遍**；user 账号见上表 |
| `minimal` | `minimal` | 仅首页能力；侧栏应极短 |

可选：`TEST_UI_ROLES=personal` 在 team 部署上只跑某一角色（调试用）。

---

## 3. 用例清单（验收真源）

> 编号 `Uxx`。判定：页面可见性以 **侧栏/顶栏文案 + 主区控件** 为准；API 是否存在不在本技能证明范围（见 admin-e2e）。

### 3.1 导航与入口（全角色相关）

| ID | 标题 | personal | team_user | team_admin | minimal | 步骤摘要 | 期望 |
|----|------|:--------:|:---------:|:----------:|:-------:|----------|------|
| U01 | 无独立「对话」导航 | ✅ | ✅ | ✅ | ✅ | 登录后展开侧栏，检索「对话」菜单项 | **不存在**一级/叶子「对话」；允许流水线「测试」按钮 |
| U02 | 普通用户无「配置→后端/策略」侧栏 | ✅ | ✅ | — | — | 侧栏无「后端管理」「策略管理」列表入口（或无「配置」组） | 后端/流水线 CRUD 入口在 **首页** |
| U03 | Admin 保留共用后端/策略入口 | — | — | ✅ | — | 侧栏可见共用后端 / 共用策略（或等价文案） | 可进入 `/backends` 或 `/pipelines` 列表页 |
| U04 | 本机代理入口 | ✅ | ✅ | — | — | 侧栏有系统代理/本机代理相关入口 | 可打开系统代理页 |
| U05 | Admin 无本机代理导航 | — | — | ✅ | — | 侧栏无主机/系统/Clash 代理菜单 | 直链若被踢回概览亦算通过 |
| U06 | 存储配置入口 | ✅ | — | ✅ | — | personal/admin 可见存储或数据存储入口 | 可打开存储配置页 |
| U07 | User 无存储配置 | — | ✅ | — | — | 侧栏无存储/数据存储；直链 `/storage` | 被带回 dashboard / 无写入口 |
| U08 | 记忆查询入口 | ✅ | ✅ | — | — | 侧栏有「记忆」 | 可打开记忆页；user **无**同步/重建索引主按钮（或按钮禁用） |
| U09 | 用量入口 | ✅ | ✅ | — | — | 有用量/会话相关导航或首页用量区 | 可查看用量摘要 |

### 3.2 首页与流水线测试对话（核心回归）

| ID | 标题 | 适用角色 | 步骤摘要 | 期望 |
|----|------|----------|----------|------|
| U10 | 首页可见后端面板 | personal, team_user, minimal；admin 若概览有后端区则测 | 打开首页 | 后端列表/卡片可见（只读或可编辑视权限） |
| U11 | 首页可见流水线面板 | 同上 | 打开首页 | 流水线列表可见 |
| U12 | 流水线「测试」打开对话抽屉 | **全部角色**（含 team_admin） | 在首页或策略列表点某一流水线「测试」 | 出现对话抽屉/面板（MinimalChat）；**不是**跳转到独立 `/chat` 菜单页 |
| U13 | 测试对话可发送 | 同上（需可用后端；无 Key 则跳过并记 SKIP） | 在抽屉输入短句发送 | 有请求中/回复/或明确错误提示（非白屏） |
| U14 | 直开 `/static/#` 或 `/chat` 路径 | personal, team_user, team_admin | 浏览器打开 `$TEST_WEB_BASE` 下 chat 路由（以实际 router 为准，如 `/static/chat`） | **不**停留在独立对话产品页：redirect 到 dashboard 或无该菜单可达 |

### 3.3 深链与权限

| ID | 标题 | 适用角色 | 步骤摘要 | 期望 |
|----|------|----------|----------|------|
| U15 | 流水线编辑深链 | personal, team_user（`can_add_own_pipelines` 默认开）, team_admin | 首页点「编辑」或打开 `/pipelines/<已有id>` | 编辑器打开，非被踢回 |
| U16 | User 不能改存储 | team_user | 打开 `/storage` | redirect 或 403 提示；无「添加存储」成功路径 |
| U17 | 我的租户 | team_user | 更多→系统→我的租户 | 页面可达 |
| U18 | 系统配置 | personal, team_admin | 打开系统配置 | 可达；team_user 不可达 |

### 3.4 复用原则抽检（防平行页回潮）

| ID | 标题 | 步骤 | 期望 |
|----|------|------|------|
| U19 | 测试对话组件一致 | 分别在 personal（或 user）与 team_admin 打开「测试」 | 同一套抽屉 UI（标题/布局同源），非另一套「Admin 专用聊天页」 |
| U20 | 首页后端与 Admin 后端页同源能力 | personal 首页增改（若环境允许）与 admin `/backends` 对照 | 字段/操作同源（Provider 面板），非两套表单 |

---

## 4. 执行流程（Agent）

### Step A — 准备

1. 读环境变量 / `.env`；探测 health + edition。  
2. **确认 Browser MCP 可用**（列出工具含 navigate/snapshot/click）。不可用 → 失败退出。  
3. 解析 `TEST_UI_ROLES`。

### Step B — 按角色循环

对每个角色：

1. 浏览器打开 Web → 登录。  
2. 按 §3 表格过滤「适用」用例，逐条执行。  
3. 每条：`snapshot → 操作 → snapshot → 判定`，写入结果数组。  
4. 登出（或清 cookie）再测下一角色。

### Step C — team_user 账号

若需 `team_user` 且未提供凭据：

1. 用 admin JWT（HTTP 允许）创建临时用户 + 密码 + 角色 normal。  
2. 浏览器登录该用户跑用例。  
3. 结束后删除用户。

（创建/删除可用 curl；**用例判定仍必须浏览器**。）

### Step D — 报告

写入：

- `/tmp/centag_ui_browser_results.json`（机器可读）  
- 对话内 Markdown 摘要（给人）

JSON 建议结构：

```json
{
  "deploy_type": "team",
  "base_url": "http://localhost:20060",
  "browser": "cursor-ide-browser",
  "started_at": "...",
  "finished_at": "...",
  "roles": [
    {
      "role": "team_admin",
      "cases": [
        {"id": "U01", "status": "pass|fail|skip|blocked", "evidence": "侧栏节点...", "notes": ""}
      ]
    }
  ],
  "summary": {"pass": 0, "fail": 0, "skip": 0, "blocked": 0}
}
```

**通过标准（交付）**：`blocked=0` 且 **P0 用例**（U01、U02、U03、U12、U14、U15）在适用角色上无 `fail`。其余 fail 须在报告说明是否可接受残余。

---

## 5. 与版本门禁的关系

| 工作流 | 要求 |
|--------|------|
| v0.2.6 Gate 3 → Gate 4 / Step 5 自测 | **必须**执行本正本（至少 `TEST_DEPLOY_TYPE` 覆盖实现所改 edition；team 改动则 admin+user 都跑） |
| `centag-admin-e2e` | 仍跑 HTTP 管理回归；**不能替代**本正本 |
| `centag-wizard-test` | 可选；不包含侧栏信息架构断言 |

版本目录可附裁剪清单：`docs/versions/<ver>/<需求>/UI测试流程.md`（指向本正本 + 版本特有备注）。

---

## 6. 触发词

`UI浏览器测试` / `ui browser test` / `WebUI 验收` / `浏览器验收` / `centag ui test`

---

## 7. 变更记录

| 日期 | 变更 |
|------|------|
| 2026-07-20 | 初版：配合 v0.2.6 普通用户 WebUI 能力矩阵；强制 Browser MCP |

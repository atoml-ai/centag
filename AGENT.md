# Centag — Agent 入口

→ 项目总纲与约束见 **[docs/harness/AGENTS.md](docs/harness/AGENTS.md)**。  
→ 技能正本目录见 **[docs/harness/skills/README.md](docs/harness/skills/README.md)**。

---

## Skill 分层原则（所有 Agent 必须遵守）

```
项目根 AGENT.md（本文件）
  标明：正本在哪、入口在哪、如何交接
        │
        ├──────────────────────┐
        ▼                      ▼
docs/harness/skills/      各 Agent 规范目录
【唯一业务正本】            【仅交互 / 收参】
步骤、判定、脚本、API       .cursor/rules/*.mdc
固化映射、报告格式          .opencode/skills/*/SKILL.md
        ▲                  .codebuddy/skills/*/ … 
        └──── 收参后交接执行 ────┘
```

### 1. 业务正本只放 `docs/harness/skills/`

- 测试流程、判定标准、预置脚本、环境变量语义、固化映射表等**业务定义**只写在此处。
- 修改技能行为 → **只改** `docs/harness/skills/` 下对应正本（及同目录脚本）。
- **禁止**在 `.cursor/`、`.opencode/` 等目录复制或改写业务步骤。

### 2. Agent 目录只放交互引导

不同 Agent 的控件不同（Cursor 的 `AskQuestion`、OpenCode 的 `question`、输入框/多选等），因此：

| 目录 | 职责 |
|------|------|
| `.cursor/rules/*.mdc` | Cursor：触发词、AskQuestion 表单、默认值读取、导出环境变量后交接 |
| `.opencode/skills/*/SKILL.md` | OpenCode：触发词、`question` 表单、交接 |
| 其它 Agent 目录同理 | 仅交互适配，不写业务正本 |

入口文件收集齐参数后，**统一加载并执行** `docs/harness/skills/` 下对应正本。

### 3. 交接契约

1. 入口探测服务 / 读 `.env`（可静默）。
2. 用该 Agent 的控件收集参数。
3. 导出环境变量（或等价上下文）。
4. 打开并严格执行正本，例如 `docs/harness/skills/centag-wizard-test.md`。
5. 失败只汇报 HTTP/接口错误；不在入口层发明新流程。

### 4. 现有技能索引（正本）

| 场景 | 正本路径 |
|------|----------|
| 向导式全面测试 | `docs/harness/skills/centag-wizard-test.md` |
| 流水线模式测试 | `docs/harness/skills/centag-pipeline-test.md` |
| 管理功能 E2E（HTTP） | `docs/harness/skills/centag-admin-e2e.md` |
| WebUI 浏览器自动化验收 | `docs/harness/skills/centag-ui-browser-test.md`（强制 Browser MCP） |
| 工作流 Step 1–5 / 门禁 | `docs/harness/skills/step*/SKILL.md`、`quality-gate/SKILL.md`（Step 1 含强制开发风险评估） |
| 核心操作 / 部署 | `centag-core.md` / `centag-deploy.md`（待补） |
| GitHub Release 发版 | `docs/harness/skills/centag-release.md` |

入口映射明细见 [docs/harness/skills/README.md](docs/harness/skills/README.md)。

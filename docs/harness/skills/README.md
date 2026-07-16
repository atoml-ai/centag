# Centag 技能目录

> 分层原则见仓库根目录 **[AGENT.md](../../../AGENT.md)**。

## 分层（强制）

| 层级 | 路径 | 内容 | 禁止 |
|------|------|------|------|
| **业务正本** | `docs/harness/skills/`（本目录） | 执行步骤、判定、脚本、固化映射、报告 | Agent 专用控件伪代码当唯一实现 |
| **交互入口** | `.cursor/`、`.opencode/`、`.codebuddy/` … | 触发词、该 Agent 的提问控件、收参、导出 env、交接正本 | 复制/改写业务步骤 |
| **总指引** | 根目录 `AGENT.md` | 标明正本与入口关系 | — |

**流程**：Agent 入口用各自控件收集参数 → 导出环境变量 → **加载本目录正本执行**。

```
用户触发「向导测试」
    → .cursor/rules/centag-wizard-test.mdc   （AskQuestion 收参）
    → docs/harness/skills/centag-wizard-test.md + wizard-test.sh
```

---

## 正本文件

### 研发工作流

| 文件 | 说明 |
|------|------|
| `step1-design/SKILL.md` | Step 1: 方案设计与确认 |
| `step2-plan/SKILL.md` | Step 2: 任务规划 |
| `step3-code/SKILL.md` | Step 3: SDD 编码实现 |
| `step4-test/SKILL.md` | Step 4: 单元测试补全 |
| `step5-review/SKILL.md` | Step 5: CR 审查 |
| `quality-gate/SKILL.md` | 门禁检查 |

### 项目操作 / 测试

| 文件 | 说明 | 预置脚本 |
|------|------|----------|
| `centag-pipeline-test.md` | 流水线模式测试 | — |
| `centag-wizard-test.md` | 向导式全面测试 | `wizard-test.sh`、`wizard-report.py` |
| `centag-admin-e2e.md` | 管理功能 E2E | `admin-e2e-test.sh` |
| `centag-core.md` | 核心操作（**待补**） | — |
| `centag-deploy.md` | 向导式部署（**待补**） | `deploy-wizard.sh` |

本目录下**不再**放置各 Agent 的交互入口 `*/SKILL.md`（已迁至 `.opencode/skills/` 等）。工作流 `step*/SKILL.md` 本身即业务正本，保留。

---

## 入口 → 正本映射

### Cursor（`.cursor/rules/`）

| 入口（仅交互） | 正本 |
|----------------|------|
| `harness-ask-ui.mdc` | 强制 AskQuestion，不写业务 |
| `centag-wizard-test.mdc` | `centag-wizard-test.md` |
| `centag-pipeline-test.mdc` | `centag-pipeline-test.md` |
| `centag-admin-e2e.mdc` | `centag-admin-e2e.md` |
| `harness-workflow.mdc`（待补） | `step*/SKILL.md` + `quality-gate` |
| `centag-core.mdc` / `centag-deploy.mdc`（待补） | 对应正本 |

### OpenCode（`.opencode/skills/`）

| 入口（仅交互） | 正本 |
|----------------|------|
| `centag-wizard-test/SKILL.md` | `centag-wizard-test.md` |
| `centag-pipeline-test/SKILL.md` | `centag-pipeline-test.md` |
| `centag-admin-e2e/SKILL.md` | `centag-admin-e2e.md` |

其它 Agent（CodeBuddy / WorkBuddy / …）按同样模式：在各自目录建交互入口，**禁止**在本目录再写一份业务。

---

## 维护规则

1. 改测试步骤 / 判定 / 脚本 → 只改本目录正本。
2. 改「怎么问用户」→ 只改对应 Agent 目录入口。
3. 新增技能：先写本目录正本，再为需要的 Agent 各加一个薄入口。
4. 根目录 `AGENT.md` 变更时，同步检查本 README 与 `docs/harness/AGENTS.md` §9。

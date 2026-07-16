# Centag 技能目录

**`skills/` 是唯一正本**，仅含纯技能内容（无 frontmatter、无 agent 关联）。

各 Agent 的入口文件在各自目录下，明文引用指向 `skills/` 下的正本。

## 正本文件

### 研发工作流技能（Workflow Steps）

| 文件 | 说明 |
|------|------|
| `skills/step1-design/SKILL.md` | Step 1: 方案设计与确认 |
| `skills/step2-plan/SKILL.md` | Step 2: 任务规划 |
| `skills/step3-code/SKILL.md` | Step 3: SDD 编码实现 |
| `skills/step4-test/SKILL.md` | Step 4: 单元测试补全 |
| `skills/step5-review/SKILL.md` | Step 5: CR 审查 |
| `skills/quality-gate/SKILL.md` | 门禁检查 |

### 项目操作技能（Project Operations）

| 文件 | 说明 |
|------|------|
| `skills/centag-core.md` | 核心操作：构建、部署、运维、调试 |
| `skills/centag-deploy.md` | 向导式部署：支持本地/Docker/Profile多种模式 |
| `skills/centag-pipeline-test.md` | 流水线模式测试 |
| `skills/centag-wizard-test.md` | 向导式全面测试 |
| `skills/centag-admin-e2e.md` | 管理功能端到端测试 |

## 入口 → 正本 映射

### Cursor（本项目的 AI Agent 入口）

| 入口规则文件 | 引用正本 |
|-------------|----------|
| `.cursor/rules/harness-baseline.mdc` | 内联（alwaysApply） |
| `.cursor/rules/harness-workflow.mdc` | `skills/step*-design\|plan\|code\|test\|review/SKILL.md` + `skills/quality-gate/SKILL.md` |
| `.cursor/rules/centag-core.mdc` | `skills/centag-core.md` |
| `.cursor/rules/centag-deploy.mdc` | `skills/centag-deploy.md` |
| `.cursor/rules/centag-pipeline-test.mdc` | `skills/centag-pipeline-test.md` |
| `.cursor/rules/centag-wizard-test.mdc` | `skills/centag-wizard-test.md` |
| `.cursor/rules/centag-admin-e2e.mdc` | `skills/centag-admin-e2e.md` |

### 其它 Agent（通过 skills/ 下的 SKILL.md 入口引用正本）

| Agent | 入口文件 | 引用正本 |
|-------|---------|----------|
| **OpenCode** | `.opencode/skills/centag-core/SKILL.md` | `skills/centag-core.md` |
| | `.opencode/skills/centag-deploy/SKILL.md` | `skills/centag-deploy.md` |
| | `.opencode/skills/centag-pipeline-test/SKILL.md` | `skills/centag-pipeline-test.md` |
| | `.opencode/skills/centag-wizard-test/SKILL.md` | `skills/centag-wizard-test.md` |
| | `.opencode/skills/centag-admin-e2e/SKILL.md` | `skills/centag-admin-e2e.md` |
| **WorkBuddy** | `.workbuddy/skills/centag-core/SKILL.md` | `skills/centag-core.md` |
| | `.workbuddy/skills/centag-deploy/SKILL.md` | `skills/centag-deploy.md` |
| | `.workbuddy/skills/centag-pipeline-test/SKILL.md` | `skills/centag-pipeline-test.md` |
| | `.workbuddy/skills/centag-wizard-test/SKILL.md` | `skills/centag-wizard-test.md` |
| | `.workbuddy/skills/centag-admin-e2e/SKILL.md` | `skills/centag-admin-e2e.md` |
| **OpenClaw** | `archive/deprecated/agents/openclaw/skills/centag-core/SKILL.md` | `skills/centag-core.md` |
| | `archive/deprecated/agents/openclaw/skills/centag-deploy/SKILL.md` | `skills/centag-deploy.md` |
| | `archive/deprecated/agents/openclaw/skills/centag-pipeline-test/SKILL.md` | `skills/centag-pipeline-test.md` |
| **Hermes Agent** | `archive/deprecated/agents/hermes-agent/skills/centag-core/SKILL.md` | `skills/centag-core.md` |
| | `archive/deprecated/agents/hermes-agent/skills/centag-deploy/SKILL.md` | `skills/centag-deploy.md` |
| | `archive/deprecated/agents/hermes-agent/skills/centag-pipeline-test/SKILL.md` | `skills/centag-pipeline-test.md` |
| **WPS-Comate** | `.wps-comate/skills/centag-core/SKILL.md` | `skills/centag-core.md` |
| | `.wps-comate/skills/centag-deploy/SKILL.md` | `skills/centag-deploy.md` |
| | `.wps-comate/skills/centag-pipeline-test/SKILL.md` | `skills/centag-pipeline-test.md` |
| **CodeBuddy** | `.codebuddy/skills/centag-core/SKILL.md` | `skills/centag-core.md` |
| | `.codebuddy/skills/centag-deploy/SKILL.md` | `skills/centag-deploy.md` |
| | `.codebuddy/skills/centag-pipeline-test/SKILL.md` | `skills/centag-pipeline-test.md` |
| | `.codebuddy/skills/centag-wizard-test/SKILL.md` | `skills/centag-wizard-test.md` |
| | `.codebuddy/skills/centag-admin-e2e/SKILL.md` | `skills/centag-admin-e2e.md` |

## 维护规则

- 修改技能 → 编辑 `skills/` 下正本
- 入口引用由各 Agent 的 AI 自动跟随
- 禁止在 `skills/` 外定义技能实现内容
- `.cursorrules` 是 Cursor 独立规则文件，不是技能入口；技能入口在 `.cursor/rules/*.mdc`

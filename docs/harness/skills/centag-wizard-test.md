# Centag Wizard Test — 正本

> 本文件为向导式全面测试的**唯一业务正本**（路径：`docs/harness/skills/`）。
> **交互入口**在各 Agent 目录（如 `.cursor/rules/centag-wizard-test.mdc`、`.opencode/skills/centag-wizard-test/`），只负责用该 Agent 控件收参；收齐后交接本文件执行。
> 分层原则见仓库根目录 **`AGENT.md`**。入口层禁止复制本文件业务步骤，也禁止用聊天正文列 A/B/C 代替控件。
>
> ## 🔴 最高原则：不分析代码 / 不生成脚本
> - **整个测试过程，任何时候都不允许分析项目代码**
> - 禁止 grep/search/read 查找源码
> - 禁止推理根因（"可能是 XX 模块的问题"）
> - 只汇报 HTTP 状态码 + 错误消息；失败则用 AskQuestion 让用户选择下一步
> - **禁止当场生成 shell/python 脚本**——所有脚本已预置在 skill 目录中：
>   - `wizard-test.sh` — Steps A–F（健康探测 → 流水线测试）
>   - `wizard-report.py` — Step G（HTML 报告生成）
>   - `wizard-report.css` — 报告样式表

---

## 前置条件（入口层必须提供的参数）

所有参数由入口层注入环境变量，本文件不再自行发现：

| 参数 | 环境变量 | 必需 | 说明 |
|------|----------|------|------|
| 产品版本 | `CENTAG_DEPLOY_TYPE` | ✅ | `gateway` / `team` / `minimal` / `personal` |
| 服务地址 | `TEST_BASE_URL` | ✅ | 如 `http://localhost:20060` |
| JWT Token | `TEST_JWT_TOKEN` | ✅ | 通过 POST /api/auth/login 获取的 access_token |
| Admin 用户名 | `ADMIN_USERNAME` | ✅ | 用于 JWT 过期时自动重新登录 |
| Admin 密码 | `ADMIN_PASSWORD` | ✅ | 用于 JWT 过期时自动重新登录 |
| 用户 Key | `TEST_USER_KEY` | 仅 team | 测试用户的 API Key |
| Minimal 密码 | `ADMIN_PASSWORD` | minimal | 单密码登录；用户名固定 `admin` |
| 后端 ID | `TEST_BACKEND_ID` | 仅 pipeline/all | 如 `bigmodel`、`deepseek`、`openai`、`ollama-local` |
| 后端类型 | `TEST_BACKEND_TYPE` | 仅 pipeline/all | `openai` 或 `ollama` |
| 后端 Base URL | `TEST_BACKEND_BASE_URL` | 仅 pipeline/all | 如 `https://open.bigmodel.cn/api/paas/v4` |
| 后端 Key | `TEST_BACKEND_KEY` | 仅 pipeline/all | `real` 模式下必填；`mock` 模式默认 `fake-e2e-key`（可选自定义） |
| 后端模型 | `TEST_BACKEND_MODEL` | 仅 pipeline/all | 如 `glm-4-flash`、`qwen2.5:1.5b` |
| 后端来源模式 | `TEST_BACKEND_SOURCE` | 仅 pipeline/all | `real`（默认，真实后端）或 `mock`（本地 fake-openai） |
| Mock Host | `TEST_MOCK_OPENAI_HOST` | 仅 mock + pipeline/all | 默认 `127.0.0.1` |
| Mock Port | `TEST_MOCK_OPENAI_PORT` | 仅 mock + pipeline/all | 默认 `28081` |
| 流水线列表 | `TEST_PIPELINES` | 仅 pipeline/all | 逗号分隔的 pipeline_id |
| 入口变体矩阵 | `TEST_ENTRY_VARIANTS` | 可选 | `header-full,model-pipeline,prompt-shortcut`（默认仅 `header-full`） |
| 每入口重复次数 | `TEST_REPEAT_PER_VARIANT` | 可选 | 默认 `1`，完整回归建议 `3` |
| 日志证据级别 | `TEST_LOG_EVIDENCE_LEVEL` | 可选 | `basic` 或 `full`（默认 `basic`） |
| 临时后端兜底 | `TEST_TEMP_BACKEND_FALLBACK` | 可选 | `auto`（默认）或 `off` |
| 临时后端前缀 | `TEST_TEMP_BACKEND_PREFIX` | 可选 | 默认 `<TEST_BACKEND_ID>-temp` |
| 管理功能测试 | `TEST_ADMIN_E2E` | 可选 | `true` 或 `false`（默认 `false`） |

---

## 凭据自动发现与重试（入口层必须执行）

为减少阻塞，入口层在用户选择"从环境变量或配置文件读取"时，必须按以下顺序自动尝试：

1. 读取 `config/secrets/.env`（若存在），并与当前进程环境合并。
2. 用户名优先级：`ADMIN_USERNAME` → `LLM_PROXY_ADMIN_USERNAME` → `admin`。
3. 密码优先级：`ADMIN_PASSWORD` → `LLM_PROXY_ADMIN_PASSWORD`。
4. 登录策略：
   - 先验证现有 `TEST_JWT_TOKEN`（`GET /api/auth/me`）。
   - 若 token 无效，再用自动发现的用户名/密码登录。
   - 仅在上述自动链路全部失败后，才要求用户介入（重试/跳过/终止）。

---

## 固化知识：内建映射表

### API 端点（已知，固定）

```bash
# 认证
POST /api/auth/login                                  → 登录，获取 JWT access_token
GET  /api/auth/me                                     → 验证 JWT 有效性

# 健康检查（无需认证）
GET  /health                                          → 健康检查

# 用户管理（需 JWT + Admin 角色，团队版专用）
GET  /api/v1/admin/users?username=xxx                 → 查询用户
POST /api/v1/admin/users                              → 创建用户 (username, password, role)
POST /api/v1/api-keys                                 → 生成用户 API Key (user_id, name)

# 后端管理（需 JWT + proxyAuth）
GET  /api/v1/backends/:id                             → 获取后端详情
PUT  /api/v1/backends/:id                             → 更新后端配置（enabled, api_key, ...）
POST /api/v1/backends/:id/probe                       → 探测后端连接（验证 Key）

# 流水线管理（需 JWT + proxyAuth）
GET  /api/v1/pipelines/:id                            → 获取流水线完整详情
PUT  /api/v1/pipelines/:id                            → 更新流水线（完整对象，至少含 name + nodes）

# LLM 代理（需 API Key 或 JWT）
POST /v1/chat/completions                             → LLM 代理入口
```

### 内建后端列表（已固化）

| 后端 ID | 名称 | 默认模型 | Base URL | 类型 |
|---------|------|---------|----------|------|
| `bigmodel` | 智谱 AI | `glm-4-flash` | `https://open.bigmodel.cn/api/paas/v4` | openai |
| `deepseek` | DeepSeek | `deepseek-chat` | `https://api.deepseek.com/v1` | openai |
| `openai` | OpenAI | `gpt-4o-mini` | `https://api.openai.com/v1` | openai |
| `siliconflow` | 硅基流动 | `deepseek-ai/DeepSeek-V3` | `https://api.siliconflow.cn/v1` | openai |
| `alibaba-dashscope` | 阿里云百炼 | `qwen-plus` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | openai |
| `ppio` | PPIO 派欧云 | `qwen/qwen2.5-32b-instruct` | `https://api.ppinfra.com/v3` | openai |
| `stepfun` | 阶跃星辰 | `step-1-flash` | `https://api.stepfun.com/v1` | openai |
| `ollama-local` | Ollama (本地) | `qwen2.5:1.5b` | `http://localhost:21434` | ollama |

### pipeline_id → X-Proxy-Mode 映射（已固化）

```
smart-scheduling  → smart-scheduling
direct-backend    → direct-backend
fallback-mode     → fallback-mode
router-mode       → router-mode
optimize-mode     → optimize-mode
audit-mode        → audit-mode
aggregator-mode   → aggregator-mode
translate-mode    → translate-mode
model-matching    → model-matching
transparent-fast  → transparent-fast
security-mode     → security-mode
```

### 构造 Chat Completions 请求体（已固化）

```json
{
  "model": "<TEST_BACKEND_MODEL>",
  "messages": [{"role": "user", "content": "用一句话介绍你自己"}],
  "max_tokens": 50
}
```

测试问题统一使用：`"用一句话介绍你自己"`

### 测试判定标准（已固化）

| 判定 | 条件 |
|------|------|
| ✅ 通过 | HTTP 200 + `choices[0].message.content` 非空 或 `usage.total_tokens > 0` |
| ❌ 失败 — 后端错误吞没 | HTTP 200 + content 空 + tokens = 0（网关吞掉了后端返回的真实错误） |
| ❌ 失败 | HTTP 非 200 |

### 产品版本 → 测试差异

| | gateway / personal | team | minimal |
|---|---|---|---|
| 默认端口 | 20060 | 可变 | 20060 |
| 启动方式 | `./start.sh run be` 或 profile | `./start.sh profile team up` | `./start.sh debug --minimal` |
| 创建用户 | 跳过（Admin JWT） | 必须 | 无多用户；单密码 |
| 测试凭据 | JWT (`TEST_JWT_TOKEN`) | 用户 Key (`TEST_USER_KEY`) | JWT；`/v1` 可选 API Key |
| Admin E2E | 后端/流水线/健康等 | 含用户/租户/成本 | 精简：auth + backends + pipelines + settings/api-keys |


---

## 执行流程

入口层收集完所有参数并注入环境变量后，按以下顺序执行：

### 0) 必选：测试类型分流（向导首步）

入口层必须先收集 `test_type`：

1. `pipeline`：仅流水线模式测试
2. `admin`：仅管理功能测试
3. `all`：流水线 + 管理功能

分流约束：

- 当 `test_type=admin`：
  - 跳过后端来源、后端 ID、模型、API Key、Pipeline 范围采集。
  - 设定 `TEST_ADMIN_E2E=true`。
  - `TEST_PIPELINES` 可留空。
- 当 `test_type=pipeline`：
  - 必须采集后端与 pipeline 相关参数。
  - 设定 `TEST_ADMIN_E2E=false`。
- 当 `test_type=all`：
  - 必须采集后端与 pipeline 相关参数。
  - 设定 `TEST_ADMIN_E2E=true`。

### 0.5) 可选：入口变体测试选项（向导新增）

在正式执行前，入口层应通过交互题让用户选择：

1. 是否启用"模式完整回归矩阵"（请求头全称 / 快捷码头 / Prompt 快捷码）
2. 每个入口重复次数（建议 `1` 或 `3`）
3. 日志证据强度：
   - `basic`：只看 HTTP + 响应头
   - `full`：额外拉取 `/api/v1/logs` 时间窗证据（`Resolved pipeline` + `pipeline execution finished`）
4. 是否启用"临时后端兜底"：
   - `auto`：检测到熔断/空响应时自动创建临时后端（推荐）
   - `off`：禁用自动兜底，仅报告失败

建议默认：

```bash
export TEST_ENTRY_VARIANTS="header-full"
export TEST_REPEAT_PER_VARIANT="1"
export TEST_LOG_EVIDENCE_LEVEL="basic"
export TEST_TEMP_BACKEND_FALLBACK="auto"
export TEST_BACKEND_SOURCE="real"
export TEST_ADMIN_E2E="false"  # test_type=pipeline
```

若用户选择完整回归：

```bash
export TEST_ENTRY_VARIANTS="header-full,model-pipeline,prompt-shortcut"
export TEST_REPEAT_PER_VARIANT="3"
export TEST_LOG_EVIDENCE_LEVEL="full"
export TEST_TEMP_BACKEND_FALLBACK="auto"
export TEST_BACKEND_SOURCE="real"
export TEST_ADMIN_E2E="true"  # test_type=all
```

### 0.1) 后端来源模式（仅 `test_type=pipeline|all`）

向导应让用户二选一：

1. **真实后端大模型（`TEST_BACKEND_SOURCE=real`）**
   - 使用用户提供的真实 API Key / Base URL
   - 行为最贴近生产，但受外部配额、限流、网络波动影响
2. **Mock 服务（`TEST_BACKEND_SOURCE=mock`）**
   - 使用本地 `fake_openai_server.py`，不依赖外网
   - 适合稳定回归与 CI 场景
   - 默认参数：
     - `TEST_MOCK_OPENAI_HOST=127.0.0.1`
     - `TEST_MOCK_OPENAI_PORT=28081`
   - API Key 策略：
     - 默认自动使用 `TEST_BACKEND_KEY=fake-e2e-key`
     - 仅在用户明确要求时，才采集自定义 fake key

### 第一步：按测试类型执行

#### 1A. `test_type=pipeline|all`：运行流水线测试脚本

```bash
# 设置好上述所有环境变量后，直接执行预置脚本
bash docs/harness/skills/wizard-test.sh
```

该脚本自动执行 Steps A–F：
- **Step A**: 健康探测 (`GET /health`)
- **Step B**: 验证 JWT + 确定认证方式（gateway/personal/minimal 用 JWT，team 用用户 Key）
- **Step B2**: 团队版验证测试用户
- **Step B4**: 根据 `TEST_BACKEND_SOURCE` 选择真实/Mock 后端模式（Mock 模式自动拉起本地 fake-openai，并自动备份/恢复目标后端配置）
- **Step C**: 配置后端 (`PUT /api/v1/backends/:id`)
- **Step D**: 探测后端连接 (`POST /api/v1/backends/:id/probe`)
- **Step E**: 更新流水线节点 (`GET` → jq 替换 → `PUT`)
- **Step F**: 逐条执行流水线测试 (`POST /v1/chat/completions`)

输出数据文件：
- `/tmp/wizard_test_data.json` — 测试结果 JSON
- `/tmp/wizard_pipeline_update_cmds.json` — 流水线更新记录
- `/tmp/wizard_probe.json` — 后端探测结果

#### 1B. `test_type=admin`：跳过流水线脚本，直接执行管理功能测试

- 不执行 `wizard-test.sh`
- 不要求后端来源/后端 ID/模型/API Key/Pipeline 配置
- 仅保留部署、服务地址、Admin 凭据、JWT（team 模式仍按管理功能测试要求处理用户上下文）

### （可选）Step G：管理功能 E2E 测试

当 `TEST_ADMIN_E2E=true` 时，执行管理功能测试：

```bash
# 执行预置管理功能测试脚本（自动处理 JWT 复用/刷新）
bash docs/harness/skills/admin-e2e-test.sh
```

管理功能测试覆盖范围（按 edition 裁剪，见 `centag-admin-e2e.md`）：
- 通用：后端、流水线、健康检查
- gateway/team：用户、API Key、Token 用量、配置、Profile；Agent 供应商可选
- team：多租户、成本看板
- minimal：登录/改密探测、`/api/v1/settings/api-keys`、后端、流水线、健康检查

输出数据文件：
- `/tmp/admin_e2e_results.json` — 管理功能测试结果

### 第二步：生成报告

```bash
# pipeline/all：在 wizard-test.sh（及可选 admin-e2e）执行后运行
# admin：在 admin-e2e-test.sh 执行后运行（自动渲染管理测试详情）
python3 docs/harness/skills/wizard-report.py [--output /path/to/report.html]
```

报告包含：概览卡片、后端配置详情、流水线节点配置表、逐条测试详情（含 curl 命令和响应）、复测指南、故障分析总结。

### 第三步：打开并汇报

```bash
open /tmp/wizard_test_report_*.html
```

向用户汇报结果摘要（通过数/失败数/通过率）和报告路径。

### （可选）第四步：模式入口矩阵补充验证

当 `TEST_ENTRY_VARIANTS` 包含多个入口时，对每个 `pipeline_id` 执行补充验证：

1. 入口变体：`header-full` / `model-pipeline` / `prompt-shortcut`
2. 每变体循环 `TEST_REPEAT_PER_VARIANT` 次
3. 每次记录：HTTP 状态码、`X-Proxy-Mode`、`X-Pipeline-Id`、`X-Backend-Id`、耗时
4. 当 `TEST_LOG_EVIDENCE_LEVEL=full` 时，追加日志 API 证据：
   - `q=Resolved pipeline: <pipeline_id>`
   - `q=pipeline execution finished` 且 `extra.pipeline_id=<pipeline_id>`
5. 报告新增证据字段：
   - `finished总数`、`finished匹配数`
   - `主 request_id`（用于定位单次请求）
   - `证据强度`（`high` / `medium` / `low`）

该补充验证用于判断“入口兼容性”而非替代主流程 Steps A-F。

### 安全与风险控制（临时后端兜底）

当检测到后端熔断/空响应时，脚本可自动创建临时后端 ID（同 base_url + key）继续验证。约束如下：

1. 不打印明文 API Key（报告中始终脱敏）
2. 仅在出现明确异常信号时触发（例如 `circuit breaker open`、HTTP 500/503、HTTP 200 且 content/tokens 为空且 `X-Cache-Read!=true`）
3. 报告中必须记录 source/final 后端 ID 与触发原因，保证审计可追溯
4. 若用户不接受自动创建资源，可设置 `TEST_TEMP_BACKEND_FALLBACK=off`

---

## 故障排查（固化）

**原则：只汇报症状，不分析代码。**

| 症状 | 原因 | 解决 |
|------|------|------|
| `/health` 不通 | 服务未启动 | gateway: `./start.sh run be`；team: `./start.sh profile team up`；minimal: `./start.sh debug --minimal` |
| JWT 登录失败 | 用户名或密码错误 | 报告错误消息，用 ask_followup_question 让用户重试 |
| JWT 401 "invalid token" | token 过期或字段错误 | 用 `/api/auth/login` 重新获取 `access_token` |
| API 返回 401 | JWT 过期 | wizard-test.sh Step B 会自动重新登录 |
| 后端 PUT 后 probe 报 "backend type is required" | 缺少 `type` 字段 | 确保 `TEST_BACKEND_TYPE` 已设为 `openai` 或 `ollama` |
| 后端 PUT 后 probe 报 "base URL is empty" | 缺少 `base_url` 字段 | 确保 `TEST_BACKEND_BASE_URL` 已设置 |
| probe 返回 success=false | Key 无效或后端不可达 | 报告错误，ask_followup_question 让用户重输 Key 或换后端 |
| HTTP 200 + 空响应 + tokens=0 | 后端错误被吞没（Key 无效/余额不足/模型不存在/限流） | 用 curl 直连后端 API 验证 |
| 流水线 PUT 返回 400 | 请求体格式不完整 | 可能缺少 `name` 字段（脚本已处理：GET 完整对象 → PUT） |
| 所有流水线返回 500 | 后端不可用 | 报告 HTTP 状态码 + 错误消息 |
| `jq` 解析错误 | 响应格式异常 | 查看 `/tmp/wizard_test_*.json` 原始内容 |

### 对应文件

如需深入测试特定流水线，加载：`docs/harness/skills/centag-pipeline-test.md`

### 预置脚本位置

| 脚本 | 用途 |
|------|------|
| `docs/harness/skills/wizard-test.sh` | Steps A–F：健康探测 → 配置后端 → 流水线测试 |
| `docs/harness/skills/admin-e2e-test.sh` | 管理功能 E2E：输出 `/tmp/admin_e2e_results.json` |
| `docs/harness/skills/wizard-report.py` | Step G：生成 HTML 测试报告 |
| `docs/harness/skills/wizard-report.css` | 报告样式表 |

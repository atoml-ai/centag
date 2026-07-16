# Centag Pipeline Test — 正本

> **业务正本**（`docs/harness/skills/`）。交互入口在 `.cursor/rules/centag-pipeline-test.mdc`、`.opencode/skills/centag-pipeline-test/` 等；收参后交接本文件。分层见根目录 **`AGENT.md`**。
> 调用方负责：探测服务、交互确认启动、日志记录。本文件仅描述服务已就绪后的测试步骤。

## 适用范围

- 适用于 **gateway / team / minimal / personal** 等仍暴露 `/v1/chat/completions` 与流水线管理的发行版。
- 不覆盖已移除的 business Agent / Mem0 / RAG 流水线。
- 优先通过 `/api/v1/logs` 取证；仅在 API 不可用时回退本地 `logs/*.log`。

## 前置条件（由入口层保证）

- Centag 服务正在运行，健康检查通过
- Admin API Key 已获取
- 工作目录在项目根目录

## 测试原则

**核心原则**：不仅要验证表面结果（HTTP 200 + 响应内容），更要验证内部执行流程是否符合设计。

**关键验证点**：
1. 执行顺序是否正确（特别是路由模式）
2. 节点选择是否符合设计（是否只执行了预期的节点）
3. 是否有意外的降级行为
4. 依赖关系是否正确

## 模式完整回归（新增）

当目标是验证某个模式在不同入口下行为一致（而不仅是“能返回 200”）时，使用以下矩阵：

### 入口矩阵

对每个模式，按以下 3 种入口分别测试：

1. `Header-Full`：`X-Proxy-Mode: <pipeline_id>`
2. `Header-Shortcut`：`X-Proxy-Mode: <shortcut_code>`（例如 `#d`）
3. `Prompt-Shortcut`：在用户消息前缀写 `<shortcut_code>`（例如 `#d 你好`）

每个入口建议重复 `3` 次，固定相同请求体（`temperature=0`）以便可比。

### 证据采集（四类）

每次请求都记录：

1. **请求证据**：入口类型、关键请求头、请求时间窗
2. **响应证据**：HTTP 状态码、`X-Proxy-Mode`、`X-Pipeline-Id`、`X-Backend-Id`、`X-Pipeline-Success`
3. **性能证据**：`curl time_total` 或等效耗时
4. **日志证据**：通过日志 API 在请求时间窗内检索
   - `q=Resolved pipeline: <pipeline_id>`
   - `q=pipeline execution finished`（并检查 `extra.pipeline_id`）

### 日志检索方式（优先 API，不读文件）

```bash
# 读取时间窗内的模式解析日志
curl -s -G "http://localhost:20060/api/v1/logs" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  --data-urlencode "category=llm" \
  --data-urlencode "limit=100" \
  --data-urlencode "from=$WINDOW_FROM" \
  --data-urlencode "to=$WINDOW_TO" \
  --data-urlencode "q=Resolved pipeline: direct-backend"

# 读取执行完成日志并检查 extra.pipeline_id
curl -s -G "http://localhost:20060/api/v1/logs" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  --data-urlencode "category=llm" \
  --data-urlencode "limit=100" \
  --data-urlencode "from=$WINDOW_FROM" \
  --data-urlencode "to=$WINDOW_TO" \
  --data-urlencode "q=pipeline execution finished"
```

### 通过判定（建议）

单次请求判定通过需同时满足：

1. `HTTP 200`
2. `X-Proxy-Mode` 等于目标模式（例如 `direct-backend`）
3. `X-Pipeline-Id` 等于目标 pipeline（例如 `direct-backend`）
4. 日志时间窗内存在目标模式证据（`Resolved pipeline` 或 `pipeline execution finished` 且 `pipeline_id` 匹配）

整组矩阵通过建议：

- 每个入口 `3/3` 通过，或至少 `2/3` 通过且无系统性偏差；
- 若 `Prompt-Shortcut` 存在偶发日志缺失，以响应头 `X-Pipeline-Id` + `pipeline execution finished` 作为主证据。

### 直连模式基线（2026-06 验证）

- `Header-Full`：`3/3` 成功，日志证据完整
- `Header-Shortcut`：`3/3` 成功，日志证据完整
- `Prompt-Shortcut`：`3/3` 成功；`Resolved pipeline` 可能偶发缺失，但 `X-Pipeline-Id=direct-backend` 与 `pipeline execution finished(pipeline_id=direct-backend)` 稳定存在

## 测试流程

### Step 1: 测试前准备

```bash
# 获取 Admin Key
ADMIN_KEY=$(grep 'LLM_PROXY_ADMIN_API_KEY' config/secrets/.env | cut -d= -f2)
[ -z "$ADMIN_KEY" ] && ADMIN_KEY=$(grep 'LLM_PROXY_DEFAULT_ADMIN_API_KEY' config/secrets/.env | cut -d= -f2)

# 记录测试开始时间（用于日志分析）
TEST_START_TIME=$(date '+%Y-%m-%dT%H:%M:%S')
echo "测试开始时间: $TEST_START_TIME"

# 清空或标记日志基线（可选）
# echo "--- 测试开始 $TEST_START_TIME ---" >> logs/centag.log

# 获取后端和流水线配置
curl -s -H "Authorization: Bearer $ADMIN_KEY" http://localhost:20060/api/v1/backends > /tmp/backends.json
curl -s -H "Authorization: Bearer $ADMIN_KEY" http://localhost:20060/api/v1/pipelines > /tmp/pipelines.json

# 检查关键后端状态
echo "=== 后端状态检查 ==="
cat /tmp/backends.json | jq -r '.data[] | "\(.id): enabled=\(.enabled), has_api_key=\(.has_api_key)"'
```

### Step 2: 执行测试

#### 2.1 基础功能测试（所有流水线）

```bash
# 基础测试用例
curl -s --max-time 20 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: <pipeline_id>" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "说一个字"}], "max_tokens": 5}' \
  > /tmp/test_basic.json

echo "=== 基础测试结果 ==="
cat /tmp/test_basic.json | jq .
```

#### 2.2 路由模式专项测试（router-mode）

```bash
# 测试用例1: 代码关键词 → 应路由到 code-generator
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: router-mode" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "写一段Python排序代码"}], "max_tokens": 100}' \
  > /tmp/test_router_code.json

sleep 2

# 测试用例2: 翻译关键词 → 应路由到 translate-gen
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: router-mode" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "翻译成英文：你好世界"}], "max_tokens": 50}' \
  > /tmp/test_router_translate.json

sleep 2

# 测试用例3: 普通对话 → 应路由到 chat-generator
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: router-mode" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "你好，请问今天天气怎么样？"}], "max_tokens": 100}' \
  > /tmp/test_router_chat.json

sleep 2

# 测试用例4: 摘要关键词 → 应路由到 summary-gen
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: router-mode" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "总结一下：机器学习是人工智能的一个子领域"}], "max_tokens": 100}' \
  > /tmp/test_router_summary.json

echo "=== 路由模式测试完成 ==="
```

#### 2.3 智能调度专项测试（smart-scheduling）

```bash
# 测试用例1: 简单问题 → 应走快速路径
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: smart-scheduling" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "1+1=?"}], "max_tokens": 10}' \
  > /tmp/test_smart_simple.json

sleep 2

# 测试用例2: 复杂问题 → 应走高质量路径
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: smart-scheduling" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "详细解释量子计算的原理和应用前景"}], "max_tokens": 200}' \
  > /tmp/test_smart_complex.json

echo "=== 智能调度测试完成 ==="
```

#### 2.4 降级模式专项测试（fallback-mode）

```bash
# 测试用例: 正常情况 → 应使用主后端
curl -s --max-time 30 -X POST "http://localhost:20060/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "X-Proxy-Mode: fallback-mode" \
  -H "Content-Type: application/json" \
  -d '{"model": "GLM-4.7-Flash", "messages": [{"role": "user", "content": "测试降级模式"}], "max_tokens": 50}' \
  > /tmp/test_fallback_normal.json

echo "=== 降级模式测试完成 ==="
```

### Step 3: 日志分析（关键步骤）

**必须检查日志**，验证内部执行流程：

```bash
echo "=== 日志分析 ==="

# 分析路由模式执行日志
echo "--- 路由模式日志分析 ---"
# 查找测试时间后的日志
grep -A 100 "pipeline execution started.*router-mode" logs/centag.log | \
  grep -E "(executing node|node execution completed|node skipped|selected_route|bypassing error)" | \
  tail -50

# 分析节点执行顺序
echo "--- 节点执行顺序分析 ---"
grep -E "executing node|node execution completed" logs/centag.log | \
  tail -20

# 检查是否有降级行为
echo "--- 降级行为检查 ---"
grep -E "bypassing error|bypass with fallback" logs/centag.log | \
  tail -10

# 检查错误日志
echo "--- 错误日志检查 ---"
grep -E "error.*node execution failed" logs/centag.log | \
  tail -10
```

#### 3.1 路由模式日志分析要点

对于路由模式，必须验证以下内容：

```bash
# 1. 验证classifier节点是否先执行
echo "=== 验证执行顺序 ==="
grep "pipeline execution started.*router-mode" -A 100 logs/centag.log | \
  grep "executing node.*classifier" -A 5

# 2. 验证路由决策
echo "=== 验证路由决策 ==="
grep "pipeline execution started.*router-mode" -A 100 logs/centag.log | \
  grep "selected_route"

# 3. 验证节点跳过情况
echo "=== 验证节点跳过 ==="
grep "pipeline execution started.*router-mode" -A 100 logs/centag.log | \
  grep "node skipped"

# 4. 验证是否只有预期节点执行
echo "=== 验证节点执行 ==="
grep "pipeline execution started.*router-mode" -A 100 logs/centag.log | \
  grep "executing node" | \
  grep -v "classifier"
```

#### 3.2 智能调度日志分析要点

```bash
# 1. 验证分支决策
echo "=== 验证分支决策 ==="
grep "pipeline execution started.*smart-scheduling" -A 100 logs/centag.log | \
  grep "selected_route"

# 2. 验证节点执行情况
echo "=== 验证节点执行 ==="
grep "pipeline execution started.*smart-scheduling" -A 100 logs/centag.log | \
  grep "executing node"
```

### Step 4: 结果验证

#### 4.1 基础功能验证

```bash
echo "=== 基础功能验证 ==="

# 验证HTTP状态码
HTTP_STATUS=$(cat /tmp/test_basic.json | jq -r '.error // empty')
if [ -n "$HTTP_STATUS" ]; then
  echo "❌ 基础测试失败: $HTTP_STATUS"
else
  echo "✅ 基础测试通过"
fi

# 验证响应内容
CONTENT=$(cat /tmp/test_basic.json | jq -r '.choices[0].message.content // empty')
if [ -z "$CONTENT" ]; then
  echo "❌ 响应内容为空"
else
  echo "✅ 响应内容: $CONTENT"
fi
```

#### 4.2 路由模式验证

```bash
echo "=== 路由模式验证 ==="

# 验证代码生成
CODE_CONTENT=$(cat /tmp/test_router_code.json | jq -r '.choices[0].message.content // empty')
if echo "$CODE_CONTENT" | grep -q "def\|function\|import\|class"; then
  echo "✅ 代码生成正确: 包含代码关键字"
else
  echo "❌ 代码生成可能不正确: $CODE_CONTENT"
fi

# 验证翻译
TRANSLATE_CONTENT=$(cat /tmp/test_router_translate.json | jq -r '.choices[0].message.content // empty')
if echo "$TRANSLATE_CONTENT" | grep -qE "[A-Za-z]"; then
  echo "✅ 翻译正确: 包含英文"
else
  echo "❌ 翻译可能不正确: $TRANSLATE_CONTENT"
fi

# 验证普通对话
CHAT_CONTENT=$(cat /tmp/test_router_chat.json | jq -r '.choices[0].message.content // empty')
if [ -n "$CHAT_CONTENT" ]; then
  echo "✅ 普通对话正确: 有响应内容"
else
  echo "❌ 普通对话可能不正确"
fi

# 验证摘要
SUMMARY_CONTENT=$(cat /tmp/test_router_summary.json | jq -r '.choices[0].message.content // empty')
if [ -n "$SUMMARY_CONTENT" ]; then
  echo "✅ 摘要生成正确: 有响应内容"
else
  echo "❌ 摘要生成可能不正确"
fi
```

#### 4.3 执行流程验证（最关键）

```bash
echo "=== 执行流程验证 ==="

# 从日志中提取执行信息
EXECUTION_LOG=$(grep "pipeline execution started.*router-mode" -A 200 logs/centag.log | \
  grep -E "executing node|node execution completed|node skipped|selected_route" | \
  head -30)

# 验证是否只有预期节点执行
EXPECTED_NODES="classifier code-generator chat-generator translate-gen summary-gen"
EXECUTED_NODES=$(echo "$EXECUTION_LOG" | grep "executing node" | \
  sed 's/.*{"node_id": "\([^"]*\)".*/\1/' | sort -u)

echo "执行的节点: $EXECUTED_NODES"

# 检查是否有意外节点执行
UNEXPECTED_NODES=""
for node in $EXECUTED_NODES; do
  if ! echo "$EXPECTED_NODES" | grep -q "$node"; then
    UNEXPECTED_NODES="$UNEXPECTED_NODES $node"
  fi
done

if [ -n "$UNEXPECTED_NODES" ]; then
  echo "❌ 发现意外执行的节点: $UNEXPECTED_NODES"
else
  echo "✅ 执行的节点符合预期"
fi
```

### Step 5: 生成详细报告

```bash
echo "=== 生成测试报告 ==="

cat > /tmp/pipeline_test_report.md << EOF
# Centag 流水线测试报告

## 测试概览
- 测试时间: $(date '+%Y-%m-%d %H:%M:%S')
- 测试流水线: $PIPELINE_ID
- 测试状态: $TEST_STATUS

## 测试结果

### 基础功能
- HTTP状态码: $HTTP_STATUS
- 响应内容: $CONTENT

### 执行流程验证
- 执行的节点: $EXECUTED_NODES
- 意外节点: $UNEXPECTED_NODES
- 路由决策: $ROUTE_DECISION

### 详细日志
\`\`\`
$EXECUTION_LOG
\`\`\`

## 问题诊断
$ISSUE_DIAGNOSIS

## 修复建议
$REPAIR_SUGGESTION
EOF

echo "报告已生成: /tmp/pipeline_test_report.md"
```

## 流水线类型专项测试

### 路由模式 (router-mode)

**测试重点**：
1. 验证`classifier`节点是否先执行
2. 验证路由决策是否正确（`selected_route`）
3. 验证是否只有对应的生成器节点执行
4. 验证其他节点是否被正确跳过

**测试用例**：
- 代码关键词："写一段Python代码"
- 翻译关键词："翻译成英文"
- 摘要关键词："总结一下"
- 普通对话："你好"

### 智能调度 (smart-scheduling)

**测试重点**：
1. 验证分支决策是否正确
2. 验证不同复杂度的输入是否走不同路径
3. 验证节点执行情况

**测试用例**：
- 简单问题："1+1=?"
- 复杂问题："详细解释量子计算"

### 降级模式 (fallback-mode)

**测试重点**：
1. 验证主后端正常时是否使用主后端
2. 验证主后端失败时是否正确降级
3. 验证降级输出是否正常

**测试用例**：
- 正常情况：使用主后端
- 模拟主后端失败：需要修改配置或使用不可用后端

### 聚合模式 (aggregator-mode)

**测试重点**：
1. 验证多个生成器是否并行执行
2. 验证聚合器是否正确合并结果
3. 验证最终输出质量

### 审核模式 (audit-mode)

**测试重点**：
1. 验证生成器是否先执行
2. 验证审核器是否正确评估
3. 验证审核结果和反馈

### 优化模式 (optimize-mode)

**测试重点**：
1. 验证生成器是否先执行
2. 验证优化器是否正确优化内容
3. 验证优化效果


## 自动跳过（依赖外部服务或未安装模板）

以下模式在依赖未满足或流水线未安装时跳过，不记为失败：

| 快捷码 | 依赖 / 条件 | 说明 |
|--------|-------------|------|
| `#t` | PostgreSQL | 透明代理 RawBody 贯通 |
| `#cm` | PostgreSQL | 缓存模式 |
| `#ch` | PostgreSQL + Redis | 缓存命中 |
| `#r` / `#o` / `#a` / `#m` / `#l` / `#sec` | 对应流水线已安装 | Centag 精简发行可能未附带这些模板 |

> 已移除（勿测）：`#mem0` / `#rag` / `#agent` / `#pi` / `#cs`（business 插件相关）。

## 快捷码（Centag 网关核心）

| 快捷码 | pipeline_id | 说明 | 测试重点 | 依赖 |
|--------|-------------|------|----------|------|
| `#s` | `smart-scheduling` | 智能调度（默认） | 分支决策、路径选择 | 无 |
| `#d` | `direct-backend` | 直连后端 | 单节点执行 | 无 |
| `#f` | `fallback-mode` | 降级模式 | 降级行为、容错能力 | 无 |
| `#tf` | `transparent-fast` | 超快透明代理 | 原样透传、未命中缓存 | 无 |
| `#ag` | `aggregator-mode` | 聚合模式 | 并行执行、结果聚合 | 无 |
| `#p` | `pipeline-mode` | 自定义流水线 | 多阶段执行 | 无 |
| `#r` | `router-mode` | 路由模式 | 执行顺序、节点选择 | 流水线已安装 |
| `#o` | `optimize-mode` | 优化模式 | 优化流程 | 流水线已安装 |
| `#a` | `audit-mode` | 审核模式 | 审核流程 | 流水线已安装 |
| `#m` | `model-matching` | 模型匹配 | 路由规则 | 流水线已安装 |
| `#l` | `translate-mode` | 翻译模式 | 翻译质量 | 流水线已安装 |
| `#sec` | `security-mode` | 安全审核 | 入站审核 | 流水线已安装 |
| `#t` | `transparent-proxy` | 透明代理 | RawBody 贯通 | PostgreSQL |
| `#cm` | `cache-mode` | 缓存模式 | 缓存策略 | PostgreSQL |
| `#ch` | `cache-hit` | 缓存命中 | 语义缓存 | PostgreSQL + Redis |

**推荐快速集**：`smart-scheduling,direct-backend,fallback-mode,transparent-fast`

**推荐标准集**：快速集 + `aggregator-mode,pipeline-mode`；若已安装再加 `router-mode`


## 常见问题诊断

### 问题1: 所有节点并行执行
**症状**：日志显示所有节点在同一时间开始执行
**原因**：生成器节点缺少`depends_on: ["classifier"]`
**修复**：为生成器节点添加依赖关系

### 问题2: 节点执行失败
**症状**：日志显示"node execution failed"
**原因**：后端不可用、API key无效、模型不存在
**修复**：检查后端配置和API key

### 问题3: 意外降级
**症状**：日志显示"bypassing error with fallback"
**原因**：主节点执行失败，系统使用fallback
**修复**：检查主节点配置，确保后端可用

### 问题4: 路由决策错误
**症状**：日志显示`selected_route`与预期不符
**原因**：路由规则配置错误、关键词匹配逻辑问题
**修复**：检查路由规则配置


## 常见修复

- 后端未启用: 修改 `config/initdata/pipeline-templates/*.yaml` 中的 `"backend"`
- 模式未注册: 检查 `internal/proxymode/manager.go` 的 `defaultModes`
- 常量不匹配: 确保 `internal/proxy/types.go` 与 `execution_mode.go` 一致
- 依赖关系缺失: 检查节点配置中的`depends_on`字段
- 路由配置错误: 检查节点配置中的`route_config`字段

# Claude Code 集成

通过 Centag 的 Anthropic Messages API 接入 Claude Code。

## 环境变量配置

将以下环境变量写入 Claude Code 的运行环境（shell profile 或项目的 `.env`）：

```bash
# Centag 服务地址
export ANTHROPIC_BASE_URL="http://localhost:20060/v1"

# 在 Centag「个人设置 / 用户管理」创建的 API Key
export ANTHROPIC_API_KEY="sk-your-centag-api-key-here"

# 如需代理（可选）
# export HTTP_PROXY="http://proxy:port"
# export HTTPS_PROXY="http://proxy:port"
```

## 使用方式

设置环境变量后正常启动 Claude Code：

```bash
# 交互模式
claude

# 非交互模式
claude -p "解释这段代码"
```

## 模型选择

Centag 支持自动路由到任何已配置的 Anthropic 协议后端。使用 Claude 原生模型名：

```bash
claude --model claude-sonnet-4-20250514 -p "..."
```

模型名可与 Centag 管理后台配置的模型别名匹配。

## Thinking / 推理展示

Centag 的 ThinkSplit 功能自动提取并分流 `thinking` 内容，Claude Code 可正常展示思考过程。

## 故障排查

1. **连接失败**：确认 `ANTHROPIC_BASE_URL` 指向正确的 Centag 地址和端口（默认 `http://localhost:20060/v1`）
2. **认证失败（401）**：确认 API Key 在 Centag 管理后台已创建且未过期
3. **模型不存在**：确认目标模型已在 Centag 后端配置中启用
4. **超时**：检查网络连通性和 Centag 日志

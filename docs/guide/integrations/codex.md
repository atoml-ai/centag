# OpenAI Codex CLI 集成

Codex 可通过 OpenAI 兼容 API（`/v1/chat/completions`）或 OpenAI Responses API（`/v1/responses`）接入 Centag。

## 方式一：Chat Completions API

### 配置文件 `~/.codex/config.toml`

```toml
[model]
# 使用 OpenAI Chat Completions 兼容模式
driver = "openai"
provider = "openai"

[models.openai]
base_url = "http://localhost:20060/v1"
api_key = "sk-your-virtual-key-here"
chat_model = "gpt-4o"
```

### 使用

```bash
codex exec "解释这段代码"
codex chat
```

## 方式二：Responses API（wire_api = "responses"）

Codex 可选择使用 OpenAI Responses 协议获得更丰富的响应格式（包括 reasoning、tool use、web search 等）。

### 配置文件 `~/.codex/config.toml`

```toml
[model]
driver = "openai"

[models.openai]
base_url = "http://localhost:20060/v1"
api_key = "sk-your-virtual-key-here"
# 指向 Centag 的 /v1/responses 端点
chat_model = "gpt-4o"
wire_api = "responses"
```

### 端点映射

Codex 的 `wire_api = "responses"` 配置会自动将请求路由到 Centag 的 `POST /v1/responses` 端点。Centag 内部将 Responses 协议转换为后端实际协议并返回统一格式。

## 模型选择

- `chat_model` 字段指定目标模型，与 Centag 管理后台配置的模型别名匹配
- 支持路由到任何后端类型（OpenAI、Anthropic、Gemini、Azure 等）

## 推理内容展示

Centag 的 ThinkSplit 自动处理推理内容分流，Codex 可正常展示模型的思考/推理过程。

## 故障排查

1. **连接失败**：确认 `base_url` 格式正确，末尾 `/v1` 不缺失
2. **401 错误**：检查 Centag 后台 Virtual Key 是否有效
3. **模型不存在**：确认 `chat_model` 名称与 Centag 后端配置中的模型别名一致
4. **Responses API 404**：确认 Centag 编译时启用了 `protocol_openairesponses` tag（默认包含）

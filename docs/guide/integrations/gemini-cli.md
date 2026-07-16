# Gemini CLI 集成

Gemini CLI 可通过两种方式接入 Centag：**OpenAI 兼容模式** 和 **Gemini 原生协议**。

## 方式一：OpenAI 兼容模式（推荐）

适用于 Gemini CLI 版本 `>= 0.9.0`。

### 环境变量

```bash
# Centag 服务地址
export GEMINI_API_KEY="sk-your-virtual-key-here"
export GEMINI_API_BASE="http://localhost:20060/v1"
```

### 使用

```bash
gemini
gemini -p "解释这段代码"
```

## 方式二：Gemini 原生协议

Centag 提供了 `/v1beta/models/*action` 原生 Gemini 入口，可直接代理 GenerateContent 等 Gemini API。

### 配置

在 Gemini CLI 中将 API Base 指向 Centag 的 Gemini 入口：

```bash
export GEMINI_API_KEY="sk-your-virtual-key-here"
export GOOGLE_GENERATIVE_AI_API_BASE="http://localhost:20060/v1beta"
```

### 注意

走 Gemini 原生协议时，请求/响应格式与 Google AI API 完全一致，不走 OpenAI 兼容层。此路径适用于需要精确控制 Gemini API 参数（如 `generationConfig`、`safetySettings`）的场景。

## 模型选择

支持任何 Centag 后端配置中的 Gemini 模型：

```bash
gemini --model gemini-2.5-flash -p "..."
```

模型名与 Centag 管理后台配置的模型别名匹配；也可路由到非 Gemini 后端（通过代理模式 `#d` 或流水线重定向）。

## 推理内容展示

Centag 的 ThinkSplit 自动处理 Gemini 的 `thought`/`reasoning` 内容分流，确保 Gemini CLI 正常展示思考过程。

## 故障排查

1. **404 路由错误**：确认 `GEMINI_API_BASE` 末尾无多余斜杠，建议值为 `http://localhost:20060/v1`
2. **401 认证失败**：检查 Centag 后台 API Key 状态
3. **原生协议 405**：确认使用了 `/v1beta` 路径（而非 `/v1`）
4. **模型不可用**：检查 Centag 后端配置中已添加并启用 Gemini 后端

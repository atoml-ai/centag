# Pi coding agent 集成

通过 Centag 的 OpenAI 兼容 API（`/v1/chat/completions`）接入 [Pi](https://github.com/earendil-works/pi)。

## 安装

```bash
curl -fsSL https://pi.dev/install.sh | sh
# 或
npm i -g --ignore-scripts @earendil-works/pi-coding-agent
```

## 方式一：Web「Agent 配置」一键接入

在 Centag 管理台打开 **Agent 配置**，选择 **Pi**，按向导选择流水线并写入配置。会合并：

- `~/.pi/agent/models.json`：`providers.centag`
- `~/.pi/agent/settings.json`：`defaultProvider` / `defaultModel`

## 方式二：手动配置

### `~/.pi/agent/models.json`

```json
{
  "providers": {
    "centag": {
      "baseUrl": "http://localhost:20060/v1",
      "apiKey": "sk-your-centag-api-key-here",
      "api": "openai-completions",
      "models": [
        {
          "id": "gpt-4o",
          "name": "gpt-4o",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 16384
        }
      ]
    }
  }
}
```

流水线路由模型可使用 `centag/<pipeline-id>` 作为 `id`。

### `~/.pi/agent/settings.json`

```json
{
  "defaultProvider": "centag",
  "defaultModel": "gpt-4o"
}
```

## 使用

```bash
# 交互模式（使用 settings 默认模型）
pi

# 指定 Centag 模型
pi --model centag/gpt-4o "解释这段代码"

# 非交互（print）
pi -p --model centag/gpt-4o "Hello, can you hear me?"
```

也可配合 wrap：

```bash
centag wrap run -- pi
```

## 故障排查

1. **模型列表没有 centag**：确认 `models.json` JSON 合法，并重新打开 `/model`（Pi 会热加载该文件）
2. **401**：检查 Centag API Key 是否有效
3. **连接失败**：确认 `baseUrl` 指向 `http://<centag-host>:<port>/v1`
4. **模型不存在**：确认 `defaultModel` / `--model` 与 Centag 后端或流水线别名一致

# Ollama 后端配置指南

## 配置说明

本项目支持使用本地运行的 Ollama 服务作为后端。以下是详细的配置步骤：

## 1. 启动 Ollama 服务

确保你已经在本地安装并启动了 Ollama 服务：

```bash
# 启动 Ollama 服务
ollama serve

# 拉取 qwen3:0.6b 模型（如果还没有）
ollama pull qwen3:0.6b
```

默认情况下，Ollama 会在 `http://localhost:21434` 上监听。

## 2. 后端配置参数

### 基本配置项：

- **名称**: 自定义的后端名称，如"本地Ollama"
- **类型**: 选择 "Ollama"
- **服务地址**: `http://localhost:21434/api`
- **API Key**: 留空（Ollama 本地服务通常不需要 API Key）
- **权重**: 1（负载均衡权重）
- **超时**: 120 秒（建议设置较长超时时间，因为本地推理可能较慢）

### 配置示例：

```json
{
  "id": "ollama-local",
  "name": "本地Ollama",
  "type": "ollama",
  "base_url": "http://localhost:21434/api",
  "api_key": "",
  "enabled": true,
  "weight": 1,
  "timeout": 120,
  "max_retries": 3,
  "metadata": {
    "local": "true"
  },
  "description": "本地运行的Ollama服务"
}
```

## 3. 在 Web UI 中添加配置

1. 打开项目 Web 界面
2. 点击左侧菜单的"⚙️ 配置管理"
3. 在后端配置管理中点击"➕ 添加后端"
4. 填写以下信息：
   - 名称：本地Ollama
   - 类型：Ollama
   - 服务地址：http://localhost:21434/api
   - API Key：（留空）
   - 权重：1
   - 超时：120
5. 点击"🔍 测试连接"验证配置
6. 点击"💾 保存"

## 4. 设置为默认后端

添加配置后，你可以通过以下方式将其设为默认后端：

1. 在后端列表中找到刚添加的 Ollama 配置
2. 确保其状态为"启用"
3. 点击"设为默认"单选按钮
4. 系统会自动将此配置作为默认后端使用

## 5. 使用模型

配置完成后，你可以在 AI 对话功能中使用 Ollama 提供的模型，包括你已经下载的 `qwen3:0.6b` 模型。

## 6. 故障排除

### 常见问题：

1. **连接失败**：
   - 检查 Ollama 服务是否正在运行
   - 确认端口 21434 是否被占用
   - 验证防火墙设置

2. **模型不可用**：
   - 确认模型已正确下载：`ollama list`
   - 检查模型名称是否正确

3. **响应超时**：
   - 适当增加超时时间设置
   - 本地推理速度取决于硬件性能

### 测试命令：

```bash
# 测试 Ollama 服务是否正常
curl http://localhost:21434/api/tags

# 测试具体模型
curl http://localhost:21434/api/generate -d '{
  "model": "qwen3:0.6b",
  "prompt": "Hello, how are you?",
  "stream": false
}'
```

## 7. 注意事项

- Ollama 本地运行需要足够的计算资源（CPU/RAM/GPU）
- 推理速度取决于你的硬件配置
- 建议根据实际需求调整超时时间
- 可以同时配置多个后端，在不同场景下切换使用
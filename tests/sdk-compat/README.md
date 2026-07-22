# SDK 兼容性测试

验证第三方 SDK 通过 Centag 代理访问大模型服务的兼容性。

## 测试目标

- openai-python SDK 兼容性
- anthropic-sdk-python 兼容性
- 请求/响应格式完整性
- 流式响应兼容性

## 前置条件

1. Centag 服务已启动（默认 `http://localhost:20060`）
2. 已配置有效的后端 API Key
3. Python 3.8+ 已安装

## 安装依赖

```bash
pip install -r requirements.txt
```

## 运行测试

```bash
# 运行所有测试
python -m pytest -v

# 运行 OpenAI SDK 测试
python -m pytest test_openai_sdk.py -v

# 运行 Anthropic SDK 测试
python -m pytest test_anthropic_sdk.py -v
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CENTAG_BASE_URL` | `http://localhost:20060` | Centag 服务地址 |
| `CENTAG_API_KEY` | `test-key` | API Key |
| `OPENAI_MODEL` | `gpt-4` | OpenAI 模型名称 |
| `ANTHROPIC_MODEL` | `claude-3-opus-20240229` | Anthropic 模型名称 |

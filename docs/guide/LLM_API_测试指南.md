# LLM API 测试指南

## 📋 概述

本指南介绍如何测试通过系统代理访问大模型 API 的功能。

## 🚀 快速开始

### 方式 1: 完整测试（包含 LLM API）

```bash
./test/test-system-proxy-complete.sh --auto
```

这会运行所有测试，包括：
- 基础连通性测试（1-7项）
- **LLM API 专项测试（8-10项）** ⭐

### 方式 2: 仅测试 LLM API

```bash
./test/test-llm-api-only.sh
```

这个脚本专门测试大模型 API 功能，不会设置系统代理。

## 🧪 LLM API 测试内容

### 测试 8: 大模型 API 访问

**测试内容：**
- ✅ 检查配置的 LLM API 域名
- ✅ 测试每个域名的 HTTPS 连通性
- ✅ 验证 OpenAI API 端点响应
- ✅ 检查代理日志中的请求记录

**测试的域名：**
- `api.openai.com`
- `api.anthropic.com`
- `api.deepseek.com`
- `api.mistral.ai`
- `api.groq.com`
- 等等（从配置文件读取）

**示例输出：**
```
>>> 测试 8: 大模型 API 访问
----------------------------------------
ℹ 测试 LLM API 域名的 HTTPS 连通性...
ℹ 测试 api.openai.com...
✓   ✓ api.openai.com 可通过代理访问 (HTTP 200)
ℹ 测试 api.anthropic.com...
✓   ✓ api.anthropic.com 可通过代理访问 (HTTP 200)

ℹ LLM API 域名测试结果: 5/5 成功

ℹ 测试 OpenAI API 端点...
✓   ✓ OpenAI API 端点响应正常
ℹ     提示: 返回了错误或数据，说明代理和路由工作正常
```

### 测试 9: 大模型 API 功能测试

**测试内容：**
- ✅ 检查后端配置的模型列表
- ✅ 通过代理访问模型列表 API
- ✅ 验证 API 响应格式

**示例输出：**
```
>>> 测试 9: 大模型 API 功能测试
----------------------------------------
ℹ 检查后端配置的模型...
✓ 后端配置了 15 个模型
ℹ   - gpt-4
ℹ   - gpt-3.5-turbo
ℹ   - claude-3-opus

ℹ 测试通过代理访问模型列表 API...
✓ ✓ 通过代理成功访问模型列表
ℹ   返回了 15 个模型
```

### 测试 10: 大模型缓存功能

**测试内容：**
- ✅ 获取缓存统计信息
- ✅ 显示缓存命中率
- ✅ 验证缓存功能正常

**示例输出：**
```
>>> 测试 10: 大模型缓存功能
----------------------------------------
ℹ 检查缓存统计...
✓ 缓存统计信息:
  总缓存条目: 42
  缓存命中: 128
  缓存未命中: 35
  命中率: 78.5%

ℹ 提示: 实际的 LLM API 请求会被缓存，提高响应速度并节省成本
ℹ       重复的请求会直接从缓存返回，不会调用真实的 LLM API
```

## 🔧 使用实际 API Key 测试

### 设置 API Key

```bash
# 设置 OpenAI API Key
export OPENAI_API_KEY="<YOUR_API_KEY_HERE>your-api-key-here"

# 运行 LLM API 测试
./test-llm-api-only.sh
```

### 测试实际调用

当设置了 `OPENAI_API_KEY` 环境变量后，脚本会进行实际的 API 调用测试：

```bash
>>> 6. 实际 API 调用测试
----------------------------------------
ℹ 检测到 OPENAI_API_KEY，进行实际 API 调用测试...
ℹ 测试 chat completions...
✓ ✓ API 调用成功
ℹ   响应: Hello! How can I assist you today?
```

**测试的功能：**
- ✅ 通过代理发送真实的 chat completion 请求
- ✅ 验证响应正确性
- ✅ 确认缓存功能（第二次调用应该更快）

## 📊 测试场景

### 场景 1: 首次部署验证

```bash
# 1. 启动服务
./start.sh run

# 2. 运行完整测试
./test-system-proxy-complete.sh --auto

# 3. 查看测试报告
# 应该看到所有 10 项测试都通过
```

### 场景 2: 验证 LLM API 路由

```bash
# 1. 设置系统代理
./test-system-proxy-complete.sh --set

# 2. 测试 LLM API（不需要 API Key）
./test-llm-api-only.sh

# 3. 查看代理日志
tail -f /tmp/centag-test.log | grep "api.openai.com"
```

### 场景 3: 测试缓存功能

```bash
# 需要真实的 API Key
export OPENAI_API_KEY="<YOUR_API_KEY_HERE>your-key"

# 1. 第一次调用（应该调用真实 API）
curl -k -x http://127.0.0.1:8081 https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# 2. 查看缓存统计
curl http://localhost:20060/api/v1/cache/stats

# 3. 第二次相同调用（应该从缓存返回，更快）
# 重复第一步的命令，注意响应时间差异
```

### 场景 4: 多模型测试

```bash
# 设置系统代理
./test-system-proxy-complete.sh --set

# 测试不同的模型提供商
export OPENAI_API_KEY="<YOUR_API_KEY_HERE>your-openai-key"
export ANTHROPIC_API_KEY="<YOUR_API_KEY_HERE>ant-your-anthropic-key"

# 测试 OpenAI
curl -k -x http://127.0.0.1:8081 https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# 测试 Anthropic
curl -k -x http://127.0.0.1:8081 https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus-20240229",
    "max_tokens": 10,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 🔍 验证代理路由

### 检查请求是否通过代理

```bash
# 方法 1: 查看日志
tail -f /tmp/centag-test.log | grep "MITM"

# 方法 2: 查看后端统计
curl http://localhost:20060/api/v1/monitor/stats

# 方法 3: 检查缓存条目
curl http://localhost:20060/api/v1/cache/list
```

### 验证路由到后端

**预期行为：**
1. 访问 `api.openai.com/v1/chat/completions` → 转发到后端 `localhost:20060`
2. 访问 `www.google.com` → 直接转发到原始服务器

**验证方法：**
```bash
# 1. 清空日志
> /tmp/centag-test.log

# 2. 访问 LLM API
curl -k -x http://127.0.0.1:8081 https://api.openai.com/v1/models

# 3. 查看日志，应该看到：
#    - MITM CONNECT request: api.openai.com
#    - Forwarding to Centag backend
tail -20 /tmp/centag-test.log
```

## 📈 性能测试

### 测试代理性能

```bash
# 测试响应时间（不使用代理）
time curl -s https://api.openai.com/v1/models

# 测试响应时间（使用代理）
time curl -k -s -x http://127.0.0.1:8081 https://api.openai.com/v1/models
```

### 测试缓存性能

```bash
# 第一次请求（未缓存）
time curl -k -s -x http://127.0.0.1:8081 \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/chat/completions \
  -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hi"}]}'

# 第二次请求（已缓存，应该更快）
time curl -k -s -x http://127.0.0.1:8081 \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/chat/completions \
  -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hi"}]}'
```

## 🐛 常见问题

### 问题 1: API 返回 401/403

**原因：** 测试使用了无效的 API Key

**解决：**
```bash
# 设置真实的 API Key
export OPENAI_API_KEY="<YOUR_API_KEY_HERE>your-real-key"

# 或者这是正常的，说明代理工作正常
# 测试脚本会正确识别错误响应
```

### 问题 2: 请求未通过代理

**检查：**
```bash
# 1. 确认系统代理已设置
networksetup -getwebproxy "Wi-Fi"  # macOS
echo $https_proxy                   # Linux

# 2. 检查代理日志
grep "CONNECT.*api.openai.com" /tmp/centag-test.log

# 3. 测试代理连接
curl -v -x http://127.0.0.1:8081 https://api.openai.com
```

### 问题 3: 缓存不工作

**检查：**
```bash
# 1. 查看缓存配置
curl http://localhost:20060/api/v1/cache/stats

# 2. 检查缓存是否启用（配置已归档）
# cat ../../archive/deprecated/configs/config.yaml | grep -A 5 "cache:"
curl http://localhost:20060/api/v1/cache/stats | grep -i enabled

# 3. 清空缓存重新测试
curl -X POST http://localhost:20060/api/v1/cache/clear
```

## 📚 相关命令速查

```bash
# 测试命令
./test-system-proxy-complete.sh --auto      # 完整测试
./test-llm-api-only.sh                      # 仅LLM API测试

# 查看日志
tail -f /tmp/centag-test.log             # 实时日志
grep "api.openai.com" /tmp/centag-test.log  # 过滤日志

# 缓存管理
curl http://localhost:20060/api/v1/cache/stats  # 查看统计
curl -X POST http://localhost:20060/api/v1/cache/clear  # 清空缓存

# 监控
curl http://localhost:20060/api/v1/monitor/stats  # 查看监控
curl http://localhost:20060/api/v1/monitor/dashboard  # 查看仪表板

# 模型列表
curl http://localhost:20060/v1/models            # 后端模型
curl -k -x http://127.0.0.1:8081 https://api.openai.com/v1/models  # 通过代理
```

## 💡 最佳实践

1. **首次测试**
   - 先运行基础测试，确保代理工作
   - 再运行 LLM API 测试
   - 最后使用真实 API Key 测试

2. **日常使用**
   - 定期查看缓存统计
   - 监控命中率，优化缓存策略
   - 查看日志排查问题

3. **性能优化**
   - 调整缓存过期时间
   - 配置语义相似度阈值
   - 使用向量数据库提升匹配速度

## 🔗 相关文档

- [系统代理测试指南](系统代理测试指南.md)
- [系统代理速查表](../SYSTEM_PROXY_CHEATSHEET.md)
- [MITM 代理修复总结](../MITM_PROXY_FIX_SUMMARY.md)

# 中间件认证配置自动化指南

> **2026-05**：中间件由 **`deploy/stack`** 编排；本仓库 **`./start.sh docker up`** 只启动 Centag。运行时配置**仅**使用 **`config/secrets/.env`**（compose `env_file`）；**不再生成或同步 `docker/.env`**。`scripts/ops/generate-secrets.sh` 只生成主服务与 **PG 元库**相关密钥；Redis/ES/Ollama 等栈变量请在 stack 或自行追加到 `config/secrets/.env`。

## 概述

1. **`./start.sh docker up`**：若缺少 **`config/secrets/.env`** 会调用 **`generate-secrets`** 生成最小可用配置。
2. **`generate-secrets`**：写入 **`config/secrets/.env`**（管理员、`PG_*`、`LLM_PROXY_*` 日志与默认路由等），**不**再包含 Elasticsearch/Redis/Chroma/Ollama 等栈侧整段模板。
3. 与 **stack** 对齐的密码：请在 **`config/secrets/.env`** 中手动追加或从 stack 同步所需变量。

## 快速开始

### 方式一: 推荐（stack + 本仓库）

```bash
# 1) 在 deploy/stack 启动所需中间件（命令以该仓库为准）
cd deploy/stack && ./start.sh help

# 2) 在本仓库生成/同步凭据并启动 Centag
cd ../centag
./start.sh docker up
```

**说明**：`docker up` 仅在缺失时生成 **`config/secrets/.env`**；中间件在 **stack** 运行，请保证 **`config/secrets/.env`** 中连接信息与 stack 一致。

### 方式二: 手动控制认证配置

如果你想更细粒度地控制认证配置:

```bash
# 1. 手动生成认证配置
./start.sh init-secrets                      # 交互式生成
./start.sh generate-secrets                  # 使用相同密码
./start.sh generate-secrets --unique-passwords  # 使用不同密码

# 2. 在 stack 启动中间件后，启动 Centag
./start.sh docker up
```

## 认证配置文件

### 文件位置

- **主配置**: `config/secrets/.env`（与 `start.sh load_env` 一致；若不存在可回退读取遗留的 `config/secrets/.env.middleware`）
- **生成**: `./start.sh generate-secrets` 覆盖写入 **`config/secrets/.env`**

### `generate-secrets` 生成的字段（摘要）

- Web 管理员用户名/密码、`LLM_PROXY_ADMIN_API_KEY`、`LLM_PROXY_API_KEY_STORAGE_SECRET`
- **`PG_*`**（元数据库）
- 基础 **`LLM_PROXY_*`**（端口、默认路由、日志）

栈侧 Redis/ES/Ollama 等请自行追加到 **`config/secrets/.env`** 或在 **deploy/stack** 管理。

### 密码策略

```bash
./start.sh generate-secrets --same-password      # 管理员与 PG 同口令（默认）
./start.sh generate-secrets --unique-passwords   # 管理员与 PG 独立口令
```

## 自动化流程

### 完整流程

```
用户执行: ./start.sh docker up
    ↓
检查: config/secrets/.env 是否存在?
    ↓ 否
调用 generate-secrets 生成 config/secrets/.env
    ↓
load_env
    ↓
docker compose up（env_file=config/secrets/.env）
```

### 关键特性

✅ **自动检测**: 缺少 `config/secrets/.env` 时生成最小配置  
✅ **单文件真源**: 不再维护 `docker/.env`

## 使用示例

### 示例 1: 启动 Ollama（stack）

```bash
cd deploy/stack && ./start.sh start ollama
docker ps | grep -i ollama
```

### 示例 2: 多个中间件与 Centag

在 **deploy/stack** 按需启动 Redis、Elasticsearch 等后，用 **`config/secrets/.env`** 中的密码验证连通性（端口与容器名以 stack 为准）：

```bash
# 示例：Redis（密码来自 config/secrets/.env）
docker exec <redis 容器名> redis-cli -a "<REDIS_PASSWORD>" ping
# 示例：ES
curl -u "elastic:<ELASTICSEARCH_PASSWORD>" "http://<ES 主机>:9200/_cluster/health"
```

### 示例 3: 查看和修改认证配置

```bash
# 查看生成的认证信息
cat config/secrets/.env

# 查看正在使用的认证信息
grep -E "(PASSWORD|API_KEY|TOKEN)" config/secrets/.env

# 重新生成认证信息
./start.sh generate-secrets --unique-passwords

# 重启 Centag 使新配置生效
./start.sh docker down
./start.sh docker up
```

## 故障排查

### 问题 1: 认证失败

**症状**: 中间件启动时认证失败

**解决方案**:
```bash
# 检查认证信息是否正确生成
cat config/secrets/.env

# 检查 config/secrets/.env
grep PASSWORD config/secrets/.env

# 重新生成认证配置
rm -f config/secrets/.env
./start.sh docker up
```

### 问题 2: 环境变量未生效

**症状**: `config/secrets/.env` 中关键变量为空或未加载

**解决方案**:
```bash
# 手动加载环境变量
export $(cat config/secrets/.env | grep -v '^#' | xargs)

# 重新启动服务
./start.sh docker down
./start.sh docker up
```

### 问题 3: 密码过期或需要更改

**症状**: 需要定期更换密码

**解决方案**:
```bash
# 重新生成新的认证配置
./start.sh generate-secrets --same-password

# 重启 Centag
./start.sh docker down
./start.sh docker up
```

## 安全最佳实践

### 1. 密码管理

- ✅ 定期更换密码(建议每 90 天)
- ✅ 生产环境使用独立密码模式
- ✅ 将认证信息存储在安全的密钥管理系统中
- ❌ 不要将认证信息提交到版本控制
- ❌ 不要在代码中硬编码密码

### 2. 文件权限

```bash
# 确保文件权限正确
chmod 600 config/secrets/.env
chmod 700 config/secrets/

# 验证权限
ls -la config/secrets/.env
# 应该显示: -rw-------
```

### 3. 环境隔离

```bash
# 开发环境
./start.sh generate-secrets --same-password

# 测试环境
./start.sh generate-secrets --unique-passwords

# 生产环境
# 使用专业的密钥管理系统(Vault, AWS Secrets Manager 等)
```

## 高级用法

### 自定义认证配置

如果你需要使用自己的认证信息:

```bash
# 1. 编辑认证配置文件
vim config/secrets/.env

# 2. 重新生成或合并变量后保存 config/secrets/.env
./start.sh generate-secrets

# 3. 启动 Centag（中间件在 deploy/stack）
./start.sh docker up
```

### 导出认证信息

```bash
# 导出所有认证信息到环境变量
export $(cat config/secrets/.env | grep -v '^#' | xargs)

# 查看已导出的环境变量
env | grep -E "(ELASTICSEARCH|REDIS|CHROMADB|OLLAMA)"
```

### 集成 CI/CD

```yaml
# .github/workflows/deploy.yml
- name: Generate Secrets
  run: |
    ./start.sh generate-secrets --unique-passwords

- name: Start Centag
  run: |
    ./start.sh docker up
```

## 常见问题

### Q: 为什么所有中间件使用相同的密码?

A: 这是默认配置,便于管理。生产环境建议使用 `--unique-passwords` 选项为每个中间件生成独立密码。

### Q: 生成的密码安全吗?

A: 是的,密码使用 openssl 或 /dev/urandom 生成,包含大小写字母、数字和特殊字符,长度为 32 位,符合安全标准。

### Q: 如何查看已生成的密码?

A: 查看配置文件:
```bash
cat config/secrets/.env
```

### Q: 可以手动修改密码吗?

A: 可以,但建议使用 `./start.sh generate-secrets` 重新生成,以确保密码强度。

### Q: 认证配置会被提交到 Git 吗?

A: 不会,`config/secrets/.env` 已添加到 `.gitignore`。

## 相关命令速查

| 命令 | 说明 |
|------|------|
| `./start.sh init-secrets` | 交互式初始化认证配置 |
| `./start.sh generate-secrets` | 生成认证配置(相同密码) |
| `./start.sh generate-secrets --unique-passwords` | 生成认证配置(独立密码) |
| `source scripts/load-secrets.sh` | 加载认证配置到当前 shell |
| `./start.sh docker up` | 启动 Centag（缺 secrets 时自动生成） |
| `cat config/secrets/.env` | 查看认证配置 |

## 获取帮助

```bash
# 查看帮助信息
./start.sh help

# 查看 secrets 加载帮助
scripts/load-secrets.sh --help

# 查看 secrets 目录说明
cat config/secrets/README.md
```

## 总结

现在的认证配置系统完全自动化,你只需要:

1. **开发环境**: `./start.sh docker up`
2. **生产环境**: 先使用独立密码生成配置,然后启动服务
3. **定期维护**: 定期重新生成密码,重启服务

无需任何手动配置,一切都自动完成! 🎉

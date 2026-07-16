# Ollama 模型拉取故障排查

> **2026-05**：Ollama 容器名、compose 路径以 **`deploy/stack`** 为准；主仓库不再通过 `./start.sh docker up ollama` 编排 Ollama。下文中的 `centag-ollama` 等仅为示例。

## 常见问题

### 1. 模型拉取失败

#### 错误信息
```
[ERROR] 模型 'bge-m3' 拉取失败，已达到最大重试次数 (3)
```

#### 可能原因

##### 网络连接问题
Ollama 需要从 `ollama.com` 下载模型，如果网络无法连接，拉取会失败。

**诊断方法**：
```bash
# 检查网络连接
curl -v https://ollama.com

# 在容器内检查
docker exec -it centag-ollama curl -v https://ollama.com
```

**解决方案**：
- 确保网络可以访问外网
- 检查防火墙设置
- 配置代理（如果需要）

##### 磁盘空间不足
模型文件较大，需要足够的磁盘空间。

**诊断方法**：
```bash
# 检查磁盘空间
df -h

# 在容器内检查
docker exec -it centag-ollama df -h
```

**解决方案**：
- 清理不必要的模型：`docker exec -it centag-ollama ollama rm <model>`
- 清理 Docker 系统资源：`docker system prune`
- 扩展磁盘空间

##### 模型名称错误
确保模型名称正确，区分大小写。

**诊断方法**：
```bash
# 在容器内检查可用模型
docker exec -it centag-ollama ollama list
```

**支持的标准模型**：
- `bge-m3` - 多语言高质量，~2.2GB
- `nomic-embed-text` - 英文轻量级，~274MB
- `all-minilm` - 英文极轻量级，~120MB
- `llama2` - LLM 模型

### 2. docker-compose 警告

#### 警告信息
```
WARN[0000] /home/caijun/workspaces/centag/docker/docker-compose.override.yaml: attribute `version` is obsolete
```

#### 解决方案
Compose v2 会提示根级 `version` 已废弃：若你仍使用自定义 **`docker-compose.override.yaml`**，删除其中的 `version` 键即可。仓库内的 **`docker-compose.gpu-override.yaml` 已移除**；GPU 相关片段请在 **deploy/stack** 的 Ollama compose 或你自建的 override 中维护。

#### 警告信息
```
WARN[0000] No services to build
```

#### 说明
这是正常提示，表示不需要构建镜像，直接使用现有镜像启动服务。

### 3. 容器启动慢

#### 原因
首次启动时需要下载模型，可能需要几分钟。

#### 解决方案
- 使用较小的模型（如 `all-minilm`）
- 延长超时时间：`DOWNLOAD_TIMEOUT=600`
- 预先下载好模型，避免启动时下载

### 4. 模型拉取超时

#### 错误信息
```
[ERROR] 拉取超时 (超过 300 秒)
```

#### 解决方案
```bash
# 编辑 .env 文件，增加超时时间
DOWNLOAD_TIMEOUT=600  # 增加到 10 分钟

# 在 deploy/stack 中重启 Ollama 服务（以该仓库文档为准），例如：
# cd deploy/stack && ./start.sh help
```

## 诊断工具

### 1. 手动拉取测试（本机或 stack 内 Ollama）

```bash
# 本机已安装 ollama CLI 时
ollama pull all-minilm
ollama pull all-minilm --verbose
```

若在 **deploy/stack** 中运行 Ollama，请用该仓库文档中的容器名执行 `docker exec …`。

### 2. 查看日志

```bash
# 容器名以 stack / 实际部署为准，例如：
docker logs <ollama 容器名>

# 实时查看
docker logs -f <ollama 容器名>

# 查看最近 100 行
docker logs --tail 100 <ollama 容器名>
```

### 3. 检查模型状态
```bash
# 列出所有模型（容器名以实际为准）
docker exec -it <ollama 容器名> ollama list

# 查看模型信息
docker exec -it <ollama 容器名> ollama show all-minilm

# 测试模型
docker exec -it <ollama 容器名> ollama run all-minilm "Hello"
```

## 推荐配置

### 开发环境（快速启动）
```bash
# config/secrets/.env
OLLAMA_DEFAULT_MODEL=all-minilm
OLLAMA_AUTOLOAD_MODEL=true
RETRY_COUNT=3
RETRY_DELAY=5
DOWNLOAD_TIMEOUT=180
```

**特点**：
- 模型小，启动快
- 适合快速测试

### 生产环境（高质量）
```bash
# config/secrets/.env
OLLAMA_DEFAULT_MODEL=bge-m3
OLLAMA_AUTOLOAD_MODEL=true
RETRY_COUNT=5
RETRY_DELAY=10
DOWNLOAD_TIMEOUT=600
```

**特点**：
- 模型质量高
- 支持多语言
- 需要更长的启动时间

### 网络环境较差
```bash
# config/secrets/.env
OLLAMA_DEFAULT_MODEL=all-minilm
OLLAMA_AUTOLOAD_MODEL=false  # 手动拉取
RETRY_COUNT=10
RETRY_DELAY=20
DOWNLOAD_TIMEOUT=900
```

**特点**：
- 手动拉取避免启动失败
- 更多重试机会
- 更长的超时时间

## 常用命令

### 模型管理
```bash
# 拉取模型
docker exec -it centag-ollama ollama pull <model>

# 列出模型
docker exec -it centag-ollama ollama list

# 删除模型
docker exec -it centag-ollama ollama rm <model>

# 查看模型信息
docker exec -it centag-ollama ollama show <model>
```

### 服务管理
```bash
# 重启服务
docker-compose restart ollama

# 停止服务
docker-compose stop ollama

# 启动服务
docker-compose start ollama

# 查看状态
docker-compose ps ollama
```

### 日志查看
```bash
# 查看完整日志
docker logs centag-ollama

# 实时跟踪
docker logs -f centag-ollama

# 查看最近 N 行
docker logs --tail 50 centag-ollama

# 查看特定时间的日志
docker logs --since 5m centag-ollama
```

## 相关资源

- [Ollama 官方文档](https://ollama.ai/docs)
- [支持的模型库](https://ollama.ai/library)
- [Ollama GitHub](https://github.com/ollama/ollama)

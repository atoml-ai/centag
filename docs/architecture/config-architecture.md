# 统一配置 API 文档

## 概述

本文档描述了统一配置管理的所有 API 接口，确保 Web 配置页面能够正确读写 `config.yaml` 文件。

## 配置架构

### 统一配置文件

所有配置现在都保存在数据库中（旧版 `config.yaml` 已归档至 `../../archive/deprecated/configs/config.yaml`）：

```yaml
server: ...
log: ...
proxy: ...
cache: ...
redis: ...
vector: ...
embedding: ...
qa_split: ...
plugins: ...
system_proxy: ...
backends: [...]      # 后端配置
storages: [...]      # 存储配置
default_storage: ... # 默认存储
```

### 配置流程

1. **加载配置**: `config.yaml` → 内存配置
2. **修改配置**: Web API → 内存配置
3. **保存配置**: 内存配置 → `config.yaml`
4. **回填配置**: `config.yaml` → Web 页面

---

## API 接口

### 1. 获取所有配置 (统一接口)

**接口**: `GET /api/v1/config`

**功能**: 获取完整的配置信息，包括所有模块

**响应示例**:
```json
{
  "success": true,
  "data": {
    "server": {
      "port": 20060,
      "host": "0.0.0.0",
      "mode": "debug"
    },
    "log": {
      "level": "info",
      "format": "json",
      "output": "file",
      "file": {
        "path": "./logs",
        "filename": "llm-proxy.log",
        "max_size": 100,
        "max_backups": 3,
        "max_age": 7,
        "compress": true
      }
    },
    "cache": {
      "enabled": true,
      "default_ttl": 3600,
      "strategy": "semantic",
      ...
    },
    "redis": {
      "enabled": false,
      "addr": "localhost:26379",
      ...
    },
    "vector": {
      "enabled": false,
      "type": "milvus",
      ...
    },
    "embedding": {
      "provider": "ollama",
      "model": "bge-m3:latest",
      "base_url": "http://localhost:21434",
      "enabled": true
    },
    "qa_split": {
      "enabled": true,
      "provider": "ollama",
      "model": "qwen2.5:1.5b",
      ...
    },
    "plugins": {
      "dir": "./plugins",
      "enabled": ["protocol/openai", "backend/openai"]
    },
    "system_proxy": {
      "enabled": true,
      "listen_port": 8081,
      "pac_enabled": true,
      "domains": ["api.openai.com", ...],
      "path_patterns": ["/v1/chat/completions", ...]
    },
    "backends": [
      {
        "id": "ollama-Ollama",
        "name": "Ollama",
        "type": "ollama",
        "base_url": "http://localhost:21434",
        "api_key": "...",
        "enabled": true,
        "weight": 100,
        "timeout": 60,
        "max_retries": 3,
        "description": "本地Ollama模型"
      }
    ],
    "storages": [
      {
        "name": "redis",
        "type": "redis",
        "enabled": true,
        "config": {
          "addr": "localhost:26379",
          "db": 0,
          "password": "...",
          "pool_size": 10
        },
        "description": "Redis存储配置"
      }
    ],
    "default_storage": "redis"
  }
}
```

---

### 2. 保存所有配置 (统一接口)

**接口**: `PUT /api/v1/config`

**功能**: 保存完整的配置信息到 `config.yaml`

**请求体**:
```json
{
  "server": { ... },
  "log": { ... },
  "proxy": { ... },
  "cache": { ... },
  "redis": { ... },
  "vector": { ... },
  "embedding": { ... },
  "qa_split": { ... },
  "plugins": { ... },
  "system_proxy": { ... },
  "backends": [ ... ],
  "storages": [ ... ],
  "default_storage": "redis"
}
```

**响应示例**:
```json
{
  "success": true,
  "message": "Configuration saved successfully"
}
```

**说明**:
- 所有字段都是可选的
- 只提供需要更新的配置部分
- 不会覆盖未提供的字段
- 最终保存的是合并后的完整配置

---

### 3. 后端配置管理

#### 3.1 列出所有后端

**接口**: `GET /api/v1/backends`

**功能**: 获取所有后端配置

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": "ollama-Ollama",
      "name": "Ollama",
      "type": "ollama",
      "base_url": "http://localhost:21434",
      "enabled": true,
      "weight": 100,
      ...
    }
  ]
}
```

#### 3.2 创建后端

**接口**: `POST /api/v1/backends`

**请求体**:
```json
{
  "name": "OpenAI",
  "type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "<YOUR_API_KEY_HERE>...",
  "enabled": true,
  "weight": 10,
  "timeout": 60,
  "max_retries": 3
}
```

**说明**: 创建后端会自动保存到 `config.yaml`

#### 3.3 更新后端

**接口**: `PUT /api/v1/backends/:id`

**请求体**:
```json
{
  "id": "ollama-Ollama",
  "name": "Ollama",
  "type": "ollama",
  "base_url": "http://localhost:21434",
  "api_key": "...",
  "enabled": true,
  "weight": 100,
  ...
}
```

**说明**: 更新后端会自动保存到 `config.yaml`

#### 3.4 删除后端

**接口**: `DELETE /api/v1/backends/:id`

**说明**: 删除后端会自动保存到 `config.yaml`

#### 3.5 测试连接

**接口**: `POST /api/v1/backends/test`

**请求体**:
```json
{
  "type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "<YOUR_API_KEY_HERE>..."
}
```

---

### 4. 存储配置管理

#### 4.1 列出所有存储

**接口**: `GET /api/v1/storage`

**功能**: 获取所有存储配置及状态

**响应**:
```json
{
  "storages": [
    {
      "name": "redis",
      "type": "redis",
      "enabled": true,
      "description": "Redis存储配置",
      "config": {
        "addr": "localhost:26379",
        "db": 0,
        ...
      },
      "is_default": true,
      "healthy": true
    }
  ],
  "default_kv": "redis"
}
```

#### 4.2 添加存储

**接口**: `POST /api/v1/storage/add`

**请求体**:
```json
{
  "name": "elasticsearch",
  "type": "elasticsearch",
  "enabled": true,
  "config": {
    "addresses": ["http://localhost:9200"],
    "username": "elastic",
    "password": "..."
  },
  "description": "Elasticsearch存储"
}
```

**说明**: 添加存储会自动保存到 `config.yaml`

#### 4.3 更新存储

**接口**: `POST /api/v1/storage/update`

**请求体**:
```json
{
  "name": "redis",
  "type": "redis",
  "enabled": true,
  "config": { ... },
  "description": "..."
}
```

**说明**: 更新存储会自动保存到 `config.yaml`

#### 4.4 删除存储

**接口**: `DELETE /api/v1/storage?name=xxx` 或 `POST /api/v1/storage/delete`

#### 4.5 设置默认存储

**接口**: `POST /api/v1/storage/set-default`

**请求体**:
```json
{
  "name": "redis"
}
```

**说明**: 设置默认存储会自动保存到 `config.yaml`

---

### 5. QA Split 配置

#### 5.1 获取 QA Split 配置

**接口**: `GET /api/v1/cache/qa-split/config`

**响应**:
```json
{
  "success": true,
  "data": {
    "configured": true,
    "enabled": true,
    "provider": "ollama",
    "model": "qwen2.5:1.5b",
    "base_url": "http://localhost:21434",
    "api_key": "...",
    "timeout": 30,
    "temperature": 0.3,
    "max_tokens": 2000,
    "prompt": "..."
  }
}
```

#### 5.2 更新 QA Split 配置

**接口**: `POST /api/v1/cache/qa-split/config`

**请求体**:
```json
{
  "enabled": true,
  "provider": "ollama",
  "model": "qwen2.5:1.5b",
  "base_url": "http://localhost:21434",
  "api_key": "...",
  "timeout": 30,
  "temperature": 0.3,
  "max_tokens": 2000,
  "prompt": "..."
}
```

**说明**: 更新 QA Split 配置会自动保存到 `config.yaml`

---

### 6. 监控配置信息 (用于仪表板)

**接口**: `GET /api/v1/monitor/config`

**功能**: 获取启用的后端和存储信息（用于监控显示）

**响应**:
```json
{
  "backends": [
    {
      "id": "ollama-Ollama",
      "name": "Ollama",
      "type": "ollama",
      "enabled": true,
      "weight": 100
    }
  ],
  "storages": [
    {
      "name": "redis",
      "type": "redis",
      "enabled": true,
      "default": true
    }
  ],
  "default_kv": "redis"
}
```

---

## 配置保存机制

### 自动保存

以下操作会自动保存配置到 `config.yaml`:

1. ✅ 创建/更新/删除后端 (`/api/v1/backends`)
2. ✅ 添加/更新/删除存储 (`/api/v1/storage`)
3. ✅ 设置默认存储 (`/api/v1/storage/set-default`)
4. ✅ 更新 QA Split 配置 (`/api/v1/cache/qa-split/config`)
5. ✅ 保存所有配置 (`PUT /api/v1/config`)

### 配置优先级

1. `config.yaml` (主配置，唯一配置源)
2. `backends.json` (已废弃，但仍支持向后兼容)

**注意**: 
- `storage.json` 已完全废弃，系统只从 `config.yaml` 加载和保存存储配置
- 建议统一使用 `config.yaml`，删除或忽略旧配置文件

---

## Web 页面集成

### 1. 加载配置

```javascript
// 获取所有配置
async function loadAllConfig() {
  const response = await fetch('/api/v1/config');
  const result = await response.json();
  if (result.success) {
    const config = result.data;
    // 填充表单
    fillServerConfig(config.server);
    fillLogConfig(config.log);
    fillBackends(config.backends);
    fillStorages(config.storages);
    // ...
  }
}
```

### 2. 保存配置

```javascript
// 保存所有配置
async function saveAllConfig(config) {
  const response = await fetch('/api/v1/config', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(config),
  });

  const result = await response.json();
  if (result.success) {
    // 保存成功
    console.log('Configuration saved successfully');
  }
}
```

### 3. 部分更新

```javascript
// 只更新后端配置
async function updateBackend(backend) {
  const response = await fetch(`/api/v1/backends/${backend.id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(backend),
  });

  const result = await response.json();
  if (result.success) {
    // 更新成功，会自动保存到 config.yaml
  }
}
```

---

## 测试清单

### 配置读取测试

- [ ] GET `/api/v1/config` - 返回完整配置
- [ ] GET `/api/v1/backends` - 返回后端列表
- [ ] GET `/api/v1/storage` - 返回存储列表
- [ ] GET `/api/v1/cache/qa-split/config` - 返回 QA Split 配置
- [ ] GET `/api/v1/monitor/config` - 返回监控配置

### 配置保存测试

- [ ] PUT `/api/v1/config` - 保存完整配置
- [ ] POST `/api/v1/backends` - 创建后端并保存
- [ ] PUT `/api/v1/backends/:id` - 更新后端并保存
- [ ] DELETE `/api/v1/backends/:id` - 删除后端并保存
- [ ] POST `/api/v1/storage/add` - 添加存储并保存
- [ ] POST `/api/v1/storage/update` - 更新存储并保存
- [ ] DELETE `/api/v1/storage` - 删除存储并保存
- [ ] POST `/api/v1/storage/set-default` - 设置默认存储并保存
- [ ] POST `/api/v1/cache/qa-split/config` - 更新 QA Split 配置并保存

### 配置回填测试

- [ ] 修改配置后刷新页面，配置值正确回填
- [ ] 添加后端后刷新页面，后端列表正确显示
- [ ] 添加存储后刷新页面，存储列表正确显示
- [ ] 修改 QA Split 配置后刷新页面，配置值正确显示

---

## 故障排查

### 问题1: 配置未保存到 config.yaml

**检查步骤**:

1. 检查日志是否有错误:
   ```bash
   tail -f bin/logs/llm-proxy.log | grep "Failed to save"
   ```

2. 检查 config.yaml 文件权限:
   ```bash
   ls -la bin/configs/config.yaml
   ```

3. 检查文件是否被其他进程锁定:
   ```bash
   lsof bin/configs/config.yaml
   ```

### 问题2: 配置未正确回填

**检查步骤**:

1. 检查 API 响应:
   ```bash
   curl http://localhost:20060/api/v1/config | jq
   ```

2. 检查 config.yaml 内容:
   ```bash
   cat bin/configs/config.yaml
   ```

3. 确认配置文件路径正确:
   ```bash
   pwd  # 应该在 bin 目录或项目根目录
   ```

### 问题3: 配置丢失

**可能原因**:
- 运行了 `make copy-files` 覆盖了修改
- 配置文件路径错误

**解决方法**:
- 始终编辑 `config/initdata/configs/config.yaml`
- 修改后运行 `make copy-files` 同步到 `bin/`
- 或直接在 `bin/configs/config.yaml` 中编辑（不会覆盖）

---

## 下一步

1. 使用新的统一 API 更新 Web 配置页面
2. 测试所有配置的保存和回填功能
3. 删除或废弃旧的 `backends.json` 文件（`storage.json` 已完全移除）
4. 更新文档和用户指南

---

## 相关文档

- [配置统一化指南](../../archive/deprecated/docs/CONFIG_UNIFICATION_GUIDE.md)（已归档）
- [快速更新说明](../CONFIG_UPDATE.md)
- [启动指南](../STARTUP_GUIDE.md)

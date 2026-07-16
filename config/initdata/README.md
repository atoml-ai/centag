# 初始化配置说明

`config/initdata/` 目录包含项目的初始配置文件，这些文件会在 `./start.sh setup` 时自动复制到 `bin/configs/` 目录。

## 📁 目录结构

```
config/initdata/
├── archive/deprecated/configs/                 # 配置文件目录
│   ├── config.yaml         # 主配置文件
│   └── backends.json       # 后端服务配置
├── scripts/                # 初始化脚本
└── README.md               # 本文档
```

## ⚙️ 配置优先级

根据 `Makefile` 中的 `copy-files` 规则，配置文件的管理方式如下：

**✅ 新版本（推荐）：**
- **只从 `config/initdata/configs/` 复制配置到 `bin/configs/`**
- 项目根目录的 `archive/deprecated/configs/` 已被 `.gitignore` 忽略（仅用于开发）
- 所有初始配置统一存放在 `config/initdata/configs/`

**配置文件路径说明：**
1. **`config/initdata/configs/`** - 初始配置源（提交到 Git）
2. **`archive/deprecated/configs/`** - 开发环境临时配置（被忽略，不提交）
3. **`bin/configs/`** - 运行时配置（被忽略，不提交）

这意味着：
- ✅ 所有初始配置都在 `config/initdata/configs/` 中维护
- ✅ 构建时自动复制到 `bin/configs/`
- ✅ 程序从 `bin/configs/` 读取配置
- ✅ Web 界面修改的配置保存到 `bin/configs/`
- ⚠️ 项目根目录的 `archive/deprecated/configs/` 不再用于生产环境

## 🎯 使用场景

### 场景1: 标准部署（推荐）
使用 `config/initdata/configs/` 中的配置，这是推荐的方式：

```bash
# 构建并启动
./start.sh build
./start.sh daemon
```

### 场景2: 自定义初始化配置
修改 `config/initdata/configs/` 目录下的配置文件，然后重新构建：

```bash
# 编辑配置
vim config/initdata/configs/backends.json

# 重新构建
./start.sh build
./start.sh restart
```

### 场景3: 开发环境临时配置
如果需要临时测试配置，可以在项目根目录创建 `archive/deprecated/configs/`（会被 .gitignore 忽略）：

```bash
# 复制配置到开发目录
cp -r config/initdata/configs/ archive/deprecated/configs/

# 修改开发配置
vim archive/deprecated/configs/config.yaml

# 注意：程序仍然从 bin/configs/ 读取
# 需要手动复制到 bin/ 或重新 build
```

### 场景4: 删除 bin 目录后重新部署
配置不会丢失，因为源文件在 `config/initdata/` 中：

```bash
# 删除 bin 目录
rm -rf bin/

# 重新构建，配置自动应用
./start.sh build
./start.sh daemon
```

## 📋 推荐做法

- **`config/initdata/configs/`** - ✅ 存放所有初始配置（提交到 Git）
  - 后端服务配置
  - 系统配置
  - 存储配置
  - 等等...
  
- **`bin/configs/`** - ⚙️ 运行时配置（自动生成，不要手动编辑）
  - 由 `make build` 自动复制
  - 程序实际读取的位置
  - Web 界面修改会保存到这里
  
- **`archive/deprecated/configs/`** - 🔧 开发环境临时配置（可选，已被 .gitignore 忽略）
  - 仅用于本地开发测试
  - 不会被提交到 Git
  - 不会影响生产环境

**配置流程：**
```
config/initdata/configs/ (源)
    ↓ make build / ./start.sh build
bin/configs/ (运行时)
    ↓ 程序读取
Proxy Claw 服务
    ↓ Web 界面修改
bin/configs/ (保存)
```

## 📝 配置文件说明

|| 文件 | 说明 |
||------|------|
|| `config.yaml` | 主配置文件,包含服务器、日志、缓存、代理等配置 |
|| `backends.json` | 后端服务配置,定义可用的 LLM API 后端 |
|| `scripts/post-update.sh` | 更新后执行的脚本 |

### config.yaml 主要配置项

```yaml
# 系统代理配置
system_proxy:
  enabled: true              # 是否启用系统代理
  listen_port: 8081          # MITM代理监听端口
  pac_enabled: true          # 是否启用PAC文件服务
  domains:                   # 需要代理的域名
    - "api.openai.com"
    - "api.anthropic.com"
    - "api.ppinfra.com"
  path_patterns:             # 需要代理的路径
    - "/v1/chat/completions"
    - "/v3/openai/chat/completions"

# 缓存配置
cache:
  enabled: true
  default_ttl: 3600
  strategy: "semantic"
```

### backends.json 配置示例

```json
{
  "backends": [
    {
      "id": "ppinfra-deepseek",
      "name": "PPInfra DeepSeek",
      "type": "openai",
      "base_url": "https://api.ppinfra.com/v3/openai",
      "api_key": "your-api-key",
      "enabled": true,
      "weight": 10,
      "timeout": 60,
      "max_retries": 3,
      "description": "PPInfra DeepSeek R1 API"
    }
  ]
}
```

## 🚀 快速开始

### 1. 配置后端服务

编辑 `config/initdata/configs/backends.json`,添加你的LLM服务

### 2. 启用系统代理(可选)

编辑 `config/initdata/configs/config.yaml`,设置 `system_proxy.enabled: true`

### 3. 构建并启动

```bash
./start.sh setup
./start.sh build
./start.sh daemon
```

## ✅ 验证配置

```bash
# 检查配置是否正确复制
cat bin/configs/config.yaml
cat bin/configs/backends.json

# 检查服务是否加载了配置
tail -20 bin/logs/centag.log | grep "Loaded backend"

# 测试服务
./test_proxy.sh
```

## 📚 相关文档

- [应用代理使用指南](../docs/APPLICATION_PROXY_GUIDE.md) - 详细的应用代理配置说明
- [应用代理快速参考](../docs/APPLICATION_PROXY_QUICK_REFERENCE.md) - 快速参考卡片
- [系统代理完整指南](../docs/SYSTEM-PROXY-GUIDE.md) - 系统代理详细说明
- [启动指南](../STARTUP_GUIDE.md) - 项目启动指南

## 💡 常见问题

### Q: 修改了 config/initdata/configs/ 中的文件,但配置没有生效?

A: 需要重新构建:
```bash
./start.sh build
./start.sh restart
```

### Q: 删除 bin 目录后,配置会丢失吗?

A: 不会。配置保存在 `config/initdata/` 中,每次 `build` 时会自动复制到 `bin/`

### Q: 如何让 Chatbox 使用 LLM 代理?

A: 参考 [应用代理使用指南](../docs/APPLICATION_PROXY_GUIDE.md) 中的详细说明

### Q: 可以在 archive/deprecated/configs/ 和 config/initdata/configs/ 中同时有同名文件吗?

A: 可以,但 `config/initdata/configs/` 中的文件会覆盖 `archive/deprecated/configs/` 中的同名文件

## 🔒 安全建议

1. API Key 应该通过环境变量配置,不要硬编码在配置文件中
2. CA 证书文件 (`certs/ca.key`) 权限应为 `600`
3. 生产环境中使用 `release` 模式而非 `debug` 模式
4. 定期检查和更新配置

## 🎉 完成!

现在你的 Proxy Claw 已经配置好了,每次执行 `./start.sh setup` 和 `./start.sh build` 后,配置会自动应用,无需重复配置!

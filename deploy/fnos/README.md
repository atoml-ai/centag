# Centag fnOS 支持

本目录包含将 Centag 打包为 fnOS（飞牛 OS）应用包（.fpk）所需的脚本和模板。

支持两种部署模式：

| 模式 | 说明 |
|------|------|
| 🐳 **Docker** | 基于 Docker Compose 运行，适合已有 Docker 环境的用户 |
| 🖥️ **Native** | 直接运行 Go 二进制，不依赖 Docker，性能更优 |

---

## 🐳 Docker 模式

### 目录结构

```
deploy/fnos/
├── README.md                 # 本文件
├── build-fpk.sh              # 统一打包脚本
├── manifest                  # 应用清单
├── docker-compose.yaml       # Docker Compose 模板
├── cmd/
│   └── main                  # 状态检测脚本
├── config/
│   ├── privilege             # 权限配置（Docker）
│   └── resource              # 资源声明（含 docker-project）
└── res/
    └── README.md             # 图标说明
```

### 使用方式

推荐经统一渠道入口（默认参数见仓库根目录 `packaging.env`）：

```bash
./scripts/packaging/package.sh fnos --mode docker --arch amd64
# 或
make package TARGET=fnos PACKAGE_MODE=docker PACKAGE_ARCH=amd64
# 或
./start.sh package fnos --mode docker --arch amd64
```

也可直接调用本目录脚本：

```bash
# 构建 Docker 镜像
docker build -t centag:latest -f deploy/docker/Dockerfile .

# 指定架构
./deploy/fnos/build-fpk.sh --mode docker --arch amd64
./deploy/fnos/build-fpk.sh --mode docker --arch arm64

# 打包并安装
./deploy/fnos/build-fpk.sh --mode docker --install
```

---

## 🖥️ Native 模式

Native 模式将 Centag 的 Go 二进制文件和前端静态文件直接打包，无需 Docker 环境。

### 目录结构

```
deploy/fnos/native/
├── manifest                  # 应用清单模板（与顶层共用）
├── ui-config                 # fnOS 桌面入口配置
├── cmd/
│   ├── main                  # 主控制脚本（start/stop/status，3378 字节）
│   ├── install_init          # 安装前初始化
│   ├── install_callback      # 安装后回调
│   ├── uninstall_init        # 卸载前
│   ├── uninstall_callback    # 卸载后
│   ├── upgrade_init          # 升级前备份
│   ├── upgrade_callback      # 升级后回调
│   ├── config_init           # 配置读取
│   └── config_callback       # 配置保存
├── config/
│   ├── privilege             # 以 centag 用户运行
│   └── resource              # 数据共享卷声明（无 docker-project）
└── res/
    └── README.md             # 图标文件说明
```

### 前置条件

| 依赖 | 版本要求 | 说明 |
|------|----------|------|
| Go | 1.25+ | 编译后端二进制 |
| Node.js | 18+ | 编译前端静态文件 |
| npm | 9+ | 安装前端依赖 |

### 使用方式

```bash
# 完整打包（自动构建 + 打包）
./deploy/fnos/build-fpk.sh --mode native

# 指定架构交叉编译
./deploy/fnos/build-fpk.sh --mode native --arch amd64
./deploy/fnos/build-fpk.sh --mode native --arch arm64

# 仅打包（使用已有构建产物，--skip-build）
./deploy/fnos/build-fpk.sh --mode native --skip-build

# 打包并安装到 fnOS
./deploy/fnos/build-fpk.sh --mode native --skip-build --install
```

### cmd/main 控制脚本说明

Native 模式的 `cmd/main` 脚本参考 fnOS 官方 [Notepad 示例](https://developer.fnnas.com/docs/core-concepts/native) 实现，支持三种操作：

| 操作 | 行为 |
|------|------|
| `start` | 设置环境变量 → 启动 Centag 二进制 → 写入 PID |
| `stop` | 读取 PID → 发送 TERM 信号 → 等待 → 必要时 KILL |
| `status` | 检查 PID 文件 → 检查进程是否存在 → 返回 0（运行中）/ 3（未运行） |

### 环境变量（由 cmd/main 自动设置）

| 变量 | 值 |
|------|-----|
| `LLM_PROXY_DB_DRIVER` | `sqlite` |
| `SQLITE_PATH` | `${TRIM_DATA_SHARE_PATHS}/centag.db` |
| `SERVER_HOST` | `0.0.0.0` |
| `SERVER_PORT` | `20060` |
| `LLM_PROXY_ADMIN_PASSWORD` | 来自包内 `config/runtime.env`（打包时注入） |
| `LLM_PROXY_ADMIN_API_KEY` / `DEFAULT` | 同上；首轮 seed 预置管理员 API Key |
| `LLM_PROXY_API_KEY_STORAGE_SECRET` | 同上；**必须有**，否则预置 Key 无法在 Web 复制完整密钥 |
| `STATIC_DIR` / `STATIC_PATH` | `${TRIM_APPDEST}/webui` |

---

## 两种模式 app.tgz 内部结构对比

| Native 模式（二进制） | Docker 模式 |
|---|---|
| `bin/centag`（Go 二进制） | `docker/docker-compose.yaml` |
| `webui/`（前端静态文件） | `docker/image.tar.gz`（Docker 镜像） |
| `ui/config`（.url 格式） | `ui/config`（.url 格式） |
| `ui/images/icon_*.png` | `ui/images/icon_*.png` |
| `config/initdata/`（初始配置） | — |
| `scripts/`（辅助脚本） | — |
| `update_config.yml` | `update_config.yml` |

---

## 通用注意事项

1. **图标文件**：`res/` 目录下的图标文件因版权原因不提交到 Git，需手动放置
2. **端口**：Docker 和 Native 模式均使用 `20060` 端口
3. **数据目录**：
   - Docker: 由 fnOS 的 `data-share` 自动管理
   - Native: 由 fnOS 的 `TRIM_DATA_SHARE_PATHS` 环境变量指定
4. **管理员密码 / 默认 API Key / 存储密钥**：打包时写入 `config/runtime.env`：
   - 密码：`--admin-password` / `PACKAGE_ADMIN_PASSWORD` / `.env` 的 `LLM_PROXY_ADMIN_PASSWORD`
   - API Key：`PACKAGE_ADMIN_API_KEY` / `.env` 的 `LLM_PROXY_ADMIN_API_KEY`（或 `DEFAULT`）
   - **`LLM_PROXY_API_KEY_STORAGE_SECRET`**（Web 复制完整 Key 必需）：`PACKAGE_API_KEY_STORAGE_SECRET` / `.env`；均空则打包时**自动生成**
   - **已安装过的数据目录**若已有用户/Key，不会被新包覆盖——需清空数据卷后重装，或在 Web 新建 Key（需已配置 STORAGE_SECRET）
5. **发行版**：默认 `minimal`；`--edition personal|team` 分别对应个人全功能 / 团队版。

# Centag fnOS FPK 构建指南与常见问题

## 一、构建命令

推荐统一入口（默认参数：仓库根目录 `packaging.env`）：

```bash
./scripts/packaging/package.sh fnos --mode native --arch amd64
make package TARGET=fnos PACKAGE_MODE=native PACKAGE_ARCH=amd64
./start.sh package fnos --mode native --arch amd64
```

### Native（二进制）模式
```bash
# 构建当前架构 native 包
./deploy/fnos/build-fpk.sh --mode native

# 指定架构
./deploy/fnos/build-fpk.sh --mode native --arch amd64
./deploy/fnos/build-fpk.sh --mode native --arch arm64

# 跳过 Go 编译（使用已有二进制）
./deploy/fnos/build-fpk.sh --mode native --skip-build

# 构建并安装
./deploy/fnos/build-fpk.sh --mode native --skip-build --install
```

### Docker 模式
```bash
# 构建 Docker 包（镜像推送至 ghcr.io）
./deploy/fnos/build-fpk.sh --mode docker --arch amd64

# 指定自定义镜像前缀
./deploy/fnos/build-fpk.sh --mode docker --arch amd64 --image-prefix myregistry.com/
```

> macOS 无 `md5sum` 时脚本会自动回退到 `md5` / `openssl`。

---

## 二、文件结构要求

### FPK 包顶层结构（两种模式通用）

```
centag-native-amd64.fpk
├── manifest              # 📄 应用清单（必须）
├── app.tgz               # 📦 应用数据包（必须）
├── cmd/                  # 📁 命令脚本（必须，含 main + 8个生命周期脚本）
│   ├── main              # - 主管理脚本（start/stop/status）
│   ├── install_init      # - 安装前初始化
│   ├── install_callback  # - 安装后回调
│   ├── uninstall_init    # - 卸载前
│   ├── uninstall_callback# - 卸载后
│   ├── upgrade_init      # - 升级前
│   ├── upgrade_callback  # - 升级后
│   ├── config_init       # - 配置页打开时
│   └── config_callback   # - 配置保存后
│   ├── uninstall_callback# - 卸载后
│   ├── upgrade_init      # - 升级前
│   ├── upgrade_callback  # - 升级后
│   ├── config_init       # - 配置页打开时
│   └── config_callback   # - 配置保存后
├── config/               # 📁 应用配置
│   ├── privilege         # - 权限定义（JSON）
│   └── resource          # - 资源定义（JSON）
├── wizard/               # 📁 用户向导（新版 fnOS 格式要求存在；安装/卸载/配置）
│   ├── install           # - 安装向导（数据库选择）
│   ├── uninstall         # - 卸载向导（数据保留选择）
│   └── config            # - 应用设置页向导（数据库 + 卸载数据保留）
├── ICON.PNG              # 📄 图标（64x64）
└── ICON_256.PNG          # 📄 图标（256x256）
```

> **注意（旧格式局限）**：fnOS 0.8.x 老格式（不存在 `app/` 目录、不接受 `wizard/`）无法使用向导。当前已迁移到**新版格式**（`wizard/` + `manifest` 用 `platform=` 而非弃用的 `arch=`），在 fnOS 1.x 上可获得安装/卸载/配置向导。若目标设备为 0.8.x 老版，则退回到「配置页 / centag.conf」机制。

### 数据保留与数据库配置

| 能力 | 实现方式 |
|---|---|
| 卸载向导选保留/删除 | `wizard/uninstall` → `$wizard_data_action`；`cmd/uninstall_callback` 据此清理或保留（显式选择优先于配置文件） |
| 卸载保留（配置页） | `cmd/uninstall_callback` 默认**保留**数据；`centag.conf` 中 `clean_data_on_uninstall=true` 时清理 |
| 安装时选数据库 | `wizard/install` → `$wizard_db_driver`/`$wizard_pg_*`；`cmd/install_callback` 写入 `centAG.conf` |
| 切换 PostgreSQL | 保存 `db_driver=postgresql` + `pg_*` 连接参数 → `cmd/main` 启动时读取注入环境变量 |
| 配置持久化 | `cmd/config_callback` 将 JSON 写入 `${DATA_DIR}/centAG.conf`（兼容 `$1` 旧调用/`wizard_*` 新调用） |
| Web 配置页 | 配置页「部署与数据」同样写 `centAG.conf`（含 PG 密码脱敏） |

> `centAG.conf` 使用**单行紧凑 JSON**；`cmd/main` 的 `json_get` 已兼容单行与多行。`manifest` 用 `platform=x86`（x86_64 二进制）与 `maintainer=atomlai` / `maintainer_url=https://github.com/atoml-ai/centag`。

### app.tgz 内部

| Native 模式（二进制） | Docker 模式 |
|---|---|
| `bin/centag`（Go 二进制） | `docker/docker-compose.yaml` |
| `webui/`（前端静态文件） | `ui/config` |
| `config/initdata/`（初始配置） | `ui/images/icon_*.png` |
| `ui/config` | `update_config.yml` |
| `ui/images/icon_*.png` | |
| `scripts/post-update.sh` | |
| `update_config.yml` | |

---

## 三、关键文件详解与陷阱

### 3.1 生命周期脚本（cmd/ 目录）

**⚠️ 最重要的常见失败原因**

fnOS 验证时会检查所有 cmd 脚本是否有效。**空文件（0字节）会导致"应用宝不符合系统要求"错误。**

- **Native 模式**：使用 `deploy/fnos/native/cmd/` 下的完整脚本（含实际逻辑）
- **Docker 模式**：`cmd/main` 用简单的状态回显；**生命周期脚本也要用 `native/cmd/` 下的非空脚本**（虽然 Docker 实际不调用它们，但 fnOS 验证脚本时需要它们存在且有效）

```bash
# ✅ 正确做法：从 native/cmd/ 复制
cp deploy/fnos/native/cmd/install_init  deploy/fnos/cmd/install_init

# ❌ 错误做法：创建空文件（导致验证失败）
: > cmd/install_init    # 0字节 = 无效脚本
```

### 3.2 config/privilege

| 字段 | Native 模式 | Docker 模式 |
|---|---|---|
| `run-as` | `package` | `package` |
| `username` | `centag` | `docker-centag` |
| `groupname` | `centag` | `docker-centag` |

- **Native 必须使用 `deploy/fnos/native/config/privilege`**（多行 JSON，username=centag）
- **Docker 使用基础 config**（单行 JSON，username=docker-centag）
- JSON 格式必须合法（缩进无关紧要，key/value 正确即可）

### 3.3 config/resource

```json
// Native 模式（无 docker-project）
{
    "data-share": {
        "shares": [
            {"name": "centag", "permission": {"rw": ["centag"]}},
            ...
        ]
    }
}

// Docker 模式（含 docker-project）
{
    "docker-project": {
        "projects": [
            {"name": "centag", "path": "docker"}
        ]
    },
    "data-share": {
        "shares": [
            {"name": "centag", "permission": {"rw": ["docker-centag"]}},
            ...
        ]
    }
}
```

**不能混用**——Native 模式不能有 `docker-project`，Docker 模式需要它。

### 3.4 ui/config

两种模式均使用 `.url` 格式（fnOS 0.8.0+ 推荐方式）：

```json
{
    ".url": {
        "centag.Application": {
            "title": "Centag",
            "icon": "images/icon_{0}.png",
            "type": "url",
            "protocol": "http",
            "port": "20060"
        }
    }
}
```

**关键说明**：
- `type`: `url`（Web 应用）
- `protocol`: `http`（fnOS 默认生成 HTTPS 反代，协议用 http 即可）
- `port`: `20060`（必须与 `manifest` 的 `service_port` 一致）
- `.url` key 必须以点开头

**生成方式**：
- Docker 模式：构建脚本通过 heredoc 在 `${APP_DIR}/ui/config` 生成
- Native 模式：从 `deploy/fnos/native/ui-config` 复制到 `${APP_DIR}/ui/config`

### 3.5 manifest

两种模式共用同一份 `deploy/fnos/manifest`，关键字段：

```
appname=centag
version=1.0.0
arch=x86_64
source=thirdparty
desktop_uidir=ui
desktop_applaunchname=centag.Application
service_port=20060
checkport=true
os_min_version=0.8.0
checksum=<manifest 文件的 MD5>
```

- `checksum` 由构建脚本自动计算，**不允许手动修改**
- `service_port` 必须与 `ui/config` 中的 port 一致
- 对 `manifest` 的任何内容修改都必须重新构建（重新计算 checksum）

### 3.6 update_config.yml

放在 `app.tgz` 根目录，用于定义应用的自动更新策略。两种模式都需要。

路径：`config/initdata/update/update_config.yml` → 复制到 `APP_DIR/update_config.yml`

---

## 四、构建脚本配置项

| 变量 | 默认值 | 说明 |
|---|---|---|
| `MODE` | `docker` | 构建模式：`docker` 或 `native` |
| `ARCH` | `amd64` | 目标架构 |
| `IMAGE_PREFIX` | `ghcr.io/marmotcai/` | Docker 镜像前缀 |
| `IMAGE_TAG` | `latest` | Docker 镜像标签 |
| `SKIP_BUILD` | `false` | 跳过 Go 编译 |
| `INSTALL_AFTER` | `false` | 构建后自动安装 |

---

## 五、常见问题排查

### Q1: 安装时提示"应用宝不符合系统要求"

**原因**：fnOS 验证 fpk 包结构或脚本时失败。

排查步骤：
```
1. 检查 cmd/ 目录下所有脚本是否非空且有 shebang（#!/bin/bash）
2. 检查 FPK 顶层**不得包含 wizard/ 目录**（实测会导致"应用包不符合系统要求"）
3. 验证 config/privilege 和 config/resource 的 JSON 合法性
4. 确认 Native/Docker 模式使用了对应模式的 config 文件
5. 检查 manifest 的 checksum 是否正确（构建脚本自动生成）
```

### Q2: Native 应用启动失败

**原因**：`cmd/main` 脚本或 `config/privilege` 配置错误。

排查：
```
1. cmd/main 是否有正确的 shebang 和管理逻辑
2. config/privilege 中 username 是否为 "centag"（非 "docker-centag"）
3. config/resource 中是否误含 docker-project 字段
4. ui/config 是否使用 .url 格式
```

### Q3: Docker 应用启动失败

**原因**：镜像未正确构建或 docker-compose.yaml 配置问题。

排查：
```
1. 确认 Docker 镜像已构建：docker images | grep centag
2. docker-compose.yaml 中 image: 标签是否正确
3. 端口映射、卷挂载路径是否正确
4. 如果是本地镜像确保已推送到可访问的 Registry
```

### Q4: 前后端界面未加载

**原因**：`ui/config` 内容不正确或 `manifest` 字段缺失。

排查：
```
1. manifest 中有 desktop_uidir=ui 和 desktop_applaunchname=centag.Application
2. ui/config 中的 main/.url 与 manifest 中的 applaunchname 一致
3. 图标文件（icon_64.png, icon_256.png）存在且有效
```

---

## 六、开发工作流

```
1. 修改代码 → git commit
2. 构建 Native：bash deploy/fnos/build-fpk.sh --mode native --skip-build
3. 上传至 fnOS 应用中心测试
4. 如问题回退到步骤1
5. Native 验证通过后，构建 Docker 版：bash deploy/fnos/build-fpk.sh --mode docker
```

### Native 模式调试技巧

```bash
# 分析已构建 fpk 的内部结构
tar -tzf ~/.centag/var/packages/centag-minimal-native-amd64.fpk

# 单独提取 app.tgz 查看
mkdir -p /tmp/fpk_debug && cd /tmp/fpk_debug
tar -xzf "$HOME/.centag/var/packages/centag-minimal-native-amd64.fpk" app.tgz
tar -xzf app.tgz
ls -la

# 与已知工作包对比
python3 -c "
import tarfile, hashlib
# 写对比脚本
"
```

---

## 七、文件模板索引

| 文件 | Native 路径 | Docker 路径 |
|---|---|---|
| manifest | `deploy/fnos/manifest` | 同左 |
| cmd/main | `deploy/fnos/native/cmd/main` | `deploy/fnos/cmd/main` |
| 生命周期脚本 | `deploy/fnos/native/cmd/*` | 复用 native/cmd/* |
| wizard/install | -（不打包 wizard） | - |
| wizard/uninstall | -（不打包 wizard） | - |
| config/privilege | `deploy/fnos/native/config/privilege` | `deploy/fnos/config/privilege` |
| config/resource | `deploy/fnos/native/config/resource` | `deploy/fnos/config/resource` |
| ui/config | `deploy/fnos/native/ui-config` | 通过 heredoc 生成 |
| docker-compose.yaml | - | `deploy/fnos/docker-compose.yaml` |

---

## 八、构建脚本 diff 关键记录

已修复的历史问题（commit b1657d1 基础上修改）：

1. **wizard/ 目录不打包**：实测 wizard/ 会导致 fnOS 0.8.x "应用包不符合系统要求"；数据保留/数据库切换改用 config 配置页实现
2. **uninstall_callback 默认保留数据**：仅 `centag.conf` 中 `clean_data_on_uninstall=true` 时清理
3. **cmd/main 支持 PostgreSQL**：启动时读取 `centag.conf` 的 `db_driver`/`pg_*`，支持外部 PG
4. **config 持久化**：config_callback 将配置写入 `${DATA_DIR}/centag.conf`
5. **Native cmd 路径**：`cp -r "${SCRIPT_DIR}/cmd/"*` → `cp -r "${SCRIPT_DIR}/native/cmd/"*`
6. **Native config 路径**：使用 `native/config/privilege` 和 `native/config/resource`
7. **Native ui/config 路径**：优先使用 `native/ui-config`
8. **Docker 生命周期脚本**：从创建空文件改为复制 `native/cmd/` 下的有内容脚本
9. **update_config.yml 加入**：两种模式 app.tgz 中都纳入
10. **Docker 镜像前缀**：默认改为 `ghcr.io/marmotcai/`
11. **config 条件分支**：按模式选择不同路径的 config 文件

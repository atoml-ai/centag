# 第三方系统 / 渠道打包

统一入口，把 NAS / 离线镜像等**渠道产物**与开发用 `make build`、热更新 `./start.sh pack` 分开。

## 入口

| 方式 | 示例 |
|------|------|
| 脚本 | `./scripts/packaging/package.sh fnos --mode native --arch amd64` |
| Make | `make package TARGET=fnos` |
| start.sh | `./start.sh package fnos --mode native --arch amd64` |
| 列目标 | `./scripts/packaging/package.sh list` |

## 默认参数

仓库根目录 [`packaging.env`](../../packaging.env)：

- `PACKAGE_ARCH` / `PACKAGE_MODE` / `PACKAGE_OUTPUT`（默认 `bin/packages`，勿写回 `dist/`）
- `PACKAGE_EDITION`（默认 `minimal`；可选 `personal` / `team`）
- `IMAGE_PREFIX`（docker 模式镜像前缀）
- `PACKAGE_APP_NAME` / `PACKAGE_APP_ID`（品牌标识，固定为 Centag）
- 管理员密码：`--admin-password` > `PACKAGE_ADMIN_PASSWORD` > `config/secrets/.env`

环境变量与 CLI 均可覆盖。fnOS 会把密码写入包内 `config/runtime.env`（勿把含真实密码的私有包提交到 Git）。

## 已注册目标

| 目标 | 说明 | 实现 |
|------|------|------|
| `fnos` | 飞牛 OS `.fpk` | `deploy/fnos/build-fpk.sh` |
| `docker-offline` | Docker 离线包 | `./start.sh docker pack` |

## 新增渠道

1. 在 `deploy/<channel>/` 放构建脚本与模板。
2. 在 `scripts/packaging/package.sh` 的 `case` 中注册目标名。
3. 如需新默认项，写入根目录 `packaging.env` 并在 README 说明。
4. 产物命名使用 `centag-...`，文案使用 **Centag**（勿写 ProxyClaw）。

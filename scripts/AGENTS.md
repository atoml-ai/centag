# scripts/ — 脚本目录

> 面向 Agent：本目录包含构建、测试、运维脚本。

## 目录职责

存放各种自动化脚本，按功能分类。

## 子目录

| 目录 | 用途 |
|------|------|
| `cert/` | 证书生成脚本 |
| `db/` | 数据库相关脚本 |
| `docker/` | Docker 辅助脚本 |
| `ollama/` | Ollama 相关脚本 |
| `ops/` | 运维脚本 |
| `packaging/` | 部署包统一入口（形态 cli/desktop × 系统 macos/linux/windows/fnos/docker） |
| `test/` | 测试脚本 |

## 核心脚本

| 脚本 | 用途 |
|------|------|
| `install.sh` | 一键安装（`curl \| bash`；默认 CLI；Win/mac 用 `--desktop`） |
| `release/build-github-artifacts.sh` | GitHub 渠道产物（全平台 CLI + 本机 desktop） |
| `release/package-desktop.sh` | 本机 desktop 包（dmg/zip） |
| `release/build-artifacts.sh` | 纯 CLI 交叉编译（npm / linux 段） |
| `release/publish-binaries.sh` | 构建并上传 GitHub Release |
| `release/require-release-branch.sh` | 发版门禁：仅版本分支（`vX` / `feature/vX` / `release/vX`） |
| `release/sync-npm-version.sh` | 将 npm `package.json` 版本对齐到发版版本（centag / offline / wrap-npm） |
| `release/require-release-branch_test.sh` | 上述门禁的表驱动 shell 测试 |
| `check-harness-hygiene.sh` | Harness 卫生检查（Centag 布局） |
| `ci-go-packages.sh` | CI 包列表生成 |
| `packaging/package.sh` | 部署包调度：`package <cli\|desktop> <os> [arch]`（见 `packaging.env`） |
| `log_analyzer.py` | 日志分析 |

## 约束

- ❌ **禁止**脚本中硬编码密钥
- ❌ **禁止**脚本修改生产数据（除非明确设计）
- ✅ **必须**：脚本有执行权限
- ✅ **必须**：脚本有注释说明

## 运行方式

```bash
# 构建（统一入口）
make build
make frontend

# Harness 检查
bash scripts/check-harness-hygiene.sh

# 发版分支门禁测试
bash scripts/release/require-release-branch_test.sh

# CI 包列表
bash scripts/ci-go-packages.sh
```

发版流程正本：`docs/harness/skills/step6-release/`（Step 6；须先过 Gate 4）。

## 相关文档

- Makefile：`../Makefile`
- CI 配置：`.github/workflows/ci.yml`

---

*最后更新：2026-07-21*

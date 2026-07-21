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
| `packaging/` | 第三方系统/渠道打包统一入口（fnOS 等） |
| `test/` | 测试脚本 |

## 核心脚本

| 脚本 | 用途 |
|------|------|
| `install.sh` | 一键安装（`curl \| bash`；默认 personal + proxyctl） |
| `release/build-artifacts.sh` | 交叉编译 Release 产物（tar.gz + checksums） |
| `release/publish-binaries.sh` | 构建并上传 GitHub Release |
| `check-harness-hygiene.sh` | Harness 卫生检查（Centag 布局） |
| `ci-go-packages.sh` | CI 包列表生成 |
| `packaging/package.sh` | 渠道打包调度（参数见根目录 `packaging.env`） |
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

# CI 包列表
bash scripts/ci-go-packages.sh
```

## 相关文档

- Makefile：`../Makefile`
- CI 配置：`.github/workflows/ci.yml`

---

*最后更新：2026-04-27*

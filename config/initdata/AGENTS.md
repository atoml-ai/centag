# config/initdata/ — 初始化数据

> 面向 Agent：本目录包含系统初始化所需的数据文件。

## 目录职责

存放首次启动或重置时需要加载的初始数据。

## 子目录

| 目录 | 用途 |
|------|------|
| `scripts/` | 初始化脚本 |
| `config/secrets/` | 密钥模板（不提交真实密钥） |
| `update/` | 更新脚本 |
| `rule/` | 规则文件 |
| `postgresql/` | PostgreSQL 初始化 |

## 核心文件

| 文件 | 用途 |
|------|------|
| （已移除运行时全量 backends） | 后端种子在各 `config/profiles/<edition>/initdata/initial-backends.yaml`；参考目录见 `_shared/backends-catalog.yaml` |

## 使用方式

初始化数据在以下场景加载：
1. 首次启动（数据库为空）
2. 执行 `make gen-init-sqlite`
3. Docker 构建时

## 约束

- ❌ **禁止**提交真实的密钥到 `config/secrets/`
- ❌ **禁止**手动修改已加载的数据（应通过 API）
- ✅ **允许**：添加新的初始化数据
- ✅ **允许**：更新 `initial-backends.json`

## 相关文档

- 后端配置：`docs/guide/backend-configuration.md`
- 数据库迁移：`database/`

---

*最后更新：2026-04-27*

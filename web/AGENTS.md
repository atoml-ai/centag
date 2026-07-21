# web/ — 前端项目

> 面向 Agent：本目录是 Vue 3 前端项目，构建产物输出到 `static/`。

## 目录职责

Centag 的 Web 管理界面，提供后端管理、缓存监控、配置管理等功能。

## 技术栈

- Vue 3 + TypeScript
- Vite 构建
- ESLint 代码检查

## 核心文件

| 文件 | 用途 |
|------|------|
| `package.json` | 依赖配置 |
| `vite.config.ts` | Vite 配置 |
| `eslint.config.js` | ESLint 配置 |
| `src/` | 源代码 |

## 常用命令

```bash
# 安装依赖
npm install

# 开发模式
npm run dev
# 或
./dev.sh

# 构建
npm run build
# 或
./build.sh

# Lint 检查
npm run lint:ci
```

## 目录结构

```
web/
├── src/
│   ├── api/          ← API 调用
│   ├── components/   ← 组件
│   ├── views/        ← 页面
│   ├── router/       ← 路由
│   ├── store/        ← 状态管理
│   └── utils/        ← 工具函数
├── public/           ← 静态资源
└── dist/             ← 构建输出（不提交）
```

## 约束

- ❌ **禁止**提交 `node_modules/`
- ❌ **禁止**提交 `dist/`（构建产物）
- ❌ **禁止**在前端硬编码后端地址（使用环境变量）
- ✅ **必须**：提交前运行 `npm run lint:ci`
- ✅ **必须**：遵循 Vue 3 Composition API 风格

## 环境变量

```bash
# .env.development
VITE_API_BASE_URL=http://localhost:20060
```

## 构建输出

构建产物默认输出到 `~/.centag/lib/<edition>/static/`（与 `scripts/install.sh` 一致），由后端服务提供。

## 相关文档

- 前端 README：`README.md`
- 后端 API：`docs/api/`
- 配置指南：`docs/guide/configuration.md`

---

*最后更新：2026-04-27*

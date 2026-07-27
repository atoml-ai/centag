# Centag 文档索引

## 快速开始

| 发行版 | 场景 | 命令 |
|--------|------|------|
| `personal` | 个人全功能，默认 SQLite | `./start.sh run personal` |
| `minimal` | 轻量单机 / CLI | `./start.sh run minimal` |
| Docker | 容器化部署 | `./start.sh docker up` |

根 README：[`../README.md`](../README.md)。

**架构说明**：
- [发行版构建规格（Dist Profiles）](guide/dist-profiles.md)
- [deploy/stack 快速开始](../deploy/stack/docs/getting-started.md)
- [业务插件外置](guide/external-business-plugins.md)

## 使用指南

- [代理模式](guide/proxy-modes.md)
- [模式行为矩阵](guide/mode-behavior-matrix.md)
- [缓存指南](guide/Cache-Guide.md)
- [语义缓存指南](guide/Semantic-Cache-Guide.md)
- [后端配置](guide/backend-configuration.md)
- [存储配置](guide/storage-configuration.md)
- [流水线变量参考](guide/pipeline-variables.md)
- [流水线节点插件标准](guide/pipeline-plugin-standard.md)
- [Processor 插件指南](guide/processor-plugins.md)
- [认证指南](guide/AUTHENTICATION_GUIDE.md)
- [Ollama 后端配置](guide/Ollama-Backend-Configuration.md)
- [Ollama 排障](guide/Ollama-Troubleshooting.md)

## API

- [多租户 API](api/tenant.md)

## 架构

- [配置架构](architecture/config-architecture.md)
- [路由策略](architecture/routing-strategy.md)

## Harness

| 文档 | 用途 |
|------|------|
| [AGENTS.md](harness/AGENTS.md) | 项目总纲 |
| [ARCHITECTURE.md](harness/ARCHITECTURE.md) | 架构约束 |
| [CONVENTIONS.md](harness/CONVENTIONS.md) | 编码规范 |
| [PATTERNS.md](harness/PATTERNS.md) | 设计模式 |
| [ANTI-PATTERNS.md](harness/ANTI-PATTERNS.md) | 反模式 |

## 安全

- [权限模型](security/permission-model.md)
- [插件安全指南](security/plugin-security-guide.md)
- [插件准入检查](security/plugin-admission-checklist.md)
- [网络策略](security/network-policy.md)
- [白名单策略](security/allowlist-policy.md)

## 版本归档

- [versions/README.md](versions/README.md) — 按版本存放需求产物（技术方案 / 任务计划等）

## Docker

- [Docker Compose 部署指南](../deploy/docker/docs/Docker-Compose部署指南.md)
- [Docker Compose 常见问题](../deploy/docker/docs/Docker-Compose常见问题.md)
- [Docker Compose 快速参考](../deploy/docker/docs/Docker-Compose快速参考.md)

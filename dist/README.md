# Dist 发行版入口（开源）

开源仓只包含可独立构建的发行版：

| 目录 | SKU | 说明 |
|------|-----|------|
| `minimal/` | minimal | 轻量，无 DB |
| `personal/` | personal | 个人全功能 |

## Team（商业版）

**本仓库不再包含 `dist/team`，也不再提供 `./start.sh build team` 转调。**

Team SKU **只**在私有仓 [`centag-pro`](https://github.com/atoml-ai/centag-pro) 构建：

```bash
cd ../centag-pro          # 与 centag 并列；分支须同名
export CENTAG_ROOT=../centag
./start.sh build team     # → bin/centag-team（对齐开源 ./start.sh build <sku>）
./start.sh build fe       # Team 前端 pack
./start.sh build all      # 后端 + 前端
```

### 分支同步

`centag-pro` 与本仓 **使用相同分支名**（例如本仓 `feature/v0.2.7` → pro 也必须是 `feature/v0.2.7`）。  
开发/发版时两侧同时开同名分支；pro 的 `build-team.sh` 在分支不一致时会警告（`CENTAG_PRO_STRICT_BRANCH=1` 时失败）。

架构说明见 [centag-pro](https://github.com/atoml-ai/centag-pro) 仓内 `docs/versions/v0.2.7/commercialization-layered/技术方案.md`（闭源）。

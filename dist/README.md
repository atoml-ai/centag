# Dist 发行版入口（开源）

开源仓只包含可独立构建的发行版：

| 目录 | SKU | 说明 |
|------|-----|------|
| `minimal/` | minimal | 轻量，无 DB |
| `personal/` | personal | 个人全功能 |

## Team（商业版）

**本仓库不再包含 `dist/team`。**

Team SKU 由私有仓 [`centag-pro`](https://github.com/atoml-ai/centag-pro) 构建：

```bash
# 并列克隆 centag-pro 后：
./start.sh build team
# 等价于转调 ../centag-pro/scripts/build-team.sh
```

或设置 `CENTAG_PRO_PATH`。缺少 pro 时构建会失败（不会产出残缺 team 二进制）。

### 分支同步

`centag-pro` 与本仓 **使用相同分支名**（例如本仓 `v0.2.7` → pro 也必须是 `v0.2.7`）。  
开发/发版时两侧同时开同名分支；构建脚本在分支不一致时会警告（`CENTAG_PRO_STRICT_BRANCH=1` 时失败）。

架构说明见 `docs/versions/v0.2.7/commercialization-layered/技术方案.md`。

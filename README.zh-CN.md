# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

**本机一键代理接入** 各类编码 Agent，**统一管理**后端与 API Key，再按场景**配置代理动作**（切换、容错、流水线）——不用每个工具各配一遍。

适合个人开发者：装好 Centag → wrap 或改配置接入 Agent → Web 上管后端与策略。

## 安装

任选一种方式。装好后运行 `centag`，浏览器打开 **http://localhost:20060**。

### 方式一：一键脚本（推荐，无需 Node.js）

```bash
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
```

默认安装到 `~/.centag/`，并尽量写入 PATH。装完即可用 `centag` / `centag wrap`。

### 方式二：npm（已有 Node.js 时）

```bash
# 全局安装（在线版，安装时从 Release 拉二进制）
npm install -g @atomlai/centag

# 或：不改全局目录，直接试用
npx --yes @atomlai/centag

# 内网 / 离线包
npm install -g @atomlai/centag-offline
```

若 `npm install -g` 报权限错误，可用 `npx`，或改用上面的一键脚本。说明见 [apps/centag-npm/README.md](apps/centag-npm/README.md)。

### 方式三：Docker（源码仓库）

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # 按需改密钥
./start.sh docker up                                 # 默认 personal 容器
```

管理界面同样是 http://localhost:20060。停服务：`./start.sh docker down`。

---

## 装好后：如何接入 Agent？

目标：Agent 继续照常用，流量走 Centag（统一后端、容错与计量）。

1. **打开 Web** → 添加并启用至少一个后端（API Key / 本地兼容端点均可）。
2. **Agent 接入**（Web 菜单「Agent 接入」）按向导为常用工具生成/写入配置；或
3. **进程代理（推荐，少改 Agent 配置）**：

```bash
# 本机已启动 Centag 时，用 wrap 拉起 Agent（自动设代理环境）
centag wrap run -- opencode
# 把 opencode 换成你的 Agent 启动命令即可

# 自检
centag wrap doctor
```

说明：`centag wrap` **不起网关**，只负责把当前 Agent 进程的流量导入已在运行的 Centag。更完整的步骤见 [本机代理出口指南](docs/guide/system-proxy-egress.md)。

---

## 核心优势

| 你真正需要的 | Centag 怎么做 |
|--------------|----------------|
| **快速切换后端** | 多后端统一管理；Web 上一键启用/切换，Agent 侧不用改来改去 |
| **自动容错 + API 池** | 多 Key 轮转、失败自动换路；单个 Key 限流或挂掉时尽量不断服务 |
| **流水线适配场景** | 透明转发、直连、调度、翻译、审核等模式可配；换场景等于换策略，不用重写客户端 |
| **计费与计量** | Token / 费用可追踪，个人用量心里有数，也方便日后对账 |

一句话：**一个入口管后端与策略，Agent 只负责写代码。**

## 能力列表

1. **后端 / 模型与 API Key 池**  
   在 Web 中配置后端与模型；同一后端支持**多个 API Key 池化轮换**，限流或失效时自动换 Key。

2. **流水线可视化编辑**  
   用画布自定义代理行为（转发、调度、审核等节点），按场景切换策略，无需改 Agent 代码。

3. **`centag wrap` 无损接入第三方 Agent**  
   用 wrap 启动 Agent，把流量导入 Centag，**不必改 Agent 自身设置**（适合不想动配置文件的用法）。

4. **直接改 Agent 配置文件接入**  
   也可把 Agent 的 API Base / Key 指到 Centag，当作普通大模型网关使用（Web「Agent 接入」向导可辅助写入）。

两种 Agent 接法可按习惯二选一：wrap 少改配置；改配置文件则走标准 OpenAI 兼容入口。

## 截图

| 仪表盘 | Agent 接入 |
|--------|------------|
| ![仪表盘](docs/assets/readme/dashboard.png) | ![Agent 接入](docs/assets/readme/agent-setup.png) |

## 文档

- [文档索引](docs/README.md)
- [环境变量](docs/guide/environment-variables.md)
- [本机代理 / wrap](docs/guide/system-proxy-egress.md)
- [API 参考](docs/api/API_REFERENCE.md)

## 反馈与支持

有问题或建议：请提 [GitHub Issues](https://github.com/atoml-ai/centag/issues)，或发邮件 **centag@atoml.com**。

## 许可证

MIT License（开源发行版：`minimal` / `personal`）

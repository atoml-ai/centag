# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

**One-click local proxy access** for coding Agents, **unified management** of backends and API keys, plus **configurable proxy actions** per scenario (switching, failover, pipelines)—no more configuring every tool separately.

For individual developers: install Centag → connect Agents via wrap or config → manage backends and policies in the Web UI.

## Install

Pick one method. After install, run `centag` and open **http://localhost:20060**.

### Option 1: One-line script (recommended, no Node.js)

```bash
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
```

Installs to `~/.centag/` by default and tries to update your PATH. Then use `centag` / `centag wrap`.

### Option 2: npm (if you already use Node.js)

```bash
# Global install (online package; downloads the binary from GitHub Releases)
npm install -g @atomlai/centag

# Or try without changing global npm paths
npx --yes @atomlai/centag

# Offline / air-gapped package
npm install -g @atomlai/centag-offline
```

If `npm install -g` hits a permission error, use `npx` or the script above. Details: [apps/centag-npm/README.md](apps/centag-npm/README.md).

### Option 3: Docker (from source)

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # edit secrets as needed
./start.sh docker up                                 # default: personal container
```

Admin UI is still http://localhost:20060. Stop with `./start.sh docker down`.

---

## After install: connect an Agent

Goal: keep using your Agent as usual, while traffic goes through Centag (shared backends, failover, metering).

1. **Open the Web UI** → add and enable at least one backend (API key or local compatible endpoint).
2. **Agent Setup** (Web menu) — wizard to generate/write configs for common tools; or
3. **Process proxy (recommended — minimal Agent config changes)**:

```bash
# With Centag already running locally, launch an Agent via wrap
centag wrap run -- opencode
# Replace opencode with your Agent launch command

# Health check
centag wrap doctor
```

Note: `centag wrap` does **not** start the gateway; it only routes the Agent process into a running Centag. Full guide: [system proxy egress](docs/guide/system-proxy-egress.md).

---

## Why Centag?

| What you need | What Centag does |
|---------------|------------------|
| **Switch backends quickly** | Manage many backends in one place; enable/switch in the Web UI without rewiring every Agent |
| **Auto failover + API key pools** | Rotate multiple keys; fail over when a key is rate-limited or down |
| **Pipelines for each scenario** | Configurable modes (passthrough, direct, scheduling, review, …); change scenario = change policy |
| **Usage & billing metrics** | Track tokens/cost so personal usage stays visible |

In short: **one gateway for backends and policies; Agents just write code.**

## Capabilities

1. **Backends / models + API key pools**  
   Configure backends and models in the Web UI; pool and rotate **multiple API keys** per backend when limited or failing.

2. **Visual pipeline editor**  
   Customize proxy behavior on a canvas (forward, schedule, review, …); switch policies by scenario without changing Agent code.

3. **`centag wrap` — non-invasive third-party Agents**  
   Launch Agents with wrap and import traffic into Centag **without changing the Agent’s own settings**.

4. **Direct Agent config file setup**  
   Point the Agent’s API Base / Key at Centag like a normal LLM gateway (the Web “Agent Setup” wizard can help write configs).

Pick either path: wrap for fewer config edits, or config files for a standard OpenAI-compatible endpoint.

## Screenshots

| Dashboard | Agent Setup |
|-----------|-------------|
| ![Dashboard](docs/assets/readme/dashboard.png) | ![Agent Setup](docs/assets/readme/agent-setup.png) |

## Documentation

- [Docs index](docs/README.md)
- [Environment variables](docs/guide/environment-variables.md)
- [Local proxy / wrap](docs/guide/system-proxy-egress.md)
- [API reference](docs/api/API_REFERENCE.md)

## Feedback & support

Questions or suggestions: open a [GitHub Issue](https://github.com/atoml-ai/centag/issues), or email **centag@atoml.com**.

## License

MIT License (open-source editions: `minimal` / `personal`)

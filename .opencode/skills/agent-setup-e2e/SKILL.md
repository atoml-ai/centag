---
name: agent-setup-e2e
description: Use ONLY when verifying the Agent Setup wizard's three connection modes (direct, proxy+pipeline, proxy+backend) end-to-end through the LLM proxy and the generated/written agent config file. Covers generate/write APIs, real proxy requests with header assertions, and log analysis. Load this skill before running or documenting E2E acceptance tests for agent setup routing.
---

# Agent Setup — E2E Test Cases

Standard acceptance test for the Agent Setup wizard (`/api/v1/agent/configs/*`).
Verifies that the three connection modes route correctly through the proxy and/or
produce a correct standalone config file.

## Prerequisites

- Dev server running (`./start.sh debug personal`), default port `:20060`.
- Admin token: `POST /api/auth/login` (NOT `/api/v1/auth/login`) with
  `{"username":"admin","password":"<LLM_PROXY_ADMIN_PASSWORD from config/secrets/.env>"}`.
  Save body `data.token` to `/tmp/ctoken`.
- Proxy API key: id=1 value `llmproxy_...` (from `GET /api/v1/keys` or `/tmp/...`).
- A real backed backend, e.g. `opencode-zen`, with a usable model (e.g. `claude-fable-5`).
- Snapshot the system proxy config first and restore it after:
  `GET /api/v1/config/proxy` → `/tmp/proxy-orig.json` (send the flat `data` object back on PUT, NOT the wrapped one).

## Test Case 1 — Direct mode (standalone config file)

Model is NOT routed through the proxy; the wizard emits a real provider config.

```
POST /api/v1/agent/configs/generate
{"agent_type":"opencode","backend_id":"opencode-zen","model":"claude-fable-5"}
POST /api/v1/agent/configs/write   (same body)
```

Assertions on `~/.config/opencode/opencode.json` (or the agent_type's path):
- `provider.*.options.baseURL` == real backend base URL (e.g. `https://opencode.ai/zen/v1`) — NOT the centag proxy address.
- `provider.*.options.apiKey` == real backend key (`sk-...`) — NOT an `llmproxy_` key.
- `provider.*.models.<model>` exists and `model` == requested model.
- No `centag` proxy host, no `llmproxy_` key anywhere.

## Test Case 2 — Proxy + Pipeline mode (via_proxy=false, pipeline selected)

Request is forwarded through the proxy's transparent pipeline; backend/model come from the
pipeline / system default.

```
POST /v1/chat/completions  (Authorization: Bearer <llmproxy_key>)
{"model":"centag/<pipeline>","messages":[...]}
```

Assertions (response headers):
- `X-Proxy-Mode` == `transparent-proxy` (or the pipeline's mode).
- `X-Backend-Id` == the pipeline's configured backend (e.g. `opencode-zen`).
- `X-Executor-Model` == the resolved model (system default model if client model unspecified).

## Test Case 3 — Proxy + Backend mode (`backend/model`, via_proxy with backend pin)

Model string `backendID/modelID` (non-`centag/` prefix) pins the backend deterministically.

```
POST /v1/chat/completions  (Authorization: Bearer <llmproxy_key>)
{"model":"opencode-zen/claude-fable-5","messages":[...]}
```

Assertions (response headers):
- `X-Proxy-Mode` == `transparent`.
- `X-Backend-Id` == `opencode-zen` (pinned, regardless of system default).
- `X-Executor-Model` == `claude-fable-5` (rewritten from `opencode-zen/claude-fable-5`).

Known data quirk: a backend's stored model mapping may alias a requested model to a
different actual model (observed: `opencode-zen` maps `claude-opus-5` → `deepseek-v4-flash-free`).
This is pre-existing backend data, NOT a routing bug — prove it by sending the bare model
`{"model":"claude-opus-5"}` (plain-model-direct path hits the same alias). Use a model whose
mapping resolves to itself (e.g. `claude-fable-5`) for clean assertions.

## Log analysis

`tail -F /tmp/centag-debug.log` (or the dev server log). Grep for:

```
grep -E "\[Config\] (backend-pinned|pipeline) model|X-Proxy-Mode|X-Backend-Id|X-Executor-Model|transparent" /tmp/centag-debug.log
```

- backend-pinned path logs `actual_model` = the rewritten `modelID` (confirms rewrite ran).
- If `X-Executor-Model` differs from the rewritten model, check the backend's `supported_models`
  mapping (data quirk), not the routing code.

## Cleanup (mandatory)

- Restore system proxy config: `PUT /api/v1/config/proxy` with the flat `data` object from `/tmp/proxy-orig.json`.
- Restore any agent config file that was overwritten by `WriteConfig` (back it up before writing).

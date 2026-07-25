# Prompt Strategy Guide

This guide explains the Prompt Strategy framework in Centag, covering system prompt handling, user prompt operations, and output post-processing.

## Overview

The Prompt Strategy framework provides configurable prompt processing across three stages:

1. **System Prompt Strategy** - Controls how system prompts are handled
2. **User Prompt Operations** - Checks and optimizes user input
3. **Output Post Operations** - Post-processes model output

## System Prompt Strategy

### Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `passthrough` | Pass through client system prompt unchanged | Transparent proxy |
| `append` | Keep client system, append gateway text | Add gateway-specific instructions |
| `replace` | Replace client system with gateway text | Enforce specific system prompt |

### Configuration

```yaml
# Node config
custom_config:
  system_prompt_strategy: passthrough | append | replace
  append_position: after_client  # or before_client, merge_last
```

### Backward Compatibility

- `inject_system_prompt: true` maps to `replace` mode
- `inject_system_prompt: false` maps to `passthrough` mode
- When both `system_prompt_strategy` and `inject_system_prompt` are set, strategy takes precedence

## User Prompt Operations

The `user_prompt_ops` node provides inbound prompt checking and optimization.

### Features

- **Check**: Pattern-based deny rules, secret key heuristics
- **Optimize**: Whitespace collapse, max length truncation

### Configuration

```yaml
- id: user_guard
  type: user_prompt_ops
  config:
    custom_config:
      check:
        enabled: true
        deny_patterns:
          - '(?i)sk-[a-z0-9]{20,}'
          - '(?i)password'
        on_hit: log | redact | block
      optimize:
        enabled: true
        max_user_chars: 32000
        collapse_whitespace: true
```

### Actions

- `log`: Log the hit, pass through content
- `redact`: Log and replace content with `[REDACTED]`
- `block`: Return error `prompt_strategy_blocked`, stop pipeline

## Output Post Operations

The `output_post_ops` node provides string-level output normalization.

### Operations

| Op | Description |
|----|-------------|
| `trim_space` | Trim leading/trailing whitespace |
| `strip_markdown_fence` | Remove markdown code fences (```json ... ```) |
| `extract_json` | Extract JSON object from text |
| `json_compact` | Compact JSON (remove whitespace) |

### Configuration

```yaml
- id: out_normalize
  type: output_post_ops
  config:
    custom_config:
      ops:
        - trim_space
        - strip_markdown_fence
        - extract_json
        - json_compact
      on_invalid_json: pass | wrap_error_object
      stream_mode: skip | buffer   # skip when metadata.stream=true (default skip)
      max_buffer_bytes: 0          # 0 = unlimited; overflow fail-open
```

## Compatibility

| Legacy field | Maps to |
|--------------|---------|
| `inject_system_prompt: false` / unset | `system_prompt_strategy: passthrough` |
| `inject_system_prompt: true` | `system_prompt_strategy: replace` |

When both are set, **`system_prompt_strategy` wins**.

## Node Types

| Type | Description |
|------|-------------|
| `user_prompt_ops` | Inbound user prompt checking and optimization |
| `output_post_ops` | Output post-processing |

## Pipeline Integration

```mermaid
flowchart LR
  Client[Client] --> U[user_prompt_ops]
  U --> S[System Prompt Strategy]
  S --> LLM[LLM Backend]
  LLM --> O[output_post_ops]
  O --> ClientOut[Response]
```

## Related Documentation

- [Proxy Modes](proxy-modes.md)
- [Mode Behavior Matrix](mode-behavior-matrix.md)
- [Pipeline Variables](pipeline-variables.md)

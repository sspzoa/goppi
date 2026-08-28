# Configuration

goppi merges, in order: built-in defaults, `~/.config/goppi/config.json`, `.goppi.json` in the cwd, then environment variables. Flags passed to `goppi` win last. `goppi inspect` prints what won.

[Defaults](#defaults) · [Files](#files) · [Environment](#environment) · [Models](#models) · [Project instructions](#project-instructions)

## Defaults

| Key | Default |
|-----|---------|
| `model` | `solar-pro4` |
| `reasoning_effort` | `medium` (`solar-mini` omits the field) |
| `max_turns` | `30` |
| `max_tokens` | `32768` |
| `base_url` | `https://api.upstage.ai/v1` |
| `workdir` | current directory (stored as an absolute path) |

`solar-pro4` reasons unless effort is `none` or `minimal`. goppi still sends `medium` by default so reasoning stays on with tools + stream — omitting the field often skips the trace.

## Files

User config (optional), `~/.config/goppi/config.json`:

```json
{
  "model": "solar-pro4",
  "reasoning_effort": "medium",
  "max_turns": 30,
  "max_tokens": 32768
}
```

See [`config.example.json`](../../config.example.json). Project override: `.goppi.json` in the working directory (same schema). Later files overwrite earlier keys.

Supported keys: `api_key`, `base_url`, `model`, `reasoning_effort`, `max_turns`, `workdir`, `max_tokens`, `prompt_cache_key`, `always_approve`.

Credentials from `goppi login` live in a separate file, `~/.config/goppi/credentials.json` (`0600`). Do not put keys in a committed `.goppi.json`.

Sessions: `~/.local/share/goppi/sessions/<id>.json` and a `last` pointer. Override the data root with `GOPPI_DATA_DIR`. See [Sessions](sessions.md).

## Environment

| Variable | Action |
|----------|--------|
| `UPSTAGE_API_KEY` | API key |
| `GOPPI_API_KEY` | API key (fallback) |
| `GOPPI_MODEL` / `UPSTAGE_MODEL` | Model (`UPSTAGE_MODEL` wins if both are set) |
| `GOPPI_EFFORT` / `UPSTAGE_REASONING_EFFORT` | Reasoning effort |
| `GOPPI_BASE_URL` | API base (no trailing slash required) |
| `GOPPI_WORKDIR` | Working directory |
| `GOPPI_ALWAYS_APPROVE` | `1` skips write/bash prompts |
| `GOPPI_DATA_DIR` | Session store root |
| `GOPPI_TUI` | `0` forces the line REPL |
| `NO_COLOR` | Disable ANSI in the line UI |

Key resolution is documented in [Authentication](authentication.md#resolution-order).

## Models

| Model | Notes |
|-------|-------|
| `solar-pro4` | Default. 512K context. Reasons unless turned off |
| `solar-pro3` | Previous flagship. Reasoning on `medium` / `high` |
| `solar-pro2` | Older. Reasoning tokens, no visible trace |
| `solar-mini` | Fast. `reasoning_effort` is cleared and not sent |

Switch in the TUI with `/model`, or `-m` / `GOPPI_MODEL`. `goppi models` lists the catalog with the current pick marked.

Effort values: `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`. Unknown values fail at normalize time.

Chat Completions is used with stream, `reasoning_effort`, tools, `parallel_tool_calls`, `prompt_cache_key`, and `max_tokens`. Sampling (`temperature`, `top_p`) and `response_format` stay at API defaults. See [Generate](https://console.upstage.ai/docs/capabilities/generate).

## Project instructions

On every turn the agent reads, in order, from the workdir:

1. `GOPPI.md`
2. `AGENTS.md`
3. `.goppi/instructions.md`

Bodies are concatenated and appended under `Project instructions:` in the system prompt. Empty files are skipped.

`goppi init` writes a `GOPPI.md` stub and is a no-op if the file already exists. Keep it short: what the repo is, how to build and test, what not to touch. Long files cost tokens on every turn.

# Configuration

goppi는 다음 순서로 합칩니다. 내장 default, `~/.config/goppi/config.json`,
cwd의 `.goppi.json`, 그다음 env. `goppi`에 넘긴 flag가 마지막에 이깁니다.
`goppi inspect`가 이긴 값을 보여 줍니다. 파일에 모르는 키가 있으면 Load가
실패합니다 (`alwaysApprove`처럼 오타가 무시되지 않습니다).

[Defaults](#defaults) ·
[Files](#files) ·
[Environment](#environment) ·
[Models](#models) ·
[Project instructions](#project-instructions)

## Defaults

| Key | Default |
|-----|---------|
| `provider` | `upstage` (`openai` / `compat` 는 OpenAI-compatible Chat Completions) |
| `mode` | `act` (`plan`이면 쓰기/bash 거부) |
| `model` | `solar-pro4` |
| `reasoning_effort` | `medium` (`solar-mini`는 field를 생략) |
| `max_turns` | `30` (상한 80) |
| `max_tokens` | `32768` (상한 131072) |
| `auto_compact` | `true` (컨텍스트가 차면 요약한 뒤 이어서 진행) |
| `compact_at` | `100000` input tokens (하한 8000, 상한 500000) |
| `base_url` | `https://api.upstage.ai/v1` |
| `workdir` | 현재 디렉터리 (절대 경로로 저장). 있는 디렉터리여야 하고 `/`·볼륨 루트는 거절 |
| `sandbox` | `workspace` (`strict`면 네트워크도 차단, `off`면 가둠 없음) |
| `worktree` | `false` (`true`면 git worktree에서 실행) |

`solar-pro4`는 effort가 `none` 또는 `minimal`이 아니면 reasoning을 합니다.
Tool+stream에서 field를 빼면 추적이 자주 빠져서, goppi는 기본으로 `medium`을
보냅니다.

## Files

User config(선택), `~/.config/goppi/config.json`:

```json
{
  "model": "solar-pro4",
  "reasoning_effort": "medium",
  "max_turns": 30,
  "max_tokens": 32768,
  "auto_compact": true,
  "compact_at": 100000
}
```

[`config.example.json`](../../config.example.json)을 참고하세요. 프로젝트
override는 workdir의 `.goppi.json`입니다. user config 다음, env 전에 합칩니다.

지원 key: `provider`, `mode`, `base_url`, `model`, `reasoning_effort`,
`max_turns`, `workdir`, `max_tokens`, `auto_compact`, `compact_at`,
`prompt_cache_key`.

`api_key`, `always_approve`, `sandbox`, `mcp_servers`, `lsp_servers`,
`worktree`, `hooks`는 프로젝트 파일에서 무시합니다. 레포에 커밋된 JSON으로
확인을 끄거나 key를 심거나 sandbox를 끄거나 MCP/LSP 프로세스를 켜거나 hook
명령을 넣을 수 없게 하기 위해서입니다. 그 값은 user config
(`~/.config/goppi/config.json`), env, 또는 CLI(`--always-approve`,
`--sandbox`, `goppi login`)만 씁니다.

MCP는 user config에만 둡니다. 서버는 최대 8개, 서버당 tool 32개. child env에서
API key/token은 빼고, `env`로 명시한 값만 다시 넣습니다.

```json
{
  "mcp_servers": {
    "fs": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]
    }
  }
}
```

`goppi mcp`가 이름을 나열하고, `goppi mcp tools`가 프로세스를 켠 뒤 tool을
보여 줍니다. `inspect` JSON의 `mcp_servers`와 `lsp_servers`는 이름만입니다.
ACP `session/new` · `session/load`의 stdio `mcpServers`는 이 목록에 더해집니다.

LSP는 user config `lsp_servers`로 켭니다. 비어 있고 workdir에 `go.mod`가
있으면 `gopls`를 자동으로 찾습니다. `GOPPI_LSP=off`면 둘 다 안 켭니다. 서버는
최대 4개. bash·MCP와 같은 sandbox로 켭니다.

Hooks는 user config에만 둡니다. 이벤트당 최대 8개. stdin에 JSON(`event`,
`name`, `input`, `workdir`, `reason`…)을 넣고 `bash -lc`로 15초 안에 돕니다.
bash와 같은 sandbox와 `.git` 되돌림을 씁니다. timeout·취소는 process group을
죽여 자식도 같이 끊습니다. API key는 child env에서 뺍니다. `pre_tool`이 0이
아니면 그 tool은 실행되지 않습니다. `matcher`는 tool 이름(`bash`, `mcp_*`).

```json
{
  "hooks": {
    "pre_tool": [{"matcher": "bash", "command": "exit 0"}],
    "post_tool": [{"command": "echo done"}],
    "session_start": [{"command": "echo start"}],
    "session_end": [{"command": "echo end"}]
  }
}
```

`goppi login` credentials는 별도 파일
`~/.config/goppi/credentials.json`(`0600`)입니다.

Session: `~/.local/share/goppi/sessions/<id>.json`과 `last` pointer. Data
root는 `GOPPI_DATA_DIR`로 바꿉니다. [Sessions](sessions.md)를 보세요.

## Environment

| Variable | Action |
|----------|--------|
| `UPSTAGE_API_KEY` | API key |
| `OPENAI_API_KEY` | API key (openai / compat) |
| `GOPPI_PROVIDER` | `upstage` · `openai` · `compat` |
| `GOPPI_MODE` | `act` · `plan` |
| `GOPPI_API_KEY` | API key (대체) |
| `GOPPI_MODEL` / `UPSTAGE_MODEL` | model (둘 다 있으면 `UPSTAGE_MODEL`) |
| `GOPPI_EFFORT` / `UPSTAGE_REASONING_EFFORT` | reasoning effort |
| `GOPPI_BASE_URL` | API base (끝 slash 없어도 됨) |
| `GOPPI_WORKDIR` | workdir |
| `GOPPI_ALWAYS_APPROVE` | `1` / `true` / `yes` / `on`이면 쓰기/bash 확인 생략 |
| `GOPPI_SANDBOX` | `workspace` · `strict` · `off` |
| `GOPPI_LSP` | `off` / `0`이면 언어 서버를 켜지 않음 |
| `GOPPI_WORKTREE` | `1` / `true` / `yes` / `on`이면 git worktree 격리 |
| `GOPPI_AUTO_COMPACT` | `off` / `0` / `false`면 자동 compact 끔 |
| `GOPPI_COMPACT_AT` | 자동 compact input token 임계값 |
| `GOPPI_DATA_DIR` | session 저장 root |
| `GOPPI_TUI` | `0`이면 line REPL |
| `GOPPI_NOTIFY` | 기본은 TTY만 BEL. `off`로 끄고 `on`으로 강제 |
| `NO_COLOR` | line UI에서 ANSI 끄기 |

Key 해석은 [Authentication](authentication.md#resolution-order)에 있습니다.

## Models

| Model | Notes |
|-------|-------|
| `solar-pro4` | 기본. 512K context. 끄지 않으면 reasoning |
| `solar-pro3` | 이전 flagship. reasoning은 `medium` / `high` |
| `solar-pro2` | 이전 세대. reasoning token은 있으나 보이는 추적 없음 |
| `solar-mini` | 빠름. `reasoning_effort`는 지우고 보내지 않음 |

TUI에서는 `/model`, 아니면 `-m` / `GOPPI_MODEL`. `goppi models`가 목록과
현재 선택을 보여 줍니다.

Effort 값: `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`.
모르는 값은 normalize에서 실패합니다.

Chat Completions는 stream, `reasoning_effort`, tool, `parallel_tool_calls`,
`prompt_cache_key`, `max_tokens`를 씁니다. Sampling(`temperature`, `top_p`)과
`response_format`은 API default입니다. 현재 backend:
[Generate](https://console.upstage.ai/docs/capabilities/generate).

## Project instructions

매 turn workdir에서 이 순서로 읽습니다.

1. `GOPPI.md`
2. `AGENTS.md`
3. `.goppi/instructions.md`

본문을 이어 붙여 system prompt의 `프로젝트 지시:` 아래에 넣습니다. 빈 파일은
건너뜁니다. 합이 32 KiB를 넘으면 자릅니다.

`goppi init`은 `GOPPI.md` stub을 쓰고, 이미 있으면 아무것도 하지 않습니다.
짧게: 이 레포가 무엇인지, 빌드·test 방법, 건드리지 말 것. 긴 파일은 매 turn
token을 씁니다.

# Headless

No TTY takeover. Use this in scripts, CI, and other programs.

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi "테스트 한 개만 돌려"
goppi -p "요약해" --output-format json
goppi --always-approve -p "go test ./..."
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

A positional string is the same as `-p`. The session is still saved under `~/.local/share/goppi/sessions/` so `-c` / `-r` work afterwards.

[JSON](#json) · [Permissions](#permissions) · [Signals](#signals) · [CI](#ci)

## JSON

`--output-format json` prints one object on stdout when the turn finishes. The agent stays quiet (no TUI, no stream printer).

```json
{
  "text": "...",
  "reasoning": "...",
  "usage": {
    "InputTokens": 0,
    "OutputTokens": 0,
    "ReasoningTokens": 0
  },
  "session_id": "..."
}
```

`text` / `reasoning` come from the last assistant message. Progress and tool logs go nowhere — parse stdout as JSON.

`--output-format` must be `plain` or `json`. Anything else is an error.

## Permissions

Dangerous tools (`bash`, `write_file`, `edit_file`) are denied unless `--always-approve` (or `--yolo` / `GOPPI_ALWAYS_APPROVE=1`). That is intentional: a piped CI job should not hang on `allow? [y/N]`.

Read-only tools still run.

## Signals

`ctrl+c` / SIGINT cancels the in-flight HTTP request. Headless `-p` wraps the run in `signal.NotifyContext`. The session is saved when the turn finishes normally; a cancel mid-request may leave the last turn unsaved.

Interactive TUI does **not** use that signal context — see [TUI](tui.md#keyboard).

## CI

```yaml
- name: goppi
  env:
    UPSTAGE_API_KEY: ${{ secrets.UPSTAGE_API_KEY }}
  run: |
    goppi --always-approve --output-format json -p "go test ./... 가 어디서 도는지 찾고 한 패키지만 돌려"
```

Pin the model if the default must stay stable: `-m solar-pro4 --effort medium`.

`goppi complete models` and `goppi complete sessions` print one name per line. The generated zsh/bash/fish scripts call these for dynamic values; you can use the same in your own wrappers.

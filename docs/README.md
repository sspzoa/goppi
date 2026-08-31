# Documentation

**고삐** (`goppi`)는 풀스크린 TUI, script용 one-shot CLI, alt-screen을 끈
line REPL로 돌아갑니다.

기본 backend는 Upstage Solar입니다. `--provider openai` 또는 `compat`이면
OpenAI-compatible Chat Completions도 씁니다.

[Getting started](user-guide/getting-started.md) ·
[Authentication](user-guide/authentication.md) ·
[TUI](user-guide/tui.md) ·
[CLI](user-guide/cli.md) ·
[Configuration](user-guide/configuration.md) ·
[Sessions](user-guide/sessions.md) ·
[Tools](user-guide/tools.md) ·
[Headless](user-guide/headless.md) ·
[Development](development.md) ·
[Security](../SECURITY.md)

## User guide

| Page | Contents |
|------|----------|
| [Getting started](user-guide/getting-started.md) | 설치, 첫 session, 첫 prompt |
| [Authentication](user-guide/authentication.md) | API key 저장, env, `login` / `logout` |
| [TUI](user-guide/tui.md) | key, slash command, panel, stream |
| [CLI](user-guide/cli.md) | command, flag, shell completion |
| [Configuration](user-guide/configuration.md) | file, env, model, effort |
| [Sessions](user-guide/sessions.md) | resume, export, 저장 위치 |
| [Tools](user-guide/tools.md) | 파일, bash, 문서, MCP, LSP, delegate, 권한 |
| [Headless](user-guide/headless.md) | `-p`, JSON, CI |

## Project

| Page | Contents |
|------|----------|
| [Development](development.md) | 빌드, test, 레이아웃 |
| [Security](../SECURITY.md) | workdir 가둠, bash 한계, API key |
| [Changelog](../CHANGELOG.md) | 릴리스별 변경 |
| [License](../LICENSE) | AGPL-3.0 |

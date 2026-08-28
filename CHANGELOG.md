# Changelog

## 0.5.1 — 2026-08-28

- TUI: `tab` / `shift+tab` complete slash commands, models, and effort
- Shell: `goppi completions` covers flags, models, sessions (zsh/bash/fish)

## 0.5.0 — 2026-08-28

Fullscreen TUI in the style of a shipping coding-agent CLI.

- Alt-screen chat with mouse scroll, streaming reasoning, and tool cards
- Permission, model, and effort overlays
- Multiline input (`ctrl+j`), prompt history, slash commands
- `GOPPI_TUI=0` keeps the old line REPL

## 0.4.0 — 2026-08-28

Product surface aligned with a shipping coding-agent CLI.

- `login` / `logout` store an Upstage API key in `~/.config/goppi/credentials.json`
- `models`, `doctor`, `inspect`, `init`, `version`
- Named sessions: `sessions list|delete`, `-c` / `-r`, `export`
- Project instructions from `GOPPI.md` / `AGENTS.md`
- Headless `-p`, `--output-format json`, `--always-approve`
- Permission prompts for `bash`, `write_file`, `edit_file`
- Default `reasoning_effort=medium` so Solar actually thinks with tools

## 0.3.0 — 2026-08-28

- Stream `delta.reasoning` and `delta.content` over SSE
- Upstage-branded terminal chrome

## 0.2.0 — 2026-08-28

- Upstage Solar only (`solar-pro4` default)
- Document Parse / OCR tools

## 0.1.0 — 2026-08-28

- First local agent loop

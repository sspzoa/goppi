# GOPPI.md

goppi is an Upstage Solar coding-agent harness.

## Build

```
make build
make test
./bin/goppi
```

## Layout

- `cmd/goppi` — CLI commands
- `internal/upstage` — Solar chat + Document Parse/OCR
- `internal/agent` — tool loop
- `internal/tools` — files, bash, documents
- `internal/tui` — fullscreen TUI (Charm Bubble Tea v2)
- `internal/complete` — slash + shell autocomplete

Agent, tools, and the Upstage client stay stdlib. The interactive TUI is Charm.

## Notes

- Default model is `solar-pro4` with `reasoning_effort=medium`.
- Do not commit `credentials.json` or API keys.
- `GOPPI_TUI=0` forces the line REPL instead of the fullscreen TUI.
- User-facing docs live in `docs/`. Keep GOPPI.md short — it is loaded on every turn.

# Development

```bash
make build    # bin/goppi
make test     # go test ./...
make fmt
./bin/goppi
```

Go 1.27. Interactive TUI uses Charm Bubble Tea v2 (`charm.land/bubbletea/v2`). Agent, tools, and the Upstage client stay in the standard library — do not pull extra deps into those packages.

[Layout](#layout) · [Tests](#tests) · [Local TUI](#local-tui) · [Screenshots](#screenshots) · [License](#license)

## Layout

| Path | Contents |
|------|----------|
| `cmd/goppi` | CLI entry: run, login, doctor, sessions, completions |
| `internal/upstage` | Solar Chat + Document Parse / OCR HTTP |
| `internal/provider` | Chat request/response, SSE |
| `internal/agent` | Tool loop and event sink |
| `internal/tui` | Fullscreen chat |
| `internal/repl` | TTY → TUI, else line loop; headless `-p` |
| `internal/tools` | Files, bash, documents |
| `internal/session` | Named transcripts |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | Slash + shell autocomplete |
| `internal/ui` | Line-mode printer, colors |
| `internal/config` | Defaults, env, credentials |

The agent talks to the UI only through `agent.Sink`. The TUI implements that sink; headless JSON sets `Quiet` and skips the stream printer.

## Tests

```bash
go test ./...
go vet ./...
```

CI (`.github/workflows/ci.yml`) runs tests, `go build`, `goppi version`, and `goppi help` on Go 1.27.

## Local TUI

```bash
GOPPI_TUI=0 ./bin/goppi          # line REPL, no alt screen
./bin/goppi                      # fullscreen (needs a TTY)
```

Brand colors: violet `#5B52FF`, ink `#0A0D14`. Keep new chrome on those tokens.

## Screenshots

The README TUI image is a capture of `docs/assets/tui-preview.html`, not a live Solar session.

```bash
python3 -m http.server 8765 --directory docs/assets
# open http://127.0.0.1:8765/tui-preview.html and capture the .frame
```

Wordmark source: `docs/assets/goppi-wordmark.svg`. Raster export: `goppi-wordmark.png`. `wordmark-preview.html` is a tight crop if you recapture it.

## License

AGPL-3.0. See [LICENSE](../LICENSE).

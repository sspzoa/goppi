# GOPPI.md

goppi is an Upstage Solar coding-agent harness. Stdlib Go only.

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

## Notes

- Default model is `solar-pro4` with `reasoning_effort=medium`.
- Do not commit `credentials.json` or API keys.

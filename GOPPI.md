# GOPPI.md

고삐는 한국형 에이전트 하네스입니다. 바이너리 이름은 `goppi`. 현재 backend는 Upstage Solar입니다.

## 빌드

```
make build
make test
./bin/goppi
```

## 레이아웃

- `cmd/goppi` — CLI command
- `internal/upstage` — 현재 chat · document HTTP client
- `internal/agent` — tool loop
- `internal/tools` — 파일, bash, 문서
- `internal/tui` — 풀스크린 TUI (Charm Bubble Tea v2)
- `internal/complete` — slash · shell completion

Agent, tool, HTTP client는 stdlib. Interactive TUI는 Charm.

## 메모

- 기본 model은 `solar-pro4`, `reasoning_effort=medium`.
- `credentials.json`과 API key는 커밋하지 말 것.
- `GOPPI_TUI=0`이면 풀스크린 대신 line REPL.
- 사용자 문서는 `docs/`. GOPPI.md는 매 turn 읽히니 짧게 유지.

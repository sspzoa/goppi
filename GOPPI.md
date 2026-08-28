# GOPPI.md

고삐는 로컬 에이전트 하네스입니다. 바이너리 이름은 `goppi`. 현재 백엔드는 Upstage Solar입니다.

## 빌드

```
make build
make test
./bin/goppi
```

## 레이아웃

- `cmd/goppi` — CLI 커맨드
- `internal/upstage` — 현재 채팅·문서 HTTP 클라이언트
- `internal/agent` — 툴 루프
- `internal/tools` — 파일, bash, 문서
- `internal/tui` — 풀스크린 TUI (Charm Bubble Tea v2)
- `internal/complete` — 슬래시·셸 자동완성

에이전트, 툴, HTTP 클라이언트는 stdlib. 인터랙티브 TUI는 Charm.

## 메모

- 기본 모델은 `solar-pro4`, `reasoning_effort=medium`.
- `credentials.json`과 API 키는 커밋하지 말 것.
- `GOPPI_TUI=0`이면 풀스크린 대신 라인 REPL.
- 사용자 문서는 `docs/`. GOPPI.md는 매 턴 읽히니 짧게 유지.

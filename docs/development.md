# Development

```bash
make build    # bin/goppi
make test     # go test ./...
make fmt
./bin/goppi
```

Go 1.27. Interactive TUI는 Charm Bubble Tea v2 (`charm.land/bubbletea/v2`)입니다. Agent, tool, HTTP client는 표준 라이브러리에 둡니다. 그 패키지에 추가 의존성을 넣지 마세요.

현재 backend는 Upstage Solar입니다. 새 provider는 agent loop에 바로 넣지 말고 `internal/provider` 뒤에 두세요.

[레이아웃](#레이아웃) · [Test](#test) · [로컬 TUI](#로컬-tui) · [라이선스](#라이선스)

## 레이아웃

| 경로 | 내용 |
|------|------|
| `cmd/goppi` | CLI entry: run, login, doctor, sessions, completions |
| `internal/upstage` | 현재 chat · document HTTP client |
| `internal/provider` | chat request/response, SSE |
| `internal/agent` | tool loop와 event sink |
| `internal/tui` | 풀스크린 chat |
| `internal/repl` | TTY면 TUI, 아니면 line loop. Headless `-p` |
| `internal/tools` | 파일, bash, 문서 |
| `internal/session` | named transcript |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | slash · shell completion |
| `internal/ui` | line-mode printer, 색 |
| `internal/config` | default, env, credentials |

에이전트는 UI와 `agent.Sink`로만 이야기합니다. TUI가 그 sink를 구현하고, headless JSON은 `Quiet`로 stream printer를 끕니다.

## Test

```bash
go test ./...
go vet ./...
```

CI (`.github/workflows/ci.yml`)는 Go 1.27에서 test, `go build`, `goppi version`, `goppi help`를 돌립니다.

## 로컬 TUI

```bash
GOPPI_TUI=0 ./bin/goppi          # line REPL, alt-screen 없음
./bin/goppi                      # 풀스크린 (TTY 필요)
```

브랜드 색: 주홍 `#C23D2A`, 먹 `#161411`, 한지 `#F2EDE6`.

## 라이선스

AGPL-3.0. [LICENSE](../LICENSE)를 보세요.

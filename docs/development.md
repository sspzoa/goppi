# 개발

```bash
make build    # bin/goppi
make test     # go test ./...
make fmt
./bin/goppi
```

Go 1.27. 인터랙티브 TUI는 Charm Bubble Tea v2 (`charm.land/bubbletea/v2`)입니다. 에이전트, 툴, HTTP 클라이언트는 표준 라이브러리에 둡니다. 그 패키지에 추가 의존성을 넣지 마세요.

현재 백엔드는 Upstage Solar입니다. 새 프로바이더는 에이전트 루프에 바로 넣지 말고 `internal/provider` 뒤에 두세요.

[레이아웃](#레이아웃) · [테스트](#테스트) · [로컬 TUI](#로컬-tui) · [라이선스](#라이선스)

## 레이아웃

| 경로 | 내용 |
|------|------|
| `cmd/goppi` | CLI 엔트리: run, login, doctor, sessions, completions |
| `internal/upstage` | 현재 채팅·문서 HTTP 클라이언트 |
| `internal/provider` | 채팅 요청/응답, SSE |
| `internal/agent` | 툴 루프와 이벤트 싱크 |
| `internal/tui` | 풀스크린 채팅 |
| `internal/repl` | TTY면 TUI, 아니면 라인 루프. 헤드리스 `-p` |
| `internal/tools` | 파일, bash, 문서 |
| `internal/session` | 이름 있는 트랜스크립트 |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | 슬래시·셸 자동완성 |
| `internal/ui` | 라인 모드 프린터, 색 |
| `internal/config` | 기본값, env, 자격 증명 |

에이전트는 UI와 `agent.Sink`로만 이야기합니다. TUI가 그 싱크를 구현하고, 헤드리스 JSON은 `Quiet`로 스트림 프린터를 끕니다.

## 테스트

```bash
go test ./...
go vet ./...
```

CI (`.github/workflows/ci.yml`)는 Go 1.27에서 테스트, `go build`, `goppi version`, `goppi help`를 돌립니다.

## 로컬 TUI

```bash
GOPPI_TUI=0 ./bin/goppi          # 라인 REPL, 알트 스크린 없음
./bin/goppi                      # 풀스크린 (TTY 필요)
```

브랜드 색: 주홍 `#C23D2A`, 먹 `#161411`, 한지 `#F2EDE6`.

## 라이선스

AGPL-3.0. [LICENSE](../LICENSE)를 보세요.

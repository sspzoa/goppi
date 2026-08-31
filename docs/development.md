# Development

```bash
make build    # bin/goppi
make test     # go test -race ./...
make check    # fmt + vet + staticcheck + govulncheck + actionlint + test + windows + install.sh
make dist     # scripts/package.sh → dist/*.tar.gz + SHA256SUMS
make verify-dist  # dist + checksum + layout + native binary version + install.sh smoke (cosign path 포함)
make release-check  # check + verify-dist (tag 전 로컬 게이트)
make release-prepare  # release-check + ci-smoke (tag push 직전)
make pre-release    # release-prepare + live-check
make live-check     # UPSTAGE_API_KEY 있을 때 Solar smoke
make ci-smoke       # ci-cli-smoke + ci-headless-smoke
make ci-cli-smoke   # CI CLI smoke
make ci-headless-smoke  # CI headless JSON smoke
make fmt
./bin/goppi
```

Go 1.27. Interactive TUI는 Charm Bubble Tea v2 (`charm.land/bubbletea/v2`)입니다.
Agent, tool, HTTP client는 표준 라이브러리에 둡니다. 그 패키지에 추가 의존성을
넣지 마세요. API key는 `.env.example`을 참고해 shell env 또는 `goppi login`으로
둡니다 (`.env`는 자동 로드되지 않음).

`internal/config/config.go`의 `Version`이 단일 출처입니다. `make build`·`make dist`·
CI·release tag는 모두 이 값과 맞춥니다.

현재 backend는 Upstage Solar입니다. 새 provider는 agent loop에 바로 넣지 말고
`internal/provider` 뒤에 두세요.

[Layout](#layout) ·
[Tests](#tests) ·
[Local TUI](#local-tui) ·
[License](#license)

## Layout

| Path | Contents |
|------|----------|
| `cmd/goppi` | CLI entry: run, login, doctor, sessions, completions |
| `internal/upstage` | chat · document HTTP client |
| `internal/provider` | chat request/response, SSE |
| `internal/agent` | tool loop와 event sink |
| `internal/tui` | 풀스크린 chat |
| `internal/repl` | TTY면 TUI, 아니면 line loop. Headless `-p` |
| `internal/tools` | 파일, bash, 문서, MCP, LSP diagnostics, delegate |
| `internal/mcp` | stdio MCP 클라이언트 |
| `internal/lsp` | stdio LSP 클라이언트 (diagnostics) |
| `internal/worktree` | git worktree 격리 |
| `internal/acp` | Agent Client Protocol v1 stdio |
| `internal/session` | named transcript |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | slash · shell completion |
| `internal/ui` | line-mode printer, 색 |
| `internal/config` | default, env, credentials |

에이전트는 UI와 `agent.Sink`로만 이야기합니다. TUI가 그 sink를 구현하고,
headless JSON은 `Quiet`로 stream printer를 끕니다.

## Tests

```bash
go test ./...
go vet ./...
GOPPI_LIVE=1 go test ./internal/provider -run Live   # 실제 Solar, API key 필요
make live-check                                      # key 없으면 skip
```

CI (`.github/workflows/ci.yml`)는 ubuntu와 macOS, Go 1.27에서 `make release-prepare`
(`release-check` + `ci-smoke`)를 돌립니다. Job timeout은
30분입니다.
`v*` release job은 tag와 `config.Version` 일치를 확인한 뒤 `make release-prepare`
(`release-check` + `ci-smoke`)·cosign 서명·`cosign verify-blob` 자체 검증을 한 다음 GitHub
release에 올립니다. tag는 `v` + `config.Version`이어야 합니다 (예: `v0.143.0`).

## Proving a release

tag 전 로컬 게이트:

```bash
make pre-release       # release-prepare + live-check
make release-prepare   # release-check + ci-smoke (CI·release workflow와 동일)
# 또는 단계별:
make release-check
make ci-smoke
make live-check        # UPSTAGE_API_KEY 필요, 없으면 skip
```

## Production readiness

| 영역 | 로컬 증거 | push/tag·key 후 |
|------|-----------|-----------------|
| 테스트/검증 | `make check`, `go test -race ./...` | GitHub Actions green |
| CI/릴리스 | `make release-prepare`, actionlint | release workflow + cosign |
| 설치 | `make verify-dist`, install 20+ tests | `install.sh` from release URL |
| CLI/headless | `make ci-smoke`, cmd E2E, headless signal cancel, max-turns JSON error, export after headless smoke | — |
| TUI/CLI | `internal/tui` 39 tests, dispatch/workdir, completions | — |
| 세션·설정 | resume/export/init idempotency/session usage·locked resume reject cmd E2E | — |
| 에러·취소 | repl/tui/agent cancel tests, headless SIGINT/SIGTERM·시작 실패 JSON E2E | — |
| 툴 권한 | bash/write/edit/apply/MCP deny·plan·`--always-approve`/`--yolo`(write+MCP) E2E | — |
| 보안 | doctor symlink/perm·inspect/export/help redaction·project config ignore·login empty key·root workdir reject | real Sigstore bundle |
| Live API | `make live-check` (key 없으면 skip) | `UPSTAGE_API_KEY` smoke |

GitHub end-to-end (push/tag 후):

1. release workflow가 green인지 확인한다.
2. [SECURITY.md](../SECURITY.md)의 `cosign verify-blob`로 `SHA256SUMS`를 검증한다.
3. release URL에서 `install.sh` smoke (또는 `GOPPI_RELEASE_BASE`로 mirror 테스트).

## Local TUI

```bash
GOPPI_TUI=0 ./bin/goppi          # line REPL, alt-screen 없음
./bin/goppi                      # 풀스크린 (TTY 필요)
```

브랜드 색: 주홍 `#C23D2A`, 먹 `#161411`, 한지 `#F2EDE6`.

## License

AGPL-3.0. [LICENSE](../LICENSE)를 보세요.

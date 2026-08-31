# 고삐

**고삐** (`goppi`)는 터미널 코딩 에이전트입니다. 풀스크린 TUI로 트리를 읽고,
파일을 고치고, shell을 돌리고, 문서를 파싱합니다. script와 CI에서는
headless로, 에디터에서는 [Agent Client Protocol](https://agentclientprotocol.com/)
로 붙습니다.

API key와 session은 이 머신에 남고, 파일·shell·문서는 workdir(그리고 허용한
extra root) 안에서만 움직입니다.

[Install](#install) ·
[Build from source](#build-from-source) ·
[Documentation](#documentation) ·
[Layout](#layout) ·
[Development](#development) ·
[License](#license) ·
[Security](SECURITY.md)

기본 backend는 [Upstage Solar](https://console.upstage.ai/docs/capabilities/generate)
입니다 (`solar-pro4`, `reasoning_effort=medium`). `--provider openai` 또는
`compat`이면 OpenAI-compatible Chat Completions도 씁니다.

---

## Install

설치 스크립트는 GitHub release 바이너리를 받아 SHA256을 확인한 뒤
`~/.local/bin/goppi`에 둡니다. 맞는 release가 없으면
[Go 1.27+](https://go.dev/dl/)로 `go install`합니다.

```bash
curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
goppi version
goppi login
goppi
```

> [!NOTE]
> `~/.local/bin`이 `PATH`에 있어야 합니다. `GOPPI_INSTALL_DIR`로 경로를 바꾸고,
> `GOPPI_INSTALL_FROM=go`면 소스 설치만 합니다.

`SHA256SUMS.sigstore.json`이 있으면 `cosign`으로 검증합니다. 없으면 SHA256만
확인합니다. 미러(`GOPPI_RELEASE_BASE`)는 서명을 강제합니다
(`GOPPI_SKIP_COSIGN=1`은 테스트용).

`$(go env GOPATH)/bin`이 이미 `PATH`에 있으면:

```bash
go install github.com/sspzoa/goppi/cmd/goppi@latest
```

`goppi login`은 `~/.config/goppi/credentials.json`을 mode `0600`으로 씁니다.
로컬 key store이며 OAuth가 아니고 browser를 열지 않습니다.
`UPSTAGE_API_KEY` 또는 `GOPPI_API_KEY`를 보내도 됩니다.
[Authentication](docs/user-guide/authentication.md)을 보세요.

## Build from source

```bash
git clone https://github.com/sspzoa/goppi.git
cd goppi
make build          # bin/goppi
make check          # fmt + vet + staticcheck + govulncheck + test + windows
make test
./bin/goppi version
```

`make install`은 `go install ./cmd/goppi`입니다.

프로젝트 디렉터리에서 `goppi`를 켜세요. 처음 물어보기 좋은 것:

```text
이 레포 구조를 설명해줘
테스트가 어디서 도는지 찾고 한 개만 돌려
```

Headless:

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi -p "요약해" --output-format json
goppi --always-approve -p "go test ./..."
goppi --worktree -p "이 버그 고쳐"
goppi acp
```

`bash`, `write_file`, `edit_file`, `apply_patch`는 `--always-approve` /
`--yolo`가 없으면 실행 전에 묻습니다. JSON·비 TTY에서는 flag 없이 해당
tool을 거부합니다. `bash`는 기본으로 workdir·임시·캐시 밖 쓰기를 거부합니다
(`--sandbox off`로 해제, `--sandbox strict`면 네트워크도 차단).

## Documentation

유저 가이드는 [`docs/`](docs/README.md)에 있습니다.

| Page | Contents |
|------|----------|
| [Getting started](docs/user-guide/getting-started.md) | 설치, API key, 첫 TUI session |
| [Authentication](docs/user-guide/authentication.md) | `login` / `logout`, env |
| [TUI](docs/user-guide/tui.md) | key, slash command, panel |
| [CLI](docs/user-guide/cli.md) | command, flag, shell completion |
| [Configuration](docs/user-guide/configuration.md) | file, model, `GOPPI.md` |
| [Sessions](docs/user-guide/sessions.md) | `-c` / `-r`, export |
| [Tools](docs/user-guide/tools.md) | 파일, bash, 문서, MCP, LSP, delegate |
| [Headless](docs/user-guide/headless.md) | `-p`, JSON, CI |
| [Development](docs/development.md) | 빌드, test, 패키지 구조 |

Shell completion:

```bash
goppi completions zsh  > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

TUI 안에서는 `/` 다음에 `tab`으로 `/model`, `/effort`와 값을 완성합니다.

## Layout

| Path | Contents |
|------|----------|
| `cmd/goppi` | CLI entry: run, login, doctor, sessions, completions |
| `internal/upstage` | chat · document HTTP client |
| `internal/provider` | chat request/response, SSE |
| `internal/agent` | tool loop와 event sink |
| `internal/tui` | 풀스크린 chat (Charm Bubble Tea v2) |
| `internal/repl` | TTY면 TUI, 아니면 line loop. Headless `-p` |
| `internal/tools` | 파일, bash, 문서, MCP, LSP, delegate |
| `internal/mcp` | stdio MCP 클라이언트 |
| `internal/session` | named transcript |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | slash · shell completion |
| `internal/config` | default, env, credentials |

Agent, tool, HTTP client는 표준 라이브러리만 씁니다. Interactive TUI는
Charm입니다. `GOPPI_TUI=0`이면 line REPL로 떨어집니다.

## Development

```bash
make check          # fmt + vet + staticcheck + govulncheck + test + windows
make release-check  # check + verify-dist (tag 전 게이트)
make release-prepare  # release-check + ci-smoke (tag push 직전)
make pre-release      # release-prepare + live-check
make ci-smoke       # ci-cli-smoke + ci-headless-smoke
make test
go vet ./...
./bin/goppi
```

자세한 내용은 [`docs/development.md`](docs/development.md)를 보세요. Release tag는
`config.Version`과 맞춰 `v0.x.y`로 push합니다.

에이전트가 매 turn 읽는 프로젝트 지시: `GOPPI.md`, `AGENTS.md`,
`.goppi/instructions.md`. `goppi init`이 stub을 씁니다.

릴리스별 변경은 [CHANGELOG.md](CHANGELOG.md)를 보세요.

## License

[AGPL-3.0](LICENSE).

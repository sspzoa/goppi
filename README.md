# 고삐

한국형 에이전트 하네스.

독자 foundation model, sovereign AI가 모델을 말하는 자리에서 고삐는 harness를 말합니다. 에이전트에 고삐를 채웁니다. API key와 session은 이 머신에 남고, 파일·shell·문서는 이 workdir에서만 움직입니다.

풀스크린 TUI로 트리를 읽고, 파일을 고치고, shell을 돌리고, 문서를 파싱합니다. script와 CI에서는 headless입니다. 바이너리 이름은 `goppi`입니다.

현재 기본 backend는 [Upstage Solar](https://console.upstage.ai/docs/capabilities/generate)입니다 (`solar-pro4`, `reasoning_effort=medium`). 다른 provider는 아직 없습니다.

[설치](#설치) ·
[소스에서 빌드](#소스에서-빌드) ·
[문서](#문서) ·
[레이아웃](#레이아웃) ·
[개발](#개발) ·
[라이선스](#라이선스)

## 설치

[Go 1.27+](https://go.dev/dl/)가 필요합니다. 설치 스크립트는 `go install`입니다.

```bash
curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
goppi version
goppi login
goppi
```

`$(go env GOPATH)/bin`이 이미 `PATH`에 있으면:

```bash
go install github.com/sspzoa/goppi/cmd/goppi@latest
```

`goppi login`은 `~/.config/goppi/credentials.json`을 mode `0600`으로 씁니다. 로컬 key store이며 OAuth가 아니고 browser를 열지 않습니다. `UPSTAGE_API_KEY` 또는 `GOPPI_API_KEY`를 보내도 됩니다. [Authentication](docs/user-guide/authentication.md)을 보세요.

## 소스에서 빌드

```bash
git clone https://github.com/sspzoa/goppi.git
cd goppi
make build          # bin/goppi
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
```

`bash`, `write_file`, `edit_file`은 `--always-approve` / `--yolo`가 없으면 실행 전에 묻습니다. JSON·비 TTY에서는 flag 없이 해당 tool을 거부합니다.

## 문서

유저 가이드는 [`docs/`](docs/README.md)에 있습니다.

| 페이지 | 내용 |
|--------|------|
| [시작하기](docs/user-guide/getting-started.md) | 설치, API key, 첫 TUI session |
| [Authentication](docs/user-guide/authentication.md) | `login` / `logout`, env |
| [TUI](docs/user-guide/tui.md) | key, slash command, panel |
| [CLI](docs/user-guide/cli.md) | command, flag, shell completion |
| [Configuration](docs/user-guide/configuration.md) | file, model, `GOPPI.md` |
| [Sessions](docs/user-guide/sessions.md) | `-c` / `-r`, export |
| [Tools](docs/user-guide/tools.md) | 파일, bash, 문서 |
| [Headless](docs/user-guide/headless.md) | `-p`, JSON, CI |
| [Development](docs/development.md) | 빌드, test, 패키지 구조 |

Shell completion:

```bash
goppi completions zsh  > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

TUI 안에서는 `/` 다음에 `tab`으로 `/model`, `/effort`와 값을 완성합니다.

## 레이아웃

| 경로 | 내용 |
|------|------|
| `cmd/goppi` | CLI entry: run, login, doctor, sessions, completions |
| `internal/upstage` | 현재 chat · document HTTP client |
| `internal/provider` | chat request/response, SSE |
| `internal/agent` | tool loop와 event sink |
| `internal/tui` | 풀스크린 chat (Charm Bubble Tea v2) |
| `internal/repl` | TTY면 TUI, 아니면 line loop. Headless `-p` |
| `internal/tools` | 파일, bash, 문서 |
| `internal/session` | named transcript |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | slash · shell completion |
| `internal/config` | default, env, credentials |

Agent, tool, HTTP client는 표준 라이브러리만 씁니다. Interactive TUI는 Charm입니다. `GOPPI_TUI=0`이면 line REPL로 떨어집니다.

## 개발

```bash
make build
make test
go vet ./...
./bin/goppi
```

CI (`.github/workflows/ci.yml`)는 Go 1.27에서 test, `go build`, `goppi version`, `goppi help`를 돌립니다.

에이전트가 매 turn 읽는 프로젝트 지시: `GOPPI.md`, `AGENTS.md`, `.goppi/instructions.md`. `goppi init`이 stub을 씁니다.

릴리스별 변경은 [CHANGELOG.md](CHANGELOG.md)를 보세요.

## 라이선스

[AGPL-3.0](LICENSE).

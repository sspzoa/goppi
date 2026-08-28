# 고삐

에이전트에 고삐를 채우는 로컬 하네스입니다.

풀스크린 TUI로 작업 트리를 읽고, 파일을 고치고, 셸을 돌리고, 문서를 파싱합니다. 스크립트·CI에서는 헤드리스로 돌아갑니다. 바이너리 이름은 `goppi`입니다.

현재 기본 백엔드는 [Upstage Solar](https://console.upstage.ai/docs/capabilities/generate)입니다 (`solar-pro4`, `reasoning_effort=medium`). 다른 프로바이더는 아직 없습니다.

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

`goppi login`은 `~/.config/goppi/credentials.json`을 모드 `0600`으로 씁니다. 로컬 키 저장소이며 OAuth가 아니고 브라우저를 열지 않습니다. `UPSTAGE_API_KEY` 또는 `GOPPI_API_KEY`를 보내도 됩니다. [인증](docs/user-guide/authentication.md)을 보세요.

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

헤드리스:

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi -p "요약해" --output-format json
goppi --always-approve -p "go test ./..."
```

`bash`, `write_file`, `edit_file`은 `--always-approve` / `--yolo`가 없으면 실행 전에 묻습니다. JSON·비 TTY에서는 플래그 없이 해당 툴을 거부합니다.

## 문서

유저 가이드는 [`docs/`](docs/README.md)에 있습니다.

| 페이지 | 내용 |
|--------|------|
| [시작하기](docs/user-guide/getting-started.md) | 설치, API 키, 첫 TUI 세션 |
| [인증](docs/user-guide/authentication.md) | `login` / `logout`, 환경 변수 |
| [TUI](docs/user-guide/tui.md) | 키, 슬래시 명령, 패널 |
| [CLI](docs/user-guide/cli.md) | 커맨드, 플래그, 셸 자동완성 |
| [설정](docs/user-guide/configuration.md) | 파일, 모델, `GOPPI.md` |
| [세션](docs/user-guide/sessions.md) | `-c` / `-r`, 내보내기 |
| [툴](docs/user-guide/tools.md) | 파일, bash, 문서 |
| [헤드리스](docs/user-guide/headless.md) | `-p`, JSON, CI |
| [개발](docs/development.md) | 빌드, 테스트, 패키지 구조 |

셸 자동완성:

```bash
goppi completions zsh  > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

TUI 안에서는 `/` 다음에 `tab`으로 `/model`, `/effort`와 값을 완성합니다.

## 레이아웃

| 경로 | 내용 |
|------|------|
| `cmd/goppi` | CLI 엔트리: run, login, doctor, sessions, completions |
| `internal/upstage` | 현재 채팅·문서 HTTP 클라이언트 |
| `internal/provider` | 채팅 요청/응답, SSE |
| `internal/agent` | 툴 루프와 이벤트 싱크 |
| `internal/tui` | 풀스크린 채팅 (Charm Bubble Tea v2) |
| `internal/repl` | TTY면 TUI, 아니면 라인 루프. 헤드리스 `-p` |
| `internal/tools` | 파일, bash, 문서 |
| `internal/session` | 이름 있는 트랜스크립트 |
| `internal/instructions` | `GOPPI.md` / `AGENTS.md` |
| `internal/complete` | 슬래시·셸 자동완성 |
| `internal/config` | 기본값, env, 자격 증명 |

에이전트, 툴, HTTP 클라이언트는 표준 라이브러리만 씁니다. 인터랙티브 TUI는 Charm입니다. `GOPPI_TUI=0`이면 라인 REPL로 떨어집니다.

## 개발

```bash
make build
make test
go vet ./...
./bin/goppi
```

CI (`.github/workflows/ci.yml`)는 Go 1.27에서 테스트, `go build`, `goppi version`, `goppi help`를 돌립니다.

에이전트가 매 턴 읽는 프로젝트 지시: `GOPPI.md`, `AGENTS.md`, `.goppi/instructions.md`. `goppi init`이 스텁을 씁니다.

릴리스별 변경은 [CHANGELOG.md](CHANGELOG.md)를 보세요.

## 라이선스

[AGPL-3.0](LICENSE).

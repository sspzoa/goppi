# Getting started

**고삐** (`goppi`)는 workdir을 읽고, 파일을 고치고, shell을 돌리고, 문서를
파싱합니다. 풀스크린 TUI로 쓰거나 script에서 headless로 돌립니다.

기본 backend는 [Upstage Solar](https://console.upstage.ai/docs/capabilities/generate)입니다.

[Install](#install) ·
[API key](#api-key) ·
[First session](#first-session) ·
[Headless](#headless) ·
[Next](#next)

## Install

설치 스크립트는 최신 GitHub release 바이너리를 SHA256 확인 후 `~/.local/bin`에
둡니다. `SHA256SUMS.sigstore.json`이 있으면 `cosign` 검증이 필수입니다.
GitHub가 아닌 `GOPPI_RELEASE_BASE`는 서명 번들 자체도 있어야 합니다. 맞는
파일이 없으면 Go 1.27+로 `go install`합니다.

```bash
curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
goppi version
```

이 트리를 직접 빌드하려면:

```bash
git clone https://github.com/sspzoa/goppi.git
cd goppi
make build
./bin/goppi version
make pre-release      # tag 전: release-prepare + live-check
make release-prepare  # release-check + ci-smoke
```

`go install github.com/sspzoa/goppi/cmd/goppi@latest`는 설치 스크립트와
같습니다. `$(go env GOPATH)/bin`(보통 `~/go/bin`)이 `PATH`에 있어야 합니다.

## API key

[console.upstage.ai](https://console.upstage.ai)에서 key를 만든 뒤:

```bash
goppi login
```

또는:

```bash
export UPSTAGE_API_KEY=up_...
```

`goppi login`은 `~/.config/goppi/credentials.json`을 mode `0600`으로 씁니다.
로컬 key store이며 OAuth가 아니고 browser를 열지 않습니다.

머신은 `goppi doctor`로 확인하세요. 자세한 내용은
[Authentication](authentication.md)입니다.

## First session

```bash
cd your-project
goppi
```

TTY면 풀스크린 TUI가 열립니다. 처음 물어보기 좋은 것:

```text
이 레포 구조를 설명해줘
테스트가 어디서 도는지 찾고 한 개만 돌려
README를 현재 코드에 맞게 고쳐
```

PDF / HWP / Office 파일은 경로를 주면 `document_parse`로 넘깁니다.

나갈 때는 `/quit`. TUI는 `ctrl+c` 다음 `enter`. Line REPL(`GOPPI_TUI=0`)은
대기 prompt에서 `ctrl+c`면 저장하고 끝납니다.

프로젝트 규칙은 `GOPPI.md`에 두면 매 turn 읽습니다.

```bash
goppi init
```

[Project instructions](configuration.md#project-instructions)를 보세요.

## Headless

같은 에이전트, alt-screen 없음:

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi -p "요약해" --output-format json
```

쓰기·bash tool은 비 TTY / JSON에서 `--always-approve` 없이 거부됩니다.
[Headless](headless.md)를 보세요.

## Next

- [TUI](tui.md) key와 slash command
- [CLI](cli.md) reference
- [Configuration](configuration.md)

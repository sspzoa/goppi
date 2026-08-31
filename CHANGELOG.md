# Changelog

## 0.168.0 — 2026-08-31

- headless `-p` invalid `output-format`도 JSON `error` 출력·cmd E2E

## 0.167.0 — 2026-08-31

- headless JSON workdir·root·`-c` 시작 실패 E2E·`complete formats` smoke

## 0.166.0 — 2026-08-31

- headless JSON 시작 실패(resume lock·missing key 등)도 `error` object 출력·cmd E2E

## 0.165.0 — 2026-08-31

- headless `--yolo` MCP allow·login `--stdin` bash completion·ci-headless export/reasoning smoke

## 0.164.0 — 2026-08-31

- headless resume locked session cmd E2E·ci-cli-smoke `mcp list`

## 0.163.0 — 2026-08-31

- root workdir reject·headless `--always-approve` MCP allow cmd E2E

## 0.162.0 — 2026-08-31

- plan mode MCP deny headless cmd E2E

## 0.161.0 — 2026-08-31

- headless MCP tool deny cmd E2E·`scripts/ci-mock-mcp.py`·ci-cli-smoke `complete slash`

## 0.160.0 — 2026-08-31

- project `lsp_servers` ignore cmd E2E·ci-cli-smoke에 `complete efforts`·`worktree list` 추가

## 0.159.0 — 2026-08-31

- project mcp/worktree/hooks ignore·worktree remove unknown id·plan mode JSON cmd E2E
- complete efforts prefix·`.env.example` GOPPI_SANDBOX 안내

## 0.158.0 — 2026-08-31

- project sandbox ignore·env strict sandbox·complete slash·help key redaction cmd E2E
- `make ci-cli-smoke`에 idempotent `logout` 추가

## 0.157.0 — 2026-08-31

- doctor last symlink·logout without key·headless positional prompt cmd E2E
- project `.goppi.json` always_approve ignore·complete sessions prefix 테스트

## 0.156.0 — 2026-08-31

- headless `--yolo`·`--max-turns`·project `.goppi.json` api_key ignore cmd E2E 테스트

## 0.155.0 — 2026-08-31

- plan mode edit_file/apply_patch·login empty prompt·sessions rm alias cmd E2E
- `make ci-headless-smoke`가 JSON `worktree` 필드를 grep 검증한다

## 0.154.0 — 2026-08-31

- doctor exports/worktrees symlink·login empty stdin·export bad id cmd E2E 테스트
- `login --stdin`과 EOF/빈 줄이 empty key로 거부되도록 수정
- `make ci-cli-smoke`가 inspect JSON을 한 번만 실행한다

## 0.153.0 — 2026-08-31

- init existing GOPPI.md 보존·sessions/mcp unknown subcommand·delete unknown id cmd E2E
- `make ci-headless-smoke`가 JSON `mode`·`workdir` 필드를 grep 검증한다

## 0.152.0 — 2026-08-31

- plan mode bash 거부·inspect plain key redaction·worktree usage cmd E2E 테스트
- `make ci-cli-smoke`가 plain inspect key source 출력을 grep 검증한다

## 0.151.0 — 2026-08-31

- headless JSON `edit_file`·`apply_patch` 거부 cmd E2E 테스트
- `make ci-cli-smoke`·inspect JSON에 `mode` 필드 검증

## 0.150.0 — 2026-08-31

- bad model/provider·`-c` without session·`export`/`sessions delete` usage cmd E2E 테스트
- `make ci-headless-smoke`가 JSON `usage` 필드를 grep 검증한다

## 0.149.0 — 2026-08-31

- headless `-p`가 SIGINT(ctrl+c)도 취소하도록 `NotifyStopHeadless` 추가 (문서와 일치)
- headless SIGINT 취소 cmd E2E 테스트

## 0.148.0 — 2026-08-31

- headless SIGTERM 취소 cmd E2E: JSON error·session_id 검증
- bad `--effort`/`--sandbox`/`--mode` cmd 거부 테스트
- `make ci-cli-smoke`에 `goppi models` grep 추가

## 0.147.0 — 2026-08-31

- `make ci-headless-smoke`가 headless JSON smoke를 한 번만 실행한다

## 0.146.0 — 2026-08-31

- `make lint`가 actionlint으로 GitHub workflow YAML을 검증한다
- `goppi completions` unknown shell cmd E2E 테스트

## 0.145.0 — 2026-08-31

- `make ci-cli-smoke`가 version·complete flags·completions bash 출력을 grep으로 검증한다
- `make ci-headless-smoke`가 headless JSON `session_id` 형식을 검증한다
- `goppi models`·completions·complete unknown kind cmd E2E 테스트

## 0.144.0 — 2026-08-31

- `make ci-cli-smoke`가 inspect·complete 출력을 grep으로 검증한다
- doctor `--fix` + leading `-C`, headless bad `-r` session id E2E 테스트

## 0.143.0 — 2026-08-31

- CI workflow가 release workflow와 같이 `make release-prepare`를 사용한다
- development·SECURITY에 production readiness·pre-release 게이트 문서화

## 0.142.0 — 2026-08-31

- `make release-prepare`(`release-check` + `ci-smoke`)로 release workflow와 로컬 tag 게이트를 맞춘다
- headless `-r` resume session cmd E2E 테스트

## 0.141.0 — 2026-08-31

- `make ci-smoke`로 CLI·headless smoke를 한 타깃으로 묶고 CI·release workflow에 적용한다
- headless `--always-approve` write 허용·sessions list secret redaction cmd E2E 테스트

## 0.140.0 — 2026-08-31

- headless JSON chat error·`-c` resume session cmd E2E 테스트

## 0.139.0 — 2026-08-31

- `make verify-dist` tarball 개수 검사가 macOS `wc -l` 앞 공백에서 실패하지 않도록 수정한다
- release workflow에 `make ci-cli-smoke` 추가

## 0.138.0 — 2026-08-31

- `make ci-cli-smoke`로 CI CLI smoke를 Makefile에서 재현한다
- `make pre-release`에 ci-cli-smoke를 포함해 CI와 동일한 로컬 게이트를 맞춘다
- `goppi export` secret redaction cmd E2E 테스트

## 0.137.0 — 2026-08-31

- `make verify-dist`가 tarball 개수·현재 버전 일치까지 검증한다
- CI·release workflow가 inline smoke 대신 `make ci-headless-smoke`를 쓴다
- headless JSON secret redaction cmd E2E 테스트

## 0.136.0 — 2026-08-31

- headless JSON에서 write_file 거부·plan mode write 거부 cmd E2E 테스트
- `make check`가 `scripts/ci-mock-solar.py` syntax를 검증한다
- GOPPI.md에 pre-release·live-check 안내

## 0.135.0 — 2026-08-31

- `make verify-dist`가 fake cosign으로 dist tarball cosign install 경로까지 검증한다
- getting-started tag 전 안내를 `make pre-release`로 맞춘다

## 0.134.0 — 2026-08-31

- `make pre-release`로 tag 전 전체 로컬 게이트(release-check·headless·live)를 한 번에 돌린다
- install.sh cosign 성공·실패 경로 테스트, release workflow dist layout·headless smoke 검증
- `.env.example`에 선택 env·live-check 안내 보강

## 0.133.0 — 2026-08-31

- CI가 mock Solar(`scripts/ci-mock-solar.py`)로 headless `--output-format json -p` smoke를 돌린다
- bad workdir `-C`에 대한 headless `run` 거부 테스트와 `make ci-headless-smoke`

## 0.132.0 — 2026-08-31

- README·development에 `live-check`와 release 증명 절차를 문서화한다

## 0.131.0 — 2026-08-31

- headless `--output-format json`에서 bash tool 거부를 cmd E2E 테스트로 검증한다
- `make live-check`로 `UPSTAGE_API_KEY`가 있을 때만 Solar smoke를 돌린다

## 0.130.0 — 2026-08-31

- `make dist`가 이전 `dist/goppi_*.tar.gz`를 지워 release artifact가 현재 버전만 남도록 한다
- bad workdir `-C`에 대한 `doctor` 거부 테스트와 CI `-C doctor` smoke

## 0.129.0 — 2026-08-31

- `make verify-dist`가 macOS CI에서도 native darwin tarball `goppi version`을 검증한다
- bad workdir `-C`에 대한 `inspect` 거부 테스트와 getting-started `release-check` 안내

## 0.128.0 — 2026-08-31

- `make verify-dist`가 빌드된 `dist/` tarball로 `install.sh` end-to-end smoke를 돌린다

## 0.127.0 — 2026-08-31

- CI ubuntu·macOS 모두 `make release-check`를 돌려 release 게이트와 동일하게 맞춘다
- release job timeout 45분, CI timeout 30분
- bad workdir `-C` 거부 테스트와 SECURITY/README maintainer 메모를 추가한다

## 0.126.0 — 2026-08-31

- release workflow가 `make release-check`로 CI·tag 게이트와 동일한 검증을 돌린다
- `peelWorkdirArgs`·`doctor -C dir`·`GOPPI_INSTALL_DIR` install 경로를 테스트한다

## 0.125.0 — 2026-08-31

- `goppi -C dir doctor` · `inspect`가 workdir를 반영한다 (`loadConfigWithWorkdirArgs`)
- `make release-check`(`check` + `verify-dist`)로 tag 전 로컬 게이트를 추가한다
- install.sh tarball download 실패 경로를 테스트한다

## 0.124.0 — 2026-08-31

- release workflow가 중복 검증 대신 `make verify-dist`를 사용한다
- install.sh broken archive·go 미설치 fallback 실패 경로를 테스트한다

## 0.123.0 — 2026-08-31

- CI ubuntu job이 `make verify-dist`로 release artifact를 매 push마다 검증한다
- install.sh `go install` fallback·Go 1.27 미만 거부·`GOPPI_INSTALL_FROM=go`를 테스트한다
- `scripts/package.sh`가 stale `SHA256SUMS.sigstore.json`을 지우고 `.gitignore`에 `*.pem`/`*.key`를 추가한다

## 0.122.0 — 2026-08-31

- `findDispatch`는 `-C`/`--cwd` 뒤 subcommand만 인식하고 `-p` 등 run flag 뒤 단어는 prompt로 둔다
- headless JSON이 `context canceled`를 `error` 필드로 내보내는지 테스트한다
- `make verify-dist`로 로컬 release tarball·checksum·layout(및 Linux version)을 검증한다
- `scripts/package.sh`는 이번에 빌드한 tarball만 `SHA256SUMS`에 넣는다

## 0.121.0 — 2026-08-31

- CI smoke에 `doctor` · `sessions list` · `init`을 추가한다
- `goppi -C dir init`처럼 run flag 뒤에 오는 subcommand를 올바르게 dispatch한다
- `goppi init -C dir`로 workdir를 지정할 수 있다

## 0.120.0 — 2026-08-31

- release workflow에 govulncheck와 4개 tarball layout 검증을 추가한다
- `TestPackageScriptTarballLayout`·`TestInstallScriptBundleRequiresCosignBinary`로 패키지·설치 경로를 검증한다
- `.env.example`을 추가하고 README에 `make check`를 문서화한다

## 0.119.0 — 2026-08-31

- release workflow가 cosign 서명 직후 `verify-blob`로 번들을 자체 검증한다

## 0.118.0 — 2026-08-31

- CI가 로컬과 같이 `make check`(staticcheck·govulncheck 포함)를 ubuntu·macOS 모두에서 돌린다
- `credentials.json`을 `.gitignore`에 추가한다

## 0.117.0 — 2026-08-31

- release workflow와 release 테스트가 `sha256sum -c SHA256SUMS`로 tarball checksum을 검증한다

## 0.116.0 — 2026-08-31

- CI smoke는 `make build`의 `./bin/goppi`를 쓴다 (로컬·릴리스 ldflags와 동일)
- release workflow는 SHA256SUMS에 4개 tarball이 모두 있는지 확인한다

## 0.115.0 — 2026-08-31

- release workflow는 staticcheck와 linux tarball `goppi version` 검증을 추가한다

## 0.114.0 — 2026-08-31

- line REPL `/quit` persist 실패가 `lineLoop` exit code까지 전파되는지 integration 테스트
- headless JSON 문서에 `session save:` 오류·exit 1 명시

## 0.113.0 — 2026-08-31

- headless `--output-format json`는 session 저장 실패를 JSON `error`와 exit 1로 돌려준다 (E2E 테스트)
- line REPL `/quit` persist 실패 회귀 테스트

## 0.112.0 — 2026-08-31

- line REPL은 SIGTERM 등 parent cancel 중 session 저장 실패를 경고만 하지 않고 `session save:` 오류로 돌려준다
- `scripts/package.sh` 아티팩트가 `config.Version`을 embed하는지 release 테스트로 확인한다

## 0.111.0 — 2026-08-31

- headless `RunOnce`는 session 저장 실패를 `session save:` 오류로 돌려준다 (회귀 테스트)

## 0.110.0 — 2026-08-31

- `make build`·`make dist`의 VERSION은 `internal/config/config.go`를 따른다 (git describe 덮어쓰기 제거)
- `make check`는 `bin/goppi version` 일치를 확인한다
- release workflow는 tag와 config Version이 같을 때만 패키징한다

## 0.109.0 — 2026-08-31

- CI는 `./goppi version`이 `internal/config/config.go`의 Version과 같은지 확인한다

## 0.108.0 — 2026-08-31

- TUI `/quit`는 persist 실패를 화면에 보여 주고 `finish()`도 같은 오류를 돌려준다
- ACP `session/load`도 상대 `cwd`를 거절한다

## 0.107.0 — 2026-08-31

- `make check` staticcheck 경고를 정리한다 (ACP context, diff TrimSuffix, Linux sandbox helper, TUI dead code)

## 0.106.0 — 2026-08-31

- session JSON에 `extra_dirs`를 저장해 ACP load·resume이 클라이언트가 다시 안 넘겨도 extra root를 복원한다

## 0.105.0 — 2026-08-31

- ACP `session/new` · `load` · `resume`는 상대 `cwd`를 거절한다
- `authenticate`는 auth method 없이도 빈 성공을 돌려 에디터가 끊기지 않게 한다

## 0.104.0 — 2026-08-31

- ACP `session/load` · `session/resume`는 저장된 workdir를 쓰고, 다른 `cwd`는 거절한다

## 0.103.0 — 2026-08-31

- TUI SIGTERM은 진행 중인 turn을 취소한 뒤 끝날 때까지 기다렸다가 transcript를 저장한다

## 0.102.0 — 2026-08-31

- ACP `session/resume`는 저장된 세션을 히스토리 replay 없이 복구한다
- line REPL SIGTERM 테스트는 transcript가 남는지까지 확인한다

## 0.101.0 — 2026-08-31

- TUI는 SIGTERM·SIGHUP으로 끝나도 메모리 transcript를 저장한다. `/quit`과 같은 persist

## 0.100.0 — 2026-08-31

- TUI 왼쪽 위와 line banner는 `고삐`만 쓴다 (`한국형` 태그 제거)
- README·docs는 grok-build식 목차와 영문 헤더, 본문은 한글

## 0.99.0 — 2026-08-31

- ACP·MCP·LSP JSON-RPC 줄과 헤더는 상한을 넘기면 거절한다 (본문 8/4 MiB, 헤더 8 KiB)
- `make check`는 CI와 같이 staticcheck·govulncheck를 돌린다. `v*` release는 패키지 전에 `go test -race`를 돌린다

## 0.98.0 — 2026-08-31

- hook stdin의 tool input·결과도 키 패턴을 지운다
- line REPL은 SIGTERM·SIGHUP 뒤 다음 프롬프트를 열지 않는다

## 0.97.0 — 2026-08-31

- session title은 저장·ACP `session/list`·`/sessions`·export 제목에서 키 패턴을 `[redacted]`로 지운다

## 0.96.0 — 2026-08-31

- TUI 스크롤백 재구성과 headless `--output json`도 키 패턴을 `[redacted]`로 지운다

## 0.95.0 — 2026-08-31

- ACP `session/prompt` · `session/close`는 transcript 쓰기에 실패하면 `session save:`로 끝난다. `end_turn`으로 위장하지 않는다

## 0.94.0 — 2026-08-31

- ACP `resource_link` `file://`는 `additionalDirectories` 안도 읽는다
- MCP·LSP·hook sandbox 쓰기 루트에 extra dir을 넣는다. bash와 같은 경계

## 0.93.0 — 2026-08-31

- ACP `session/update` · `session/request_permission`과 확인 panel은 키 패턴을 `[redacted]`로 지운다. load replay와 live stream 포함

## 0.92.0 — 2026-08-31

- glob·grep는 ACP `additionalDirectories`도 검색한다
- `goppi login`은 키를 위치 인자로 받지 않는다. `--stdin` 또는 프롬프트

## 0.91.0 — 2026-08-31

- ACP `session/new` · `session/load`의 `additionalDirectories`를 파일·bash sandbox 경계에 넣는다. 상대 경로와 `/`는 거절. 최대 8개
- `goppi export` Markdown도 키 패턴을 `[redacted]`로 지운다

## 0.90.0 — 2026-08-31

- CI와 `make check`가 Windows용 `go test -c`까지 컴파일한다. 테스트 파일이 깨져도 머지 전에 걸린다

## 0.89.0 — 2026-08-31

- LSP(자동 `gopls` 포함)도 bash·MCP와 같은 workspace/strict sandbox로 켠다

## 0.88.0 — 2026-08-31

- MCP 서버도 bash와 같은 workspace/strict sandbox로 켠다. 세션·`goppi mcp tools` 모두

## 0.87.0 — 2026-08-31

- ACP `session/new` · `session/load`의 stdio `mcpServers`를 켠다. HTTP/SSE는 건너뛴다. user config 서버와 합친다

## 0.86.0 — 2026-08-31

- workdir는 있는 디렉터리여야 하고 `/`(볼륨 루트)는 거절한다. sandbox 쓰기 루트가 파일시스템 전체가 되지 않게

## 0.85.0 — 2026-08-31

- `goppi login` · `doctor`는 키 앞뒤를 터미널에 찍지 않는다. 있으면 source만 (`goppi login`, `UPSTAGE_API_KEY`)

## 0.84.0 — 2026-08-31

- ACP 테스트는 클라이언트 읽기를 한 `Conn`으로 유지한다. 알림이 파이프에 쌓여도 다음 `readRPC`가 새 bufio로 삼키지 않는다

## 0.83.0 — 2026-08-31

- ACP `session/close` · `load` · `delete`는 그 세션 prompt를 취소한 뒤 끝날 때까지 기다린다. 두 agent가 같은 transcript를 동시에 쓰지 않는다

## 0.82.0 — 2026-08-31

- MCP·LSP stderr는 터미널에 쓰지 않고 8 KiB로 가둔다. TUI alt-screen과 키 로그가 깨지지 않는다

## 0.81.0 — 2026-08-31

- `install.sh`는 `SHA256SUMS.sigstore.json`이 있으면 cosign 검증을 강제한다. 공식 GitHub도 SHA256만으로 넘어가지 않는다 (`GOPPI_SKIP_COSIGN=1`은 테스트용)

## 0.80.0 — 2026-08-31

- hook도 bash와 같은 sandbox와 `.git` 되돌림을 쓴다. workdir 밖 쓰기와 `HEAD`/`refs` 변조를 막는다

## 0.79.0 — 2026-08-31

- Windows에서도 같은 session id는 `LockFileEx`로 한 프로세스만 잡는다. CI는 Windows amd64 크로스 컴파일을 검사한다

## 0.78.0 — 2026-08-31

- 이미 `.git`이 있으면 bash가 끝난 뒤 `HEAD` · `refs` · `packed-refs`도 되돌린다. 객체만 지우고 브랜치가 빈 SHA를 가리키는 일을 막는다. macOS seatbelt도 그 경로 쓰기를 실행 중 거부한다

## 0.77.0 — 2026-08-31

- ACP `session/set_mode` · `session/set_config_option`은 그 세션이 prompt 중이면 `busy`. 생성 중에 mode/model이 바뀌지 않는다

## 0.76.0 — 2026-08-31

- `write_file` / `edit_file` / `apply_patch`는 `.git` 아래를 전부 거부한다. `HEAD`·`refs`로 브랜치를 바꾸는 일을 막는다. `bash`의 hook 되돌림은 그대로

## 0.75.0 — 2026-08-31

- ACP `session/prompt`가 같은 세션에서 겹치면 `busy`로 거절한다. 두 `Run`이 transcript를 동시에 쓰지 않는다

## 0.74.0 — 2026-08-31

- Linux sandbox helper는 child env를 다시 `ScrubEnv`한다. `cmd.Env`를 빠뜨려도 jailed bash에 API key가 넘어가지 않는다. `darwinProfile` 테스트는 darwin 전용 파일이라 Linux CI가 빌드된다

## 0.73.0 — 2026-08-31

- `GOPPI_RELEASE_BASE`가 GitHub `sspzoa/goppi` release가 아니면 cosign 서명을 강제한다. 미러의 SHA256SUMS만으로는 설치하지 않는다 (`GOPPI_SKIP_COSIGN=1`은 테스트용)

## 0.72.0 — 2026-08-31

- `sessions/` · `exports/` · `worktrees/`가 symlink면 쓰지 않는다. `goppi doctor`도 그 디렉터리를 검사한다

## 0.71.0 — 2026-08-31

- git worktree child에도 `*API_KEY` / `*SECRET` / `*_TOKEN`이 넘어가지 않는다. bash·MCP·LSP와 같은 `ScrubEnv`

## 0.70.0 — 2026-08-31

- `goppi help`와 셸 자동완성이 `complete`를 빠뜨리지 않는다. 커맨드 목록은 `CLICommands` 한곳

## 0.69.0 — 2026-08-31

- session lock이 symlink면 flock하지 않는다 (unix `O_NOFOLLOW`). `last` pointer는 일반 파일·64바이트만 읽고, 삭제 때도 링크를 따라가지 않는다

## 0.68.0 — 2026-08-31

- hook timeout·취소는 process group을 죽인다. `bash -lc`만 끊고 `sleep` 같은 자식이 남는 일을 막는다

## 0.67.0 — 2026-08-31

- session JSON과 `last` / `last.json` pointer가 symlink면 따라가지 않는다. `-c`는 링크를 무시하고 남은 세션 중 가장 최근 것으로 이어간다

## 0.66.0 — 2026-08-31

- `pre_tool` hook이 거절할 때 stdout의 키 패턴이 에러로 모델에 다시 들어가지 않는다. tool 결과와 같이 `[redacted]`

## 0.65.0 — 2026-08-31

- Line REPL 대기 prompt의 `ctrl+c`는 기본 SIGINT로 죽지 않는다. transcript를 저장하고 MCP·LSP·bash를 닫은 뒤 끝난다. 생성 중의 SIGINT는 그대로 turn만 취소

## 0.64.0 — 2026-08-31

- `credentials.json`·user `config.json`·config/data dir가 symlink면 읽지 않는다. `login`은 링크를 지우고 일반 파일로 쓴다. 링크 너머로 키가 새지 않는다

## 0.63.0 — 2026-08-31

- workspace/strict sandbox는 `sudo` / `su` / `doas`와 setuid를 막는다. Linux는 기존 `no_new_privs`, macOS seatbelt는 그 바이너리 exec를 거부. `sandbox=off`는 그대로

## 0.62.0 — 2026-08-31

- 같은 session id를 두 프로세스가 동시에 잡지 못한다. `-c` / `-r` / `/sessions` / ACP load는 `already in use`. 종료·`/new`·삭제가 lock을 푼다
- ACP 종료는 진행 중인 prompt가 끝난 뒤에 세션을 닫는다

## 0.61.0 — 2026-08-31

- `web_fetch`는 연결 직전 IP를 다시 검사한다. 검사 때 공인 IP, 연결 때 169.254/사설망으로 바뀌는 DNS rebinding을 막는다

## 0.60.0 — 2026-08-31

- SIGTERM·SIGHUP는 MCP·LSP·bash를 닫고 session_end를 돌린 뒤 끝난다. Headless·TUI·ACP·line REPL. SIGINT는 그대로 turn 취소

## 0.59.0 — 2026-08-31

- `bash`는 이미 `.git`이 있으면 끝난 뒤 `.git/hooks` · `.git/config` · `.git/objects` 쓰기를 되돌린다. Linux landlock과 `sandbox=off`도 같다. worktree는 `commondir`를 따른다. `.git`이 없던 디렉터리의 `git init`은 그대로

## 0.58.0 — 2026-08-31

- macOS workspace/strict sandbox는 `.git/hooks` · `.git/config` · `.git/objects` 쓰기를 거부한다. Linux landlock은 허용한 workdir 아래를 빼지 못한다. `sandbox=off`는 그대로

## 0.57.0 — 2026-08-31

- LSP child에는 `*API_KEY` / `*SECRET` / `*_TOKEN`이 넘어가지 않는다
- `write_file` / `edit_file` / `apply_patch`는 `.git/hooks`, `.git/config`, `.git/objects` 쓰기를 거부한다

## 0.56.0 — 2026-08-31

- MCP·LSP 종료와 초기화 실패가 process group을 죽인다. `npx`/`sleep` 자식이 세션 종료 후에도 남지 않는다 (unix)

## 0.55.0 — 2026-08-31

- `config.json`과 `.goppi.json`에 모르는 키가 있으면 시작을 거부한다. `alwaysApprove` / `sandox` 같은 오타가 기본값으로 조용히 지나가지 않는다

## 0.54.0 — 2026-08-31

- SSE 누적 8 MiB 상한이 tool argument에도 적용된다. 본문만 막고 인자로 메모리를 채우던 구멍을 닫는다

## 0.53.0 — 2026-08-31

- `glob` / `grep`는 하위 디렉터리 `.gitignore`도 따른다. 가까운 파일이 나중 규칙을 이긴다. 파일당 256 KiB 상한

## 0.52.0 — 2026-08-31

- session을 disk에 쓰지 못하면 턴이 성공해도 `session save:` 로 실패한다. 답은 transcript에 남는다

## 0.51.0 — 2026-08-31

- `-c`는 `last`가 없거나 그 파일이 없으면 남은 세션 중 가장 최근 것으로 이어간다. 잘못된 pointer(`../`)는 거부. 파일 이름은 session id다
- `goppi doctor`는 깨진 session JSON을 실패로 본다

## 0.50.0 — 2026-08-31

- `glob` / `grep`는 루트 `.gitignore`와 `.git/info/exclude`를 따른다. `!` 예외와 맨 앞 `/` 고정. 경로를 직접 주면 그 파일은 검색한다. workdir 이름이 `bin`이어도 검색한다

## 0.49.0 — 2026-08-31

- `write_file` / `edit_file` / `apply_patch` 허용 화면에 바꿀 줄 미리보기를 넣는다. TUI panel과 line REPL 모두

## 0.48.0 — 2026-08-31

- 프롬프트 `@path`는 workdir 텍스트 파일을 그 턴 user message에 붙인다. 이미지는 기존처럼 `image_url`. 파일당 64 KiB, 최대 4개. workdir 밖·바이너리는 건너뛴다

## 0.47.0 — 2026-08-31

- provider와 compact, `/copy`로 나가는 transcript는 키 패턴을 지운다. 화면의 지금 입력은 그대로

## 0.46.0 — 2026-08-31

- tool 결과와 세션 저장본은 env 시크릿 값과 `sk-`/`up_`/`ghp_`/`AKIA` 같은 키 패턴을 `[redacted]`로 바꾼다. 메모리의 지금 transcript는 그대로

## 0.45.0 — 2026-08-31

- `goppi doctor`는 `sessions/*.json`·`exports/*.md`·`last`가 group/other에 열려 있으면 실패한다. `--fix`가 `0600`으로 되돌린다

## 0.44.0 — 2026-08-31

- `web_fetch`는 localhost, 루프백, RFC1918, CGNAT(100.64/10), userinfo를 거부한다. hostname이 그 IP로 풀려도 막는다

## 0.43.0 — 2026-08-31

- session JSON이 last·Σ 토큰을 남긴다. `-c` / `-r` / `/sessions` 이어가면 `/status`와 export에 그대로 나온다
- Line REPL `/clear`는 `/new`와 같다

## 0.42.0 — 2026-08-31

- user config `hooks.session_end`. `/new`·세션 load·종료·삭제가 돌린다. stdin JSON에 `reason`(`reset`/`load`/`close`/`delete`)
- `/new`와 세션 이어가기는 `session_start`를 다시 돌린다. `/status` last 토큰은 새 세션에서 0

## 0.41.0 — 2026-08-31

- ACP `session/delete`가 transcript·export·worktree를 지운다. `initialize`에 `sessionCapabilities.delete`를 광고한다. 없는 id는 성공
- TUI·REPL `/delete [id]`가 같다. 지금 세션이면 지운 뒤 새 id를 만든다
- `sessions delete` prefix는 해석된 id의 worktree를 지운다. export Markdown도 같이 지운다

## 0.40.0 — 2026-08-31

- `/new`와 세션 load가 이전 세션의 백그라운드 bash를 죽인다. job 한도가 새 transcript에 새지 않는다
- `ResetRuntime`이 세션 allow·undo 스냅샷도 같이 지운다

## 0.39.0 — 2026-08-31

- `/status`가 마지막 turn 토큰(input→output reasoning)을 보여 준다. TUI는 이번 세션 합도 `Σ`

## 0.38.0 — 2026-08-31

- TUI·REPL `/copy`가 마지막 assistant 답을 OSC 52로 터미널 클립보드에 넣는다 (256 KiB 상한)
- TTY가 아니면 시퀀스를 쓰지 않는다. 답이 없으면 실패

## 0.37.0 — 2026-08-31

- ACP `session/load`가 user/assistant/reasoning을 `session/update`로 다시 보낸 뒤 응답한다
- 같은 id가 이미 열려 있으면 먼저 닫고, 저장된 model·effort·todos를 `LoadFile`로 복원한다

## 0.36.0 — 2026-08-31

- ACP `session/list`가 disk(+메모리) 세션을 돌려준다. `cwd` 필터, cursor 페이지
- `session/close`가 실행 중 턴을 취소하고 저장한 뒤 tool을 닫는다. 프로세스 종료 때도 남은 세션을 정리한다

## 0.35.0 — 2026-08-31

- ACP `session/new`·`load`가 `modes`(act/plan)와 `configOptions`(mode·model·effort)를 돌려준다
- `session/set_mode` / `session/set_config_option`으로 에디터가 모드·모델·reasoning을 바꾼다

## 0.34.0 — 2026-08-31

- ACP `session/prompt`가 `resource`(embedded context)와 `resource_link`를 받는다. initialize `embeddedContext=true`
- `resource_link`의 `file://`는 workdir 안에서만 읽고, 밖은 URI만 남긴다

## 0.33.0 — 2026-08-31

- `goppi doctor --online`이 저장된 키로 `GET /models`(없으면 1토큰 chat)을 호출한다. 거부되면 실패

## 0.32.0 — 2026-08-31

- `goppi doctor`는 `config.json`에 `api_key`가 있는데 group/other에 열려 있으면 실패한다
- config·data 디렉터리가 world-writable이면 실패한다. `--fix`가 `0600`/`0700`으로 되돌린다

## 0.31.0 — 2026-08-31

- `/retry`는 마지막 user 턴부터 지우고 같은 프롬프트를 다시 보낸다. 실패·취소 다음, 또는 답 재생성
- 이미지 첨부는 path가 본문에 없으면 Incoming으로 다시 붙인다

## 0.30.0 — 2026-08-31

- `goppi login`은 저장 전에 API가 키를 받는지 확인한다. 거부되면 파일을 쓰지 않는다
- 공기갭·테스트는 `--offline`

## 0.29.0 — 2026-08-31

- TUI·REPL `/export`가 지금(또는 id) 세션을 `exports/<id>.md`(mode `0600`)로 쓴다
- CLI `goppi export`는 그대로 stdout. 파일은 `/export` 또는 리다이렉트

## 0.28.0 — 2026-08-31

- `goppi doctor`는 `credentials.json`이 group/other에 열려 있으면 실패한다. `--fix`가 `0600`으로 되돌린다
- 키가 있어도 world-readable이면 CI·공유 머신에서 통과시키지 않는다

## 0.27.0 — 2026-08-31

- `/sessions` + id(또는 유일한 prefix)로 TUI·REPL에서 세션을 이어간다. 지금 transcript는 먼저 저장한다
- `-r` / `sessions delete` / `export`도 prefix를 받는다. `..` 경로는 거부

## 0.26.0 — 2026-08-31

- API 401/403/404/429/5xx는 상태와 다음에 할 일을 같이 보여 준다. `upstage 401 Unauthorized` 대신 `API 401` + `goppi login`
- 턴이 끝나면 터미널 BEL (iTerm은 OSC 9). 취소는 울리지 않는다. `GOPPI_NOTIFY=off`로 끔

## 0.25.0 — 2026-08-31

- session id가 있으면 user·assistant·tool마다 disk에 남긴다. 프로세스 종료·취소 전에 한 턴이 끝나지 않아도 `-c`로 이어간다
- TUI / REPL / headless `-p`는 첫 메시지 전에 id를 만든다. 테스트처럼 id가 없으면 쓰지 않는다

## 0.24.0 — 2026-08-31

- `apply_patch`: Codex식 `*** Begin Patch`로 여러 파일·여러 hunk를 한 번에. Add / Update / Delete
- workdir 가둠, plan 거부, 권한 panel, `/undo`·`/diff` 스택에 남긴다. hunk가 두 번 맞으면 거부

## 0.23.0 — 2026-08-31

- `/diff`: 이번 세션 `write_file` / `edit_file`을 원래 내용과 unified diff로 본다. Cline / OpenCode / Codex의 session diff
- 첫 스냅샷 기준. `/new`는 undo 스택과 같이 지운다. 생성 중에도 된다

## 0.22.0 — 2026-08-31

- 권한 panel `a` / REPL `a` / ACP `allow-session`: 그 tool을 이번 세션에서 다시 묻지 않는다. Cline / Codex / OpenCode의 allow-for-session
- `y`는 한 번만. `/new`와 `Reset`은 허용 목록을 지운다. `--always-approve`는 그대로 전역

## 0.21.0 — 2026-08-31

- TUI에서 생성 중 enter는 다음 메시지를 대기열에 넣는다 (최대 4). 턴이 끝나면 이어서 보낸다. 취소하면 버린다. OpenCode / Cline / Codex의 follow-up queue
- `/jobs`: 백그라운드 bash 한 줄 목록. 생성 중에도 된다

## 0.20.0 — 2026-08-31

- `ask_user`: 선택지가 있으면 번호로 고른다. Cline / Codex / pi의 질문 tool
- TTY·TUI·ACP(`session/request_permission`)에서만. headless JSON은 거부해서 모델이 추측하게 한다

## 0.19.0 — 2026-08-31

- `goppi acp` session/prompt가 image 블록을 받는다. initialize `promptCapabilities.image=true`
- 디스크에 쓰지 않고 메모리 data URL로만 붙인다. 장당 4 MiB, 최대 3장

## 0.18.0 — 2026-08-31

- `bash` `background=true`가 job을 남긴다. `bash_poll` / `bash_kill`. Cline / OpenCode / Codex의 background shell
- 최대 4개. turn 취소는 건드리지 않고, 에이전트 Close가 process group을 죽인다

## 0.17.0 — 2026-08-31

- `--sandbox strict` / `GOPPI_SANDBOX=strict`: 쓰기 가둠에 네트워크 jail을 더한다. Codex식 network-off
- 기본은 여전히 `workspace` (`go test`·`git`·`curl`이 필요해서)

## 0.16.0 — 2026-08-31

- 한 turn의 읽기 tool은 같이 실행한다 (최대 8). 쓰기·bash·MCP가 섞이면 순서로
- 취소는 배치 전체를 끊고, 없는 tool 결과는 기존처럼 transcript를 닫는다

## 0.15.0 — 2026-08-31

- user config `hooks`: `pre_tool` / `post_tool` / `session_start`. Cline / Codex / OpenCode와 같은 정책 훅
- 프로젝트 `.goppi.json`의 hooks는 무시. 명령은 API key를 빼 둔 채 15초 안에 돈다. `pre_tool` 비0 종료는 그 tool을 막는다

## 0.14.0 — 2026-08-31

- 컨텍스트가 차면 자동으로 `/compact`하고 같은 turn을 이어 간다. Cline / OpenCode / Codex와 같은 축
- `GOPPI_AUTO_COMPACT=off`로 끄고, `compact_at`(기본 100000 input tokens)로 임계값을 바꾼다

## 0.13.0 — 2026-08-31

- `goppi acp`: Agent Client Protocol v1 stdio (initialize, session/new|load|prompt|cancel, permission)
- `sessions delete`가 해당 git worktree도 지운다

## 0.12.0 — 2026-08-31

- `--worktree` / `GOPPI_WORKTREE`가 git worktree를 만들어 메인 체크아웃을 그대로 둔다. 브랜치 `goppi/<session>`
- `goppi worktree list|remove`. 프로젝트 `.goppi.json`은 `worktree`를 켤 수 없다
- Line REPL `/status`가 mode·sandbox·worktree·session을 보여 준다

## 0.11.0 — 2026-08-31

- `diagnostics` tool: stdio LSP (JSON-RPC). user config `lsp_servers` 또는 Go 모듈에서 `gopls` 자동
- 프로젝트 `.goppi.json`의 `lsp_servers`는 무시. `GOPPI_LSP=off`로 끌 수 있다

## 0.10.0 — 2026-08-31

- `bash` 기본 sandbox는 `workspace`: workdir·임시·캐시 밖 쓰기를 거부한다 (macOS seatbelt, Linux landlock)
- `GOPPI_SANDBOX=off` / `--sandbox off` / user config만 끌 수 있다. 프로젝트 `.goppi.json`은 무시

## 0.9.5 — 2026-08-31

- Line REPL `/quit`와 EOF도 현재 transcript를 저장한다
- HTTP client는 TLS 1.2 미만을 거부한다
- `goppi doctor`가 workdir·session dir 쓰기 가능 여부를 확인한다
- zsh/bash/fish completion에 `mcp list|tools`를 넣는다
- CI에 `inspect --json` / `complete commands` smoke와 job timeout을 넣는다

## 0.9.4 — 2026-08-31

- HTTP 응답, SSE 누적, tool input, API key 길이에 상한
- TUI 종료 시 현재 transcript를 저장

## 0.9.3 — 2026-08-31

- `/new`는 메모리에 있는 transcript를 먼저 저장한 뒤에 새 session을 연다
- session 파일은 8 MiB를 넘기면 읽기·쓰기를 거부. 깨진 JSON은 목록에서 건너뛴다
- `goppi sessions` / `export` / `delete` 경로를 테스트로 고정

## 0.9.2 — 2026-08-31

- Line REPL의 SIGINT는 그 turn만 취소한다. 다음 prompt가 죽지 않는다
- `bash`는 process group을 죽여서 취소·timeout에 sleep 자식이 남지 않게 한다
- 권한 panel에서 `ctrl+c`는 거부 + turn 취소
- `inspect --json`은 API key 값을 넣지 않는다. credentials 파일은 `0600`

## 0.9.1 — 2026-08-31

- `scripts/package.sh`가 release artifact를 만들고, `install.sh`는 그 레이아웃을 SHA256(+ 있으면 cosign)으로 설치. 로컬 미러는 `GOPPI_RELEASE_BASE`
- headless JSON은 실패해도 object를 남기고 `error` / `mode`를 넣는다
- `web_fetch`는 link-local / metadata host를 IP lookup까지 거부
- session은 image path만 저장하고 base64는 쓰지 않는다

## 0.9.0 — 2026-08-31

- `read_image`와 프롬프트 `@path.png`가 workdir 이미지를 Chat Completions `image_url`로 붙인다
- png / jpeg / gif / webp / bmp, 파일당 4 MiB, 메시지당 3장. session에는 path만 남긴다

## 0.8.0 — 2026-08-31

Cline / OpenCode / Codex / pi 쪽의 MCP와 서브에이전트 코어.

- stdio MCP 클라이언트 (JSON-RPC 2.0, Content-Length + NDJSON)
- `mcp_servers`는 `~/.config/goppi/config.json`만. 프로젝트 `.goppi.json`은 무시
- MCP tool 이름 `mcp_<server>_<tool>`. plan 모드에서 거부, TTY에서는 먼저 묻는다
- `delegate`: 읽기 전용 서브에이전트 (최대 8턴, MCP/재귀 delegate 없음)
- `goppi mcp [list|tools]`, `/mcp`, `inspect`에 서버 이름만

## 0.7.0 — 2026-08-31

Cline / OpenCode / grok-build / Codex / pi 계열과 같은 에이전트 코어.

- Plan / Act 모드 (`/plan`, `/act`, `--mode`, `GOPPI_MODE`). plan은 write/bash를 거부
- `todo_write` 체크리스트
- `web_fetch` (http/s, HTML 태그 제거, metadata host 거부)
- OpenAI-compatible provider (`--provider openai|compat`, `OPENAI_API_KEY`)
- 프로젝트 skill: `.goppi/skills/<name>/SKILL.md`, `read_skill`, `/skills`
- `/compact`로 긴 session 요약
- 취소된 tool round를 닫아 resume이 깨지지 않게
- write/edit 체크포인트와 `/undo` (Cline checkpoint의 로컬 버전)

## 0.6.8 — 2026-08-31

- 실제 Solar 호출 smoke: `GOPPI_LIVE=1 UPSTAGE_API_KEY=... go test ./internal/provider -run Live`

## 0.6.7 — 2026-08-31

- 취소·에러 뒤에도 session을 저장 (`-c`로 이어갈 수 있게)
- grep 한 줄은 8 KiB에서 자름
- `login --stdin` / `logout` / `init` 테스트, `make check`
- headless JSON usage는 `input_tokens` / `output_tokens` / `reasoning_tokens`
- CI에 staticcheck. tool call은 순차 실행임을 문서화

## 0.6.6 — 2026-08-31

- `goppi doctor`는 API key / workdir / session dir이 깨지면 종료 코드 1
- `write_file` / `edit_file`은 2 MiB 상한, atomic write
- 프로젝트 지시는 32 KiB에서 자름. `max_turns` ≤ 80, `max_tokens` ≤ 131072
- CI에 `govulncheck`. grep/glob이 venv·target 같은 디렉터리를 건너뜀

## 0.6.5 — 2026-08-31

- `install.sh`는 GitHub release 바이너리를 SHA256 확인 후 설치하고, 없으면 `go install`로 넘어감
- CI를 macOS에서도 돌리고 `install.sh` 문법을 검사. workflow는 `contents: read`
- `GOPPI_ALWAYS_APPROVE`는 `1` / `true` / `yes` / `on`
- agent loop가 cancel된 context를 turn·tool 앞에서 끊음
- `Retry-After` HTTP-date, document tool의 workdir 탈출 테스트

## 0.6.4 — 2026-08-31

- 프로젝트 `.goppi.json`은 `always_approve`와 `api_key`를 적용하지 않음 (user config / env / `--always-approve`만)
- `bash`는 `*API_KEY` / `*SECRET` / `*_TOKEN` / AWS key를 child env에서 제거. timeout 상한 300초
- user data dir을 `0700`으로 생성
- `install.sh`가 Go 1.27+를 확인
- release가 SHA256SUMS를 keyless cosign으로 서명
- stream chat 취소 httptest, `.env` gitignore

## 0.6.3 — 2026-08-31

- 파일 tool은 workdir 밖 경로와 symlink 탈출을 거부
- session id는 16자리 hex만 허용 (`-r` / delete / export)
- session 파일은 atomic write + mode `0600`
- HTTP `User-Agent: goppi/<version>`
- 모르는 model은 normalize에서 거부
- `goppi doctor`가 `always_approve`를 표시
- line REPL도 SIGINT로 in-flight request를 취소
- CI에 `gofmt`, `go vet`, `go test -race`
- agent loop 단위 테스트
- HTTP 429/5xx 재시도, `SECURITY.md`, Dependabot
- build에 version ldflags
- tag `v*` release workflow + SHA256SUMS, CLI/repl 테스트
- Solar chat stream httptest
- TUI/REPL에서 always_approve 경고

## 0.6.2 — 2026-08-28

- TUI: `ctrl+c`가 다시 동작함. SIGINT를 모델로 넘기고, Kitty가 `String()=="c"`로 보내는 키도 `ctrl+c`로 인식
- 종료 확인에서 두 번째 `ctrl+c`는 바로 종료

## 0.6.1 — 2026-08-28

문서·제품 카피를 한국형 하네스로 다시 씀.

- 독자 foundation model / sovereign AI에 대응하는 자리로 브랜딩
- 제품 용어(TUI, session, reasoning, headless, slash command)는 영어 유지
- CLI help, TUI 태그, GitHub description을 같은 축으로 맞춤

## 0.6.0 — 2026-08-28

TUI 전면 재설계. 거터(gutter) 기반 트랜스크립트.

- 모든 블록에 1글자 거터 기호(`❯` 나, `●` 고삐, `✻` 생각, `✓`/`✗`/스피너 툴, `·` 시스템) + 행잉 인덴트. 라벨 줄 제거
- 툴은 테두리 박스 대신 한 줄 + 결과 요약. 실행 중엔 트랜스크립트 안에서 스피너
- reasoning은 스트리밍 중 꼬리 3줄, 끝나면 한 줄로 접힘 (`ctrl+o` 토글)
- 중앙 모달 폐지. 권한/종료 확인은 입력창 자리의 패널로 교체
- 모델/effort 선택을 자동완성 목록으로 일원화 (피커 패널 제거, 현재 값 미리 선택, enter 한 번으로 적용)
- 도움말(`?`, `/help`)은 트랜스크립트에 출력
- 헤더 1줄로 축소, 입력창은 1~5줄 자동 성장

## 0.5.3 — 2026-08-28

- README/문서 이미지와 업스테이지 전용 브랜딩 제거
- 고삐·하네스 톤으로 카피·TUI 색(주홍/먹/한지) 정리
- 현재 기본 백엔드는 Upstage Solar로 유지
- 사용자 문서를 한국어로 정리

## 0.5.2 — 2026-08-28

- 제품 문서: README, 유저 가이드, TUI 스크린샷, 워드마크

## 0.5.1 — 2026-08-28

- TUI: `tab` / `shift+tab`으로 슬래시 명령, 모델, effort 완성
- 셸: `goppi completions`가 플래그, 모델, 세션 지원 (zsh/bash/fish)

## 0.5.0 — 2026-08-28

출시형 코딩 에이전트 CLI 스타일의 풀스크린 TUI.

- 마우스 스크롤, 스트리밍 reasoning, 툴 카드가 있는 알트 스크린 채팅
- 권한, 모델, effort 오버레이
- 여러 줄 입력 (`ctrl+j`), 프롬프트 히스토리, 슬래시 명령
- `GOPPI_TUI=0`이면 예전 라인 REPL 유지

## 0.4.0 — 2026-08-28

출시형 코딩 에이전트 CLI에 맞춘 제품 표면.

- `login` / `logout`이 Upstage API 키를 `~/.config/goppi/credentials.json`에 저장
- `models`, `doctor`, `inspect`, `init`, `version`
- 이름 있는 세션: `sessions list|delete`, `-c` / `-r`, `export`
- `GOPPI.md` / `AGENTS.md` 프로젝트 지시
- 헤드리스 `-p`, `--output-format json`, `--always-approve`
- `bash`, `write_file`, `edit_file` 권한 확인
- 기본 `reasoning_effort=medium`으로 툴과 함께 reasoning이 켜지게

## 0.3.0 — 2026-08-28

- SSE로 `delta.reasoning`과 `delta.content` 스트림
- 업스테이지 톤의 터미널 크롬

## 0.2.0 — 2026-08-28

- Solar 채팅 백엔드 (기본 `solar-pro4`)
- Document Parse / OCR 툴

## 0.1.0 — 2026-08-28

- 첫 로컬 에이전트 루프

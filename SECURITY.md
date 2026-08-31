# Security

고삐는 이 머신에서 도는 에이전트 하네스입니다. 파일 tool은 workdir에 가두고, `bash`는 기본으로 workspace-write sandbox입니다.

## 무엇을 가둡니까

- 파일 tool(`read_file`, `write_file`, `edit_file`, `apply_patch`, `glob`, `grep`, `document_*`, `read_image`)은 workdir(그리고 ACP `additionalDirectories`) 밖 경로와 symlink 탈출을 거부합니다. `write_file` / `edit_file` / `apply_patch`는 `.git` 아래(중첩 저장소 포함) 쓰기를 거부합니다. `read_image`는 4 MiB를 넘기면 거부합니다. 프롬프트 `@path` 텍스트 첨도 같은 resolve를 씁니다.
- session id는 16자리 hex만 받습니다. 같은 id는 exclusive lock으로 한 프로세스만 잡고(unix flock, Windows `LockFileEx`), 다른 프로세스는 `already in use`입니다. session JSON·lock·`last` / `last.json`이 symlink면 읽거나 flock하지 않습니다. `sessions/` · `exports/` · `worktrees/` 디렉터리도 symlink면 쓰지 않습니다. `last`는 64바이트를 넘는 파일도 무시합니다.
- `bash`, `write_file`, `edit_file`, `apply_patch`는 TTY에서 실행 전에 묻습니다. `a`는 그 tool만 이번 세션에서 통과합니다. `/new`는 목록을 지웁니다.
- Headless JSON과 비 TTY는 `--always-approve` 없이 해당 tool을 거부합니다.
- API key는 `~/.config/goppi/credentials.json` (mode `0600`)에 둡니다. session 파일도 `0600`입니다. data dir은 `0700`입니다. 그 경로가 symlink면 읽지 않습니다. `goppi login`은 링크를 지우고 일반 파일로 씁니다. `goppi doctor`는 credentials·`config.json`의 `api_key`가 group/other에 열려 있거나 symlink이거나 config/data dir이 world-writable이거나 `sessions/`·`exports/`·`worktrees/`가 symlink이거나 session·export·`last`가 열려 있으면 실패하고, `--fix`가 `0600`/`0700`으로 되돌립니다 (symlink는 되돌리지 않음). `--online`은 저장된 키를 API에 확인합니다. `goppi login`은 저장 전에 API가 키를 받는지 확인하고, 거부되면 파일을 쓰지 않습니다.
- 프로젝트 `.goppi.json`의 `always_approve`, `api_key`, `sandbox`, `mcp_servers`, `lsp_servers`, `worktree`, `hooks`는 읽지 않습니다.
- `hooks` 명령은 user config만. API key를 빼고 15초 안에 돕니다. timeout·취소는 process group을 죽입니다. `pre_tool` 실패는 그 tool을 막습니다. hook stdin JSON·stdout·거절 메시지의 키 패턴은 `[redacted]`입니다. `session_end`는 종료·삭제·`/new`에서도 돕니다. hook은 bash와 같은 sandbox와 `.git` 되돌림을 씁니다.
- `--worktree`는 메인 체크아웃 대신 데이터 디렉터리의 git worktree에서 파일·bash를 돌립니다. 같은 계정의 git 객체에는 접근합니다. `sessions delete`는 그 worktree를 같이 지웁니다.
- `goppi acp`는 에디터가 `session/new` · `session/load`에 넘긴 stdio MCP command를 켭니다 (에디터와 같은 신뢰). HTTP/SSE는 켜지 않습니다. user config 서버와 합칩니다. 쓰기 tool은 `session/request_permission`으로 묻습니다. `session/update`와 `session/request_permission`은 저장 transcript와 같이 키 패턴을 `[redacted]`로 지웁니다. prompt 이미지는 디스크에 쓰지 않고 4 MiB·3장 상한을 넘기면 거부합니다. `resource_link`의 `file://`는 workdir과 `additionalDirectories` 안에서만 읽고, 밖은 내용 없이 URI만 남깁니다. MCP·LSP·hook sandbox 쓰기도 그 extra 루트를 허용합니다. `session/close`와 프로세스 종료는 활성 세션의 tool(MCP·LSP·bash)을 닫습니다. unix에서는 MCP·LSP process group도 같이 죽입니다. `session/delete`와 `/delete`는 JSON·export·worktree를 지웁니다.
- workdir는 있는 디렉터리여야 합니다. `/`와 볼륨 루트(그리고 그쪽으로 가는 symlink)는 거절합니다. 그렇지 않으면 workspace sandbox 쓰기 루트가 파일시스템 전체가 됩니다.
- `bash`의 기본 sandbox(`workspace`)는 workdir, 임시 디렉터리, Go/캐시 경로 밖 쓰기를 거부합니다. macOS는 `sandbox-exec`, Linux는 Landlock입니다. 읽기는 막지 않습니다. `strict`는 여기에 네트워크 jail을 더합니다 (macOS `deny network*`, Linux `CLONE_NEWNET`). 이미 `.git`이 있으면 명령이 끝난 뒤 hooks/config/objects/HEAD/refs/packed-refs 쓰기를 되돌립니다. workspace/strict는 `sudo`/`su`/`doas`와 setuid를 막습니다 (Linux `no_new_privs`, macOS는 그 경로 exec 거부).
- MCP·LSP 서버는 bash와 같은 workspace/strict sandbox로 켭니다 (`goppi mcp tools`, 자동 `gopls` 포함). ACP extra dir이 있으면 그 경로 쓰기도 허용합니다.
- `bash`와 MCP·LSP·git worktree child 프로세스에는 `*API_KEY`, `*SECRET`, `*_TOKEN`, AWS access key가 넘어가지 않습니다. Linux sandbox helper reexec도 같은 `ScrubEnv`를 다시 적용합니다. MCP `env`에 명시한 값은 예외입니다. MCP·LSP stderr는 터미널에 쓰지 않고 8 KiB로 가둡니다.
- MCP tool과 `bash` / 파일 쓰기는 TTY에서 실행 전에 묻습니다. plan 모드는 MCP를 거부합니다.
- `web_fetch`는 file URL, userinfo, localhost, 루프백, RFC1918, CGNAT(100.64/10), metadata/link-local(169.254/16, fe80::)을 거부합니다. hostname이 그 IP로 풀려도 막습니다. 연결 직전 IP를 다시 검사해 DNS rebinding을 막습니다.
- session JSON은 image path만 남기고 data URL은 쓰지 않습니다. tool 결과, 저장 transcript, provider/compact 요청, `/copy`, ACP `session/update` · `session/request_permission` · `session/list` title, 확인 panel, TUI 스크롤백, `--output json`, session title은 env 시크릿 값과 흔한 API key 패턴(`sk-`, `up_`, `ghp_`, `AKIA`…)을 `[redacted]`로 바꿉니다.
- HTTP 응답(8 MiB), SSE 누적(본문·reasoning·tool argument 합 8 MiB), tool input, API key(2 KiB), ACP JSON-RPC 줄(8 MiB)과 헤더(8 KiB), MCP·LSP 줄(4 MiB)은 상한을 넘기면 거부합니다.

## 무엇을 가두지 않습니까

`bash`는 허용되어도 workdir·임시·캐시 밖에는 쓰지 못합니다. `workspace`에서는 읽기와 네트워크(`curl`)를 막지 않습니다. workspace/strict는 `sudo`/`su`/`doas`와 setuid를 막습니다. 이미 `.git`이 있으면 명령이 끝난 뒤 `.git/hooks` · `.git/config` · `.git/objects` · `.git/HEAD` · `.git/refs` · `.git/packed-refs` 쓰기를 되돌립니다 (`sandbox=off` 포함). macOS seatbelt는 그 경로 쓰기를 실행 중에도 거부합니다. `.git`이 없던 곳의 `git init`은 되돌리지 않습니다. `GOPPI_SANDBOX=off` / `--sandbox off` / user config `sandbox`는 그 가둠을 끕니다. `--always-approve` / `--yolo` / `GOPPI_ALWAYS_APPROVE`(`1`/`true`/`yes`/`on`) / user config의 `always_approve`는 실행 전 확인만 끕니다.

신뢰하지 않는 prompt나 공유 머신에서는 `--always-approve`를 쓰지 마세요. `goppi doctor`가 켜져 있으면 표시합니다.

## 키

- `credentials.json`에 key를 두고, 커밋되는 `.goppi.json`에는 넣지 마세요 (넣어도 무시됩니다). 레포에는 `credentials.json`·`*.pem`·`*.key`·`.env`를 커밋하지 마세요 (`.gitignore`에 있음).
- `goppi inspect`와 `goppi doctor`는 key 값이 아니라 source만 보여 줍니다. `login` 성공 메시지도 키를 찍지 않습니다. `goppi login KEY`처럼 인자로 넘기면 거절합니다 (`--stdin` 또는 프롬프트).
- 유출되면 `goppi logout` 후 Upstage console에서 key를 회전하세요.

## Release 서명

`v*` tag release의 `SHA256SUMS`는 Sigstore keyless cosign으로 서명됩니다. 번들은 `SHA256SUMS.sigstore.json`입니다. `install.sh`는 그 번들이 있으면 서명을 강제합니다. `GOPPI_RELEASE_BASE`가 `https://github.com/sspzoa/goppi/releases/`가 아니면 번들 자체도 필수입니다. `GOPPI_SKIP_COSIGN=1`은 로컬 테스트용입니다.

tag 전 로컬 게이트는 `make pre-release`(`release-prepare` + `live-check`)입니다. `make release-prepare`는 CI·release workflow와 동일하고, `make verify-dist`가 tarball·checksum·install smoke를 검증합니다.

```bash
cosign verify-blob --bundle SHA256SUMS.sigstore.json --certificate-identity-regexp 'https://github.com/sspzoa/goppi/' --certificate-oidc-issuer https://token.actions.githubusercontent.com SHA256SUMS
```

## 보고

취약점은 GitHub Security Advisory 또는 repo issue로 알려 주세요. 공개 issue에는 key나 session transcript를 넣지 마세요.

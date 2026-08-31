# GOPPI.md

고삐는 한국형 에이전트 하네스입니다. 바이너리 이름은 `goppi`. 현재 backend는 Upstage Solar입니다.

## 빌드

```
make build
make test
make release-prepare  # CI·release workflow와 동일
make pre-release     # release-prepare + live-check (UPSTAGE_API_KEY 필요)
./bin/goppi
```

## 레이아웃

- `cmd/goppi` — CLI command
- `internal/upstage` — 현재 chat · document HTTP client
- `internal/agent` — tool loop
- `internal/tools` — 파일, bash, 문서, MCP, LSP diagnostics, delegate
- `internal/worktree` — git worktree 격리 (`--worktree`). git child는 API key를 받지 않는다.
- `internal/acp` — Agent Client Protocol v1 (`goppi acp`)
- `internal/rpcio` — ACP·MCP·LSP JSON-RPC 줄 상한
- `internal/mcp` — stdio MCP 클라이언트
- `internal/tui` — 풀스크린 TUI (Charm Bubble Tea v2)
- `internal/complete` — slash · shell completion

Agent, tool, HTTP client는 stdlib. Interactive TUI는 Charm.

## 메모

- 기본 model은 `solar-pro4`, `reasoning_effort=medium`. `--provider openai|compat` 가능.
- `/plan`은 읽기 전용. skill은 `.goppi/skills/<name>/SKILL.md`.
- 긴 session은 `compact_at` input token 또는 context overflow에서 자동 compact. `GOPPI_AUTO_COMPACT=off`로 끔.
- MCP는 `~/.config/goppi/config.json`의 `mcp_servers`와 ACP 에디터가 넘긴 stdio 서버. HTTP/SSE는 건너뛴다. MCP·LSP(자동 `gopls` 포함)는 bash와 같은 sandbox. 종료 때 process group을 죽인다. stderr는 터미널에 쓰지 않는다. `delegate`는 읽기 전용 서브에이전트.
- hooks(`pre_tool` / `post_tool` / `session_start` / `session_end`)도 user config만. `/new`·load·종료·삭제가 end를 돌린다. hook 거절 메시지와 stdin JSON의 키 패턴은 지운다. timeout·취소는 process group을 죽인다. bash와 같은 sandbox·`.git` 되돌림.
- 읽기 tool은 한 배치에서 같이 돈다. 쓰기·bash는 순서로. glob·grep는 workdir와 ACP extra 루트, 하위 `.gitignore`를 따른다. `write_file` / `edit_file` / `apply_patch`는 `.git` 아래를 쓰지 않는다.
- bash sandbox: `workspace`(기본) · `strict`(네트워크 차단) · `off`. workdir는 있는 디렉터리여야 하고 `/`는 거절. 이미 `.git`이 있으면 hooks/config/objects/HEAD/refs/packed-refs 쓰기는 끝난 뒤 되돌림. workspace/strict는 sudo/setuid 거부. Linux helper는 API key를 다시 뺀다.
- 서버는 `bash` `background=true` + `bash_poll` / `bash_kill`. `/jobs`로 목록. `/new`와 세션 load는 그 job을 죽인다.
- TUI에서 생성 중 enter는 다음 메시지를 대기열에 넣는다. 취소하면 버린다.
- 선택이 필요하면 `ask_user`. headless에서는 거부.
- 쓰기 tool은 `y` 한 번, `a` 이번 세션. 파일 쓰기는 바꿀 줄을 보여 준다. `/new`면 목록을 지운다.
- `/diff`는 이번 세션 파일 변경.
- session은 tool이 끝날 때마다 저장한다. id가 있을 때만. 쓰기에 실패하면 턴이 `session save:` 로 끝난다. ACP `session/prompt` · `session/close`도 같다.
- `/sessions <id>`로 이어간다. prefix 가능. `-c`는 `last`가 없으면 가장 최근 세션. 같은 id는 한 프로세스만. `/delete`는 transcript·export·worktree를 지운다. `/export`는 Markdown 파일. `/copy`는 마지막 답. `/retry`는 마지막 프롬프트. `/status`는 토큰. 이어가면 last·Σ도 같이 복원한다.
- 턴이 끝나면 BEL. `GOPPI_NOTIFY=off`로 끔. SIGTERM·SIGHUP은 tool을 닫고 끝낸다. Line REPL 대기 `ctrl+c`도 저장 후 종료.
- 여러 곳 수정은 `apply_patch`.
- `@path`는 workdir 텍스트를 붙인다. 이미지는 `@shot.png` 또는 `read_image`. ACP는 image·embedded resource·resource_link. 에디터는 act/plan·모델, list/close/delete/resume, `additionalDirectories`(파일·glob·grep·`resource_link`·bash·MCP·LSP·hook sandbox). `authenticate`는 키 없이 빈 성공. `session/new`·load·resume의 `cwd`는 절대 경로. load는 transcript를 다시 보내고, resume은 복구만 한다. 같은 세션 prompt가 겹치거나 prompt 중 set_mode이면 busy. close/load/resume/delete는 prompt가 끝날 때까지 기다린다.
- `credentials.json`과 API key는 커밋하지 말 것. `.goppi.json`의 `api_key` / `always_approve`는 무시된다. 모르는 config 키는 Load가 실패한다. credentials·user config·data dir·`sessions/`·`exports/`·`worktrees/`·session JSON·lock·`last` symlink는 읽거나 flock하지 않는다. 열린 키·세션·export 파일은 `goppi doctor --fix`. `login`과 `doctor --online`은 키를 확인한다. `--offline`은 생략. `login`은 키를 인자로 받지 않는다. `login`·`doctor`·`inspect`는 키 값을 찍지 않는다. tool 결과, 저장 transcript, provider 요청, `/copy`, ACP update·permission, 확인 panel, TUI 스크롤백, `--output json`, session title(`session/list`·`/sessions`·export H1)은 키 패턴을 지운다.
- 실제 API smoke: `make live-check` 또는 `GOPPI_LIVE=1 go test ./internal/provider -run Live`
- `GOPPI_TUI=0`이면 풀스크린 대신 line REPL.
- 사용자 문서는 `docs/`. GOPPI.md는 매 turn 읽히니 짧게 유지.

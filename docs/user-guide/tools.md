# Tools

에이전트는 tool로 workdir을 보고 바꿉니다. 호출 시점은 model이 정합니다. Spec은 매 chat turn에 나가고, 결과는 `role=tool` message로 돌아옵니다. env 시크릿 값과 `sk-` / `up_` / `ghp_` / `AKIA` 같은 키 패턴은 결과, 저장 transcript, provider/compact 요청, `/copy`, ACP `session/update` · `session/request_permission` · `session/list` title, 확인 panel, TUI 스크롤백, `--output json`, session title에서 `[redacted]`로 바꿉니다.

[Files and shell](#files-and-shell) ·
[Documents](#documents) ·
[Editor (ACP)](#editor-acp) ·
[Worktree](#worktree) ·
[Plan / Act](#plan--act) ·
[Images](#images) ·
[MCP](#mcp) ·
[Delegate](#delegate) ·
[Skills](#skills) ·
[Hooks](#hooks) ·
[Permissions](#permissions) ·
[What Generate already does](#what-generate-already-does)

## Files and shell

| Tool | 동작 | 먼저 묻는지 |
|------|------|-------------|
| `read_file` | UTF-8 파일, 줄 번호. 긴 파일은 `offset` / `limit` (상한 512 KiB) | 아니요 |
| `write_file` | 만들거나 덮어쓰기 | 예 |
| `edit_file` | `old_string`이 정확히 한 번인 곳만 교체 | 예 |
| `apply_patch` | 여러 파일·여러 hunk (`*** Begin Patch`) | 예 |
| `glob` | pattern으로 경로 찾기. workdir와 ACP 추가 루트. 하위 `.gitignore` 적용 | 아니요 |
| `grep` | 파일 내용 검색. workdir와 ACP 추가 루트. 하위 `.gitignore` 적용 | 아니요 |
| `bash` | workdir에서 shell (git, test, 빌드). `background=true`면 job으로 남김 | 예 |
| `bash_poll` | 백그라운드 job 출력. id 없으면 목록 | 아니요 |
| `bash_kill` | 백그라운드 job과 process group 종료 | 예 |
| `ask_user` | 사용자에게 선택/질문을 던짐 | 아니요 (TTY에서 답 기다림) |
| `todo_write` | session 체크리스트 | 아니요 |
| `web_fetch` | 공개 http(s) URL 텍스트. localhost·사설 IP·userinfo 거부. 연결 직전 IP 재검사 | 아니요 |
| `read_skill` | `.goppi/skills/<name>/SKILL.md` | 아니요 |
| `read_image` | 스크린샷·UI 이미지를 모델에 붙임 | 아니요 |
| `diagnostics` | 언어 서버 진단 (파일 path) | 아니요 |
| `delegate` | 읽기 전용 서브에이전트 (최대 8턴) | 아니요 |
| `mcp_<server>_<tool>` | 신뢰된 stdio MCP 서버 | 예 |

`diagnostics`는 stdio LSP 클라이언트다. Cline / OpenCode / Codex의 언어 서버 진단과 같은 축이다. user config `lsp_servers`로 켠다. 비어 있고 workdir가 Go 모듈이면 `gopls`를 PATH에서 찾는다. `GOPPI_LSP=off`면 시작하지 않는다. 프로젝트 `.goppi.json`은 무시한다. 서버는 최대 4개. plan 모드에서도 읽을 수 있다. LSP stderr는 터미널에 쓰지 않고 8 KiB로 가둔다. 서버는 bash·MCP와 같은 sandbox다.

```json
{
  "lsp_servers": {
    "go": { "command": "gopls", "language": "go" }
  }
}
```

긴 transcript는 `/compact`로 접습니다. 기본은 자동입니다. 직전 turn의 input token이 `compact_at`(기본 100000) 이상이거나 API가 context overflow를 주면 요약한 뒤 같은 일을 이어 갑니다. `GOPPI_AUTO_COMPACT=off`면 수동 `/compact`만 합니다. Cline / OpenCode / Codex의 auto-compact와 같은 축입니다.

쓰기 tool(`bash`, `write_file`, `edit_file`, `apply_patch`, `bash_kill`, `mcp_*`)은 매번 묻습니다. `write_file` / `edit_file` / `apply_patch`는 바꿀 줄을 같이 보여 줍니다. TUI·REPL에서 `a`를 누르면 그 이름만 이번 세션에서 통과합니다. ACP는 `allow-session` (`kind: allow_always`). `/new`와 agent Reset은 목록을 지웁니다. `--always-approve` / `--yolo`는 전역입니다.

`ask_user`는 TTY에서 답을 기다립니다. 옵션이 있으면 번호(1-8)로 고르고, 없으면 예/아니오입니다. `--output-format json`과 비 TTY는 `no user to ask`로 거부합니다. ACP는 `session/request_permission`으로 옵션을 띄웁니다. Cline / Codex / pi의 질문 tool과 같은 축입니다.

`write_file` / `edit_file` / `apply_patch`는 덮기 전 내용을 기억합니다. `/undo`가 마지막 파일 수정을 되돌립니다. `/diff`는 파일마다 첫 스냅샷과 지금을 unified diff로 보여 줍니다. 프로세스 메모리에만 있고, 최대 40개 스냅샷입니다. `/new`는 같이 지웁니다. API가 실패하거나 답을 다시 받으려면 `/retry`가 마지막 user 턴을 지우고 같은 프롬프트를 보냅니다.

`apply_patch`는 Codex 형식입니다. `*** Begin Patch` … `*** End Patch` 안에 `*** Add File:` / `*** Update File:` / `*** Delete File:`과 `@@` hunk를 둡니다. hunk의 old 쪽은 파일에서 한 번만 맞아야 합니다. 여러 곳·여러 파일이면 `edit_file`보다 이쪽을 씁니다.

`edit_file`은 정확히 한 번 맞아야 합니다. 유일하지 않으면 `old_string`을 넓히거나 줄이세요. 경로는 workdir 상대 또는 절대이고, workdir 밖이면 거부됩니다. `..`와 workdir 밖을 가리키는 symlink도 막습니다. `.git` 아래(중첩 저장소 포함) 쓰기는 거부됩니다. `write_file` / `edit_file`은 2 MiB를 넘기면 거부하고, 같은 디렉터리에 atomic rename으로 씁니다.

`bash`는 workdir에서 시작합니다. 기본 sandbox는 `workspace`입니다. workdir, `/tmp`, `$TMPDIR`, `GOCACHE`/`GOMODCACHE`, `~/.cache`, `~/.npm`, `~/Library/Caches` 밖 쓰기는 거부됩니다. 이미 `.git`이 있으면 명령이 끝난 뒤 `.git/hooks` · `.git/config` · `.git/objects` · `.git/HEAD` · `.git/refs` · `.git/packed-refs` 쓰기를 되돌립니다. macOS seatbelt는 그 경로 쓰기를 실행 중에도 거부합니다. 읽기와 네트워크는 그대로입니다. `--sandbox strict` / `GOPPI_SANDBOX=strict`면 네트워크도 막습니다 (macOS seatbelt `deny network*`, Linux network namespace). `go test`·`git pull`·`curl`이 필요하면 `workspace`를 쓰세요. workspace/strict는 `sudo`/`su`/`doas`와 setuid를 막습니다. `GOPPI_SANDBOX=off` 또는 `--sandbox off`면 이 계정 권한으로 돕니다. timeout은 기본 60초, 상한 300초입니다. API key와 token은 child env에서 빼 둡니다. Linux sandbox helper reexec도 다시 뺍니다. 취소·timeout은 process group을 죽여 자식(`sleep` 등)도 같이 끊습니다. 서버는 `background=true`로 켜세요. 그 job은 turn 취소에 죽지 않고, `bash_poll` / `bash_kill`로 보거나 끊습니다. `/jobs`는 출력 없이 한 줄 목록입니다. 최대 4개. `/new`, 세션 load, 프로세스 종료 때 전부 죽입니다. Cline / OpenCode / Codex의 background shell과 같은 축입니다.

## Documents

문서 tool은 현재 backend의 parse/OCR API를 씁니다 (chat과 같은 API key). Upload 상한은 50 MB입니다.

| Tool | 동작 |
|------|------|
| `document_parse` | PDF / HWP / HWPX / DOCX / PPTX / XLSX / TIFF / 이미지 → 레이아웃 있는 Markdown. 문서 기본값. `mode`: `auto`(기본) · `standard` · `enhanced`. `ocr`: `auto`(기본) · `force` |
| `document_ocr` | 레이아웃이 필요 없을 때 순수 텍스트만 |

Binary byte를 추측하거나 `pdftotext`로 돌리기보다 `document_parse`를 쓰세요. Parse / OCR은 chat과 다른 endpoint를 tool로 묶은 것입니다.

파싱 결과는 약 20만 자에서 잘립니다. 스캔이면 `ocr=force`.

## Editor (ACP)

`goppi acp`는 [Agent Client Protocol](https://agentclientprotocol.com/) v1을 stdin/stdout JSON-RPC로 제공합니다. 줄 프레임은 8 MiB, 헤더는 8 KiB를 넘기면 거절합니다. Codex / Zed 쪽 에디터 브리지와 같은 축입니다. `initialize`, `authenticate`, `session/new`, `session/load`, `session/resume`, `session/list`, `session/close`, `session/delete`, `session/prompt`, `session/cancel`, `session/set_mode`, `session/set_config_option`, `session/update`, `session/request_permission`을 구현합니다. `authenticate`는 키를 요구하지 않고 빈 성공을 줍니다. `session/resume`은 load와 같이 세션을 복구하지만 transcript를 `session/update`로 다시 보내지 않습니다. `session/new`의 `cwd`와 load·resume에 넘긴 `cwd`는 절대 경로여야 합니다. load·resume의 `cwd`는 저장된 workdir와 같아야 합니다. 같은 세션의 `session/prompt`가 겹치거나, prompt 중에 `set_mode` / `set_config_option`을 보내면 `busy`입니다. `session/close` · `load` · `resume` · `delete`는 그 세션 prompt를 취소한 뒤 끝날 때까지 기다립니다. transcript 쓰기에 실패하면 `session/prompt`와 `session/close`는 `session save:` 오류입니다. `session/delete`는 transcript·export·worktree를 지웁니다. 세션 생성 때 `modes`(act/plan)와 `configOptions`(mode·model·effort)를 줍니다. `session/load`는 저장된 transcript를 `session/update`로 다시 보낸 뒤 이어갑니다. `session/update`와 `session/request_permission`은 키 패턴을 `[redacted]`로 지웁니다. prompt의 `image` 블록(base64, png/jpeg/gif/webp/bmp, 장당 4 MiB, 최대 3장)을 chat `image_url`로 붙입니다. `resource`(embedded context)와 `resource_link`는 user 텍스트에 `[resource …]`로 붙습니다. `file://` 링크는 workdir과 `additionalDirectories` 안에서만 읽고(파일당 256 KiB, 최대 8개), 밖은 URI만 남깁니다. MCP·LSP·hook sandbox 쓰기도 extra 루트를 허용합니다. session에는 path만 남고 data URL은 쓰지 않습니다. 에디터가 `session/new` · `session/load`에 넘긴 stdio MCP(`command` + `args` + `env`)는 user config 서버와 같이 켭니다. HTTP/SSE transport는 켜지 않습니다. `additionalDirectories`는 cwd 외에 절대 경로 루트를 더한다(최대 8, `/` 불가). 상대 경로는 여전히 cwd. 저장 시 session JSON `extra_dirs`에 남고 load·resume에서 클라이언트가 다시 안 넘겨도 복원된다. 세션을 닫으면 MCP도 같이 죽입니다.

## Worktree

`--worktree` 또는 `GOPPI_WORKTREE=1`이면 현재 git repo에서 브랜치 `goppi/<session>` worktree를 `~/.local/share/goppi/worktrees/` 아래에 만든다. 파일·bash는 그 경로에서만 돈다. 메인 체크아웃은 그대로다. `git` child에는 API key가 넘어가지 않는다. Cline / OpenCode / Codex의 isolated worktree와 같은 축이다. git repo가 아니면 실패한다. `goppi worktree list` / `remove <id>`. 프로젝트 `.goppi.json`은 `worktree`를 켤 수 없다.

## Plan / Act

`/plan` 또는 `--mode plan`이면 에이전트는 읽고 계획만 한다. `write_file` / `edit_file` / `apply_patch` / `bash` / `mcp_*`는 거부된다. `/act`로 실행 모드로 돌아온다. OpenCode의 plan agent, Cline의 Plan/Act와 같은 축이다.

## Images

프롬프트에 `@path`를 쓰면 workdir 파일을 그 턴에 붙입니다. 텍스트는 user 본문 아래 `--- @path ---` 블록(파일당 64 KiB, 최대 4개, UTF-8만). 이미지는 `@shot.png`처럼 `image_url`로 붙습니다. `read_image`는 같은 일을 tool로 합니다. png / jpeg / gif / webp / bmp, 파일당 4 MiB, 메시지당 3장. session JSON에는 path만 남고, 파일이 있으면 resume 때 다시 읽습니다. workdir 밖·바이너리·빈 파일은 건너뜁니다. PDF는 `document_parse`. OpenAI-compatible `image_url` part로 나갑니다.

## MCP

stdio MCP (JSON-RPC 2.0, `Content-Length` 또는 NDJSON). 설정은 `~/.config/goppi/config.json`의 `mcp_servers`와 ACP 에디터 `mcpServers`. 프로젝트 `.goppi.json`은 무시한다. 켜진 tool 이름은 `mcp_<server>_<tool>`이다. 세션을 닫거나 초기화가 실패하면 unix에서 process group을 죽여 `npx` 자식도 남기지 않는다. 서버 stderr는 터미널에 쓰지 않고 8 KiB로 가둔다. Cline / OpenCode / Codex의 MCP와 같은 축이고, 서버는 bash와 같은 sandbox다.

`goppi mcp` · `/mcp`로 목록을 본다.

## Delegate

`delegate`는 같은 모델로 읽기 전용 서브에이전트를 돌린다. plan 모드, 최대 8턴, MCP와 재귀 delegate는 없다. 넓은 탐색이나 두 번째 패스용이다. pi / Codex의 subagent와 같은 축이다.

## Skills

프로젝트 스킬은 workdir `.goppi/skills/<name>/SKILL.md` 또는 `~/.config/goppi/skills/<name>/SKILL.md`. 시스템 프롬프트에 이름이 올라가고, 본문은 `read_skill`로 읽는다.

## Hooks

`~/.config/goppi/config.json`의 `hooks`만 실행한다. 프로젝트 `.goppi.json`은 무시한다. `pre_tool`은 tool 실행 전(확인 panel보다 앞), 비0 종료면 `hook denied`. hook stdin JSON·거절 메시지와 `post_tool` stdout의 키 패턴은 `[redacted]`. `post_tool` stdout은 tool 결과에 붙는다. `session_start`는 에이전트가 켜질 때와 `/new`·세션 load 뒤. `session_end`는 `/new`·load·종료·삭제 때(`reason`: `reset`/`load`/`close`/`delete`). 명령은 bash와 같은 sandbox(`workspace`/`strict`/`off`)로 돌고, 이미 `.git`이 있으면 끝난 뒤 `HEAD`/`refs`/hooks/config/objects 쓰기를 되돌린다. timeout·취소는 process group을 죽인다.

## Permissions

`tools.Dangerous`는 `bash`, `write_file`, `edit_file`, `apply_patch`, 그리고 `mcp_*`입니다.

| 상황 | 동작 |
|------|------|
| TUI | 허용/거부 panel (`y` 한 번, `a` 세션, `n` / `esc` 거부) |
| Line REPL | stderr에 `allow? [y once / a session / N]` |
| `--always-approve` / `--yolo` / `GOPPI_ALWAYS_APPROVE` | 묻지 않음 (`1`/`true`/`yes`/`on`) |
| `--output-format json` 또는 비 TTY | `--always-approve` 없으면 거부 |

한 turn에 읽기 tool만 있으면 같이 실행합니다 (`read_file`, `glob`, `grep`, `web_fetch`, `read_skill`, `diagnostics`, `document_parse`, `document_ocr`). 최대 8개. 쓰기·`bash`·`mcp_*`·`delegate`가 하나라도 있으면 그 배치는 순서로 돕니다. 확인 panel과 취소는 그대로입니다.

거부는 model에 `permission denied: <name>`으로 돌아가서, 그 부작용 없이 이어갈 수 있습니다.

## What Generate already does

Chat Completions는 stream, `reasoning_effort`, tool, `parallel_tool_calls`, `prompt_cache_key`, `max_tokens`를 씁니다. Sampling(`temperature`, `top_p`)과 `response_format`은 API default입니다. Embeddings, classify, information extraction, 비동기 parse는 연결되어 있지 않습니다.

현재 backend: [Generate](https://console.upstage.ai/docs/capabilities/generate), [Document Parse](https://console.upstage.ai/docs/capabilities/document-parse).

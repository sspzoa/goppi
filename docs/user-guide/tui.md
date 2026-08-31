# TUI

TTY에서 `goppi`를 켜면 alt-screen chat이 열립니다. mouse wheel로 transcript를
스크롤합니다. gutter mark로 turn을 구분하고, 권한·종료 확인은 입력창 자리의
panel로 나타납니다. model·effort 선택은 autocomplete list 하나로 처리합니다.

Line REPL로 강제하려면 `GOPPI_TUI=0`.

[Layout](#layout) ·
[Keyboard](#keyboard) ·
[Slash commands](#slash-commands) ·
[Autocomplete](#autocomplete) ·
[Permissions](#permissions) ·
[Streaming](#streaming)

## Layout

| Region | Contents |
|--------|----------|
| Header (1줄) | `고삐` · mode · model · effort · workdir · session id · token 수 · version |
| Transcript | gutter + hanging indent로 block 구분 |
| Input | 1~5줄 자동 성장 prompt (`ctrl+j`로 줄바꿈). 권한·종료 확인 시 panel로 교체. 생성 중에도 입력 가능 |
| Footer | 상태 spinner 또는 key hint |

Gutter:

| Mark | Meaning |
|------|---------|
| `❯` | 내 message |
| `●` | 고삐 답변 |
| `✻` | reasoning. 끝나면 한 줄로 접힘, `ctrl+o`로 펼침 |
| spinner / `✓` / `✗` | tool 실행 중 / 성공 / 실패. 한 줄 + 결과 요약 |
| `·` | system message |

Tool 줄은 이름, 상세(명령·경로), 결과 요약(줄 수 등)을 보여줍니다. 연속된
tool 줄은 빈 줄 없이 붙습니다.

## Keyboard

| Key | Action |
|-----|--------|
| `enter` | 보내기. 생성 중이면 대기열(최대 4). 줄이 slash 접두사(`/mo`)면 먼저 완성 |
| `tab` / `shift+tab` | slash command, model, effort 값 순환 |
| `↑` `↓` | 제안 목록, 또는 prompt history |
| `ctrl+j` | 줄바꿈 |
| `ctrl+c` | 진행 중인 turn 취소하고 대기열도 버림. 대기 중이면 종료 확인. 확인 화면에서 한 번 더 누르면 종료 |
| `ctrl+d` | 입력이 비어 있으면 종료 |
| `ctrl+n` | 새 session (새 id와 prompt-cache key) |
| `ctrl+o` | 접힌 reasoning 펼치기/접기 |
| `ctrl+l` | transcript 맨 아래 |
| `pgup` `pgdn` | 스크롤 |
| `?` | help를 transcript에 출력 (입력이 비어 있을 때) |
| `esc` | 현재 panel 닫기 |

종료(`ctrl+c` 확인 / `ctrl+d` / `/quit` / SIGTERM / SIGHUP)는 메모리에 있는
transcript를 저장한 뒤 MCP·LSP·bash를 닫습니다. SIGTERM·SIGHUP은 TUI
context를 취소하고, 진행 중인 turn이 끝난 뒤 `/quit`과 같이 persist합니다.

첫 `ctrl+c`는 process를 죽이지 **않습니다**. 생성 중이면 turn만 취소하고
쌓아 둔 follow-up도 버립니다. 대기 중이면 종료 확인을 엽니다. 확인에서 다시
`ctrl+c` / `y` / `enter`면 종료입니다. 권한 panel에서 `ctrl+c`는 거부한 뒤
그 turn도 취소합니다.

턴이 끝나거나 API 에러가 나면 터미널 BEL을 냅니다 (다른 창을 보고 있을 때).
취소는 울리지 않습니다. `GOPPI_NOTIFY=off`로 끕니다.

생성 중에 입력한 일반 메시지는 대기열에 들어갑니다. 턴이 끝나면 순서대로
이어서 보냅니다. `/jobs` `/status` `/help` 같은 조회 명령은 생성 중에도
바로 실행됩니다.

`GOPPI_TUI=0` line REPL도 생성 중의 SIGINT는 그 turn만 끊습니다. 대기
prompt의 `ctrl+c`와 `ctrl+d`는 transcript를 저장하고 종료합니다.
SIGTERM·SIGHUP은 stdin을 닫고 종료합니다. Headless `-p`는 프로세스 전체
context를 취소합니다.

## Slash commands

`/`를 치고 `tab`을 누르세요. 주요 command:

| Command | Action |
|---------|--------|
| `/help` | help를 transcript에 출력 (`/?`는 alias) |
| `/plan` `/act` | 읽기 전용 계획 / 실행 |
| `/model [name]` | model 선택. 인자 없이 enter를 치면 목록이 열리고 현재 값이 선택돼 있습니다 |
| `/effort [level]` | effort 선택. `none` … `max`. 인자 없이 치면 목록 |
| `/compact` | 긴 session을 요약으로 접기. input token이 `compact_at`을 넘거나 API가 context overflow를 주면 자동으로도 접는다 |
| `/undo` | 마지막 `write_file` / `edit_file` / `apply_patch` 되돌리기 |
| `/diff` | 이번 세션 파일 변경 (첫 스냅샷 대비 unified diff). 생성 중에도 됨 |
| `/export [id]` | 지금 세션(또는 id)을 `$GOPPI_DATA_DIR/exports/<id>.md`로. 생성 중에도 됨 |
| `/copy` | 마지막 assistant 답을 터미널 클립보드(OSC 52). 생성 중에도 됨 |
| `/retry` | 마지막 user 프롬프트를 다시 보낸다. 실패·취소 다음이나 답을 다시 받을 때 |
| `/jobs` | 백그라운드 bash (`background=true`) 한 줄 목록. 생성 중에도 됨 |
| `/skills` | 프로젝트 skill 목록 |
| `/mcp` | 설정된 MCP 서버와 등록된 `mcp_*` tool |
| `/new` | session 초기화 (`/clear`는 alias) |
| `/tools` | 등록된 tool 목록 |
| `/sessions [id]` | 인자 없이 enter면 목록에서 고른다. id 또는 유일한 prefix로 이어간다 |
| `/delete [id]` | transcript·export·worktree를 지운다. 인자 없으면 지금 세션. 지운 뒤 새 id |
| `/status` | mode, model, sandbox, worktree, compact, jobs, last/Σ tokens, workdir, session id. 이어가면 저장된 토큰을 다시 보여 준다 |
| `/quit` | 종료 (`/exit`, `/q`) |

`/model so` + `tab`은 `/model solar-pro4`가 됩니다. `/effort med` + `tab`은
`/effort medium`.

`/new`는 지금 transcript를 저장한 뒤 새 session id와 `prompt_cache_key`를
만듭니다. 이전 파일은 disk에 남고, 삭제하지는 않습니다. 그 세션에서 켠
백그라운드 bash는 죽입니다. user config `session_end`를 돌린 뒤 새 세션에서
`session_start`를 다시 돌립니다. `/status` last 토큰은 0입니다.

## Autocomplete

TUI와 shell script는 `internal/complete`를 공유합니다. `/` 완성은
`goppi complete slash`와 같은 목록을 봅니다.

- 빈 `/`는 주요 command를 보여 줍니다. alias는 직접 치기 전까지 숨깁니다.
- `/model` + `enter`는 값 목록을 열고 현재 값을 미리 선택합니다. `↑↓`로 고르고 `enter` 한 번이면 적용됩니다.
- 접두사가 덜 끝났을 때 `enter`는 선택한 항목을 넣고, 그 선택으로 줄이 완성되면 바로 실행합니다.

## Permissions

`bash`, `write_file`, `edit_file`, `apply_patch`는 입력창 자리에 허용/거부
panel을 띄웁니다. 쓰기 tool은 바꿀 줄을 같이 보여 줍니다. `y` / `enter`는
이번만, `a`는 이번 세션 동안 그 tool을 다시 묻지 않습니다. `n` / `esc`는
거부. `/new`는 세션 허용 목록을 지웁니다.

Prompt를 건너뛰려면 `--always-approve`(alias `--yolo`) 또는
`GOPPI_ALWAYS_APPROVE=1`. Headless JSON과 비 TTY는 `--always-approve` 없이
해당 tool을 거부합니다. 읽기 전용 tool(`read_file`, `glob`, `grep`,
document parse/OCR)은 묻지 않습니다.

## Streaming

현재 backend는 `stream=true` SSE입니다. `delta.reasoning`은 streaming 중
마지막 3줄만 흐리게 보여주고, turn이 끝나면 한 줄로 접힙니다(`ctrl+o`로
펼침). `delta.content`가 보이는 답입니다. Tool call은 시작할 때 spinner
줄이 생기고 끝나면 `✓`/`✗`와 요약으로 갱신됩니다.

기본 effort는 `medium`이라 tool+stream에서도 reasoning이 켜집니다.
`solar-mini`는 `reasoning_effort`를 보내지 않습니다.
[Configuration](configuration.md#models)을 보세요.

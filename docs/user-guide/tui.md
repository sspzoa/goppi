# TUI

TTY에서 `goppi`를 켜면 alt-screen chat이 열립니다. mouse wheel로 transcript를 스크롤합니다. gutter mark로 turn을 구분하고, 권한·종료 확인은 입력창 자리의 panel로 나타납니다. model·effort 선택은 autocomplete list 하나로 처리합니다.

Line REPL로 강제하려면 `GOPPI_TUI=0`.

[레이아웃](#레이아웃) · [키보드](#키보드) · [Slash command](#slash-command) · [Autocomplete](#autocomplete) · [권한](#권한) · [Streaming](#streaming)

## 레이아웃

| 영역 | 내용 |
|------|------|
| Header (1줄) | `고삐` · `한국형` · model · effort · workdir · session id · token 수 · version |
| Transcript | gutter + hanging indent로 block 구분 |
| Input | 1~5줄 자동 성장 prompt (`ctrl+j`로 줄바꿈). 권한·종료 확인 시 panel로 교체 |
| Footer | 상태 spinner 또는 key hint |

Gutter:

| 기호 | 의미 |
|------|------|
| `❯` | 내 message |
| `●` | 고삐 답변 |
| `✻` | reasoning. 끝나면 한 줄로 접힘, `ctrl+o`로 펼침 |
| spinner / `✓` / `✗` | tool 실행 중 / 성공 / 실패. 한 줄 + 결과 요약 |
| `·` | system message |

Tool 줄은 이름, 상세(명령·경로), 결과 요약(줄 수 등)을 보여줍니다. 연속된 tool 줄은 빈 줄 없이 붙습니다.

## 키보드

| 키 | 동작 |
|----|------|
| `enter` | 보내기. 줄이 slash 접두사(`/mo`)면 먼저 완성 |
| `tab` / `shift+tab` | slash command, model, effort 값 순환 |
| `↑` `↓` | 제안 목록, 또는 prompt history |
| `ctrl+j` | 줄바꿈 |
| `ctrl+c` | 진행 중인 turn 취소. 대기 중이면 종료 확인. 확인 화면에서 한 번 더 누르면 종료 |
| `ctrl+d` | 입력이 비어 있으면 종료 |
| `ctrl+n` | 새 session (새 id와 prompt-cache key) |
| `ctrl+o` | 접힌 reasoning 펼치기/접기 |
| `ctrl+l` | transcript 맨 아래 |
| `pgup` `pgdn` | 스크롤 |
| `?` | help를 transcript에 출력 (입력이 비어 있을 때) |
| `esc` | 현재 panel 닫기 |

첫 `ctrl+c`는 process를 죽이지 **않습니다**. 생성 중이면 turn만 취소하고, 대기 중이면 종료 확인을 엽니다. 확인에서 다시 `ctrl+c` / `y` / `enter`면 종료입니다. Headless `-p`만 signal context를 씁니다.

## Slash command

`/`를 치고 `tab`을 누르세요. 주요 command:

| Command | 동작 |
|---------|------|
| `/help` | help를 transcript에 출력 (`/?`는 alias) |
| `/model [name]` | model 선택. 인자 없이 enter를 치면 목록이 열리고 현재 값이 선택돼 있습니다 |
| `/effort [level]` | effort 선택. `none` … `max`. 인자 없이 치면 목록 |
| `/new` | session 초기화 (`/clear`는 alias) |
| `/tools` | 등록된 tool 목록 |
| `/sessions` | 최근 session |
| `/status` | 현재 model, effort, workdir, session id |
| `/quit` | 종료 (`/exit`, `/q`) |

`/model so` + `tab`은 `/model solar-pro4`가 됩니다. `/effort med` + `tab`은 `/effort medium`.

`/new`는 새 session id와 `prompt_cache_key`를 만듭니다. 이전 transcript는 disk에 남고, 삭제하지는 않습니다.

## Autocomplete

TUI와 shell script는 `internal/complete`를 공유합니다. `/` 완성은 `goppi complete slash`와 같은 목록을 봅니다.

- 빈 `/`는 주요 command를 보여 줍니다. alias는 직접 치기 전까지 숨깁니다.
- `/model` + `enter`는 값 목록을 열고 현재 값을 미리 선택합니다. `↑↓`로 고르고 `enter` 한 번이면 적용됩니다.
- 접두사가 덜 끝났을 때 `enter`는 선택한 항목을 넣고, 그 선택으로 줄이 완성되면 바로 실행합니다.

## 권한

`bash`, `write_file`, `edit_file`은 입력창 자리에 허용/거부 panel을 띄웁니다. `y` / `enter`는 허용, `n` / `esc`는 거부.

Prompt를 건너뛰려면 `--always-approve`(alias `--yolo`) 또는 `GOPPI_ALWAYS_APPROVE=1`. Headless JSON과 비 TTY는 `--always-approve` 없이 해당 tool을 거부합니다. 읽기 전용 tool(`read_file`, `glob`, `grep`, document parse/OCR)은 묻지 않습니다.

## Streaming

현재 backend는 `stream=true` SSE입니다. `delta.reasoning`은 streaming 중 마지막 3줄만 흐리게 보여주고, turn이 끝나면 한 줄로 접힙니다(`ctrl+o`로 펼침). `delta.content`가 보이는 답입니다. Tool call은 시작할 때 spinner 줄이 생기고 끝나면 `✓`/`✗`와 요약으로 갱신됩니다.

기본 effort는 `medium`이라 tool+stream에서도 reasoning이 켜집니다. `solar-mini`는 `reasoning_effort`를 보내지 않습니다. [Configuration](configuration.md#모델)을 보세요.

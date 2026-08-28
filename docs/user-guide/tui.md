# TUI

TTY에서 `goppi`를 켜면 알트 스크린 채팅이 열립니다. 마우스 휠로 트랜스크립트를 스크롤합니다. 스트리밍 reasoning, 툴 카드, 권한 오버레이가 한 화면에 있습니다.

라인 REPL로 강제하려면 `GOPPI_TUI=0`.

[레이아웃](#레이아웃) · [키보드](#키보드) · [슬래시 명령](#슬래시-명령) · [자동완성](#자동완성) · [권한](#권한) · [스트리밍](#스트리밍)

## 레이아웃

| 영역 | 내용 |
|------|------|
| 헤더 | `고삐` · `하네스` · 모델 · effort · workdir · 세션 id · 토큰 수 · 버전 |
| 트랜스크립트 | 사용자 턴, 흐린 reasoning, 답변, 툴 카드 |
| 입력 | 여러 줄 프롬프트 (`ctrl+j`로 줄바꿈) |
| 푸터 | `enter` 보내기 · `/` 명령 · `tab` 완성 · `?` 도움말 |
| 오버레이 | 도움말, 권한, 모델 피커, effort 피커 |

툴 카드는 `running` / `ok` / `fail`과 한 줄 요약(명령, 경로, 줄 수)을 보여줍니다.

## 키보드

| 키 | 동작 |
|----|------|
| `enter` | 보내기. 줄이 슬래시 접두사(`/mo`)면 먼저 완성 |
| `tab` / `shift+tab` | 슬래시 명령, 모델, effort 값 순환 |
| `↑` `↓` | 제안 목록, 또는 프롬프트 히스토리 |
| `ctrl+j` | 줄바꿈 |
| `ctrl+c` | 진행 중인 턴 취소. 대기 중이면 종료 확인 |
| `ctrl+d` | 입력이 비어 있으면 종료 |
| `ctrl+n` | 새 세션 (새 id와 prompt-cache 키) |
| `ctrl+l` | 트랜스크립트 맨 아래 |
| `pgup` `pgdn` | 스크롤 |
| `?` | 도움말 오버레이 (입력이 비어 있을 때) |
| `esc` | 현재 오버레이 닫기 |

첫 `ctrl+c`는 프로세스를 죽이지 **않습니다**. 헤드리스 `-p`만 시그널 컨텍스트를 씁니다. TUI는 쓰지 않아서 실수로 알트 스크린이 깨지지 않습니다.

## 슬래시 명령

`/`를 치고 `tab`을 누르세요. 주요 명령:

| 명령 | 동작 |
|------|------|
| `/help` | 도움말 오버레이 (`/?`는 별칭) |
| `/model [name]` | 모델 피커, 또는 `solar-pro4` / `solar-pro3` / `solar-pro2` / `solar-mini` |
| `/effort [level]` | effort 피커, 또는 `none` … `max` |
| `/new` | 세션 초기화 (`/clear`는 별칭) |
| `/tools` | 등록된 툴 목록 |
| `/sessions` | 최근 세션 |
| `/status` | 모델, effort, workdir, 세션 id |
| `/quit` | 종료 (`/exit`, `/q`) |

`/model so` + `tab`은 `/model solar-pro4`가 됩니다. `/effort med` + `tab`은 `/effort medium`.

`/new`는 새 세션 id와 `prompt_cache_key`를 만듭니다. 이전 트랜스크립트는 디스크에 남고, 삭제하지는 않습니다.

## 자동완성

TUI와 셸 스크립트는 `internal/complete`를 공유합니다. `/` 완성은 `goppi complete slash`와 같은 목록을 봅니다.

- 빈 `/`는 주요 명령을 보여 줍니다. 별칭은 직접 치기 전까지 숨깁니다.
- `/model ` 또는 `/effort ` 다음에는 `tab`으로 값을 고릅니다.
- 접두사가 덜 끝났을 때 `enter`는 먼저 선택한 항목을 넣고, 줄이 준비된 뒤에만 보냅니다.

## 권한

`bash`, `write_file`, `edit_file`은 허용/거부 모달을 엽니다. `y` / `enter`는 허용, `n` / `esc`는 거부.

프롬프트를 건너뛰려면 `--always-approve`(별칭 `--yolo`) 또는 `GOPPI_ALWAYS_APPROVE=1`. 헤드리스 JSON과 비 TTY는 `--always-approve` 없이 해당 툴을 거부합니다. 읽기 전용 툴(`read_file`, `glob`, `grep`, document parse/OCR)은 묻지 않습니다.

## 스트리밍

현재 백엔드는 `stream=true` SSE입니다. `delta.reasoning`은 흐리고 이탤릭, `delta.content`가 보이는 답입니다. 툴 호출은 시작할 때 카드가 생기고 끝나면 갱신됩니다.

기본 effort는 `medium`이라 툴+스트림에서도 reasoning이 켜집니다. `solar-mini`는 `reasoning_effort`를 보내지 않습니다. [설정](configuration.md#모델)을 보세요.

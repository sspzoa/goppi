# CLI

```text
goppi                     인터랙티브 풀스크린 TUI
goppi -p "task"           헤드리스 원샷
goppi "task"              헤드리스 원샷 (위치 인자 == -p)
```

[커맨드](#커맨드) · [플래그](#플래그) · [Inspect](#inspect) · [셸 자동완성](#셸-자동완성) · [내부용](#내부용)

## 커맨드

| 커맨드 | 동작 |
|--------|------|
| `login [--stdin] [key]` | API 키 저장 |
| `logout` | 저장된 키 삭제 |
| `models` | 채팅 모델 목록 (현재 선택 표시) |
| `doctor` | 키, workdir, 세션 디렉터리, 지시 파일 확인 |
| `init` | 없으면 `GOPPI.md` 작성 |
| `inspect [--json]` | 해석된 설정 출력 |
| `sessions` | 세션 목록 (`list`가 기본) |
| `sessions delete <id>` | 세션 삭제 (`rm`은 별칭) |
| `export [id]` | 세션을 Markdown으로 (생략하면 마지막) |
| `completions [zsh\|bash\|fish]` | 자동완성 스크립트 출력 |
| `version` | `goppi <version>` 출력 |
| `help` | 이 화면 (`-h`, `--help`) |

`goppi complete <kind>`는 그 스크립트가 부르는 내부 명령입니다. [내부용](#내부용)을 보세요.

## 플래그

인터랙티브와 헤드리스 에이전트(`goppi` / `goppi -p`)에 공통입니다.

| 플래그 | 동작 |
|--------|------|
| `-p`, `--prompt` | 헤드리스 프롬프트 |
| `-m`, `--model` | `solar-pro4` · `solar-pro3` · `solar-pro2` · `solar-mini` |
| `--effort` | `none` · `minimal` · `low` · `medium` · `high` · `xhigh` · `max` |
| `-C`, `--cwd` | 작업 디렉터리 (절대 경로로 해석) |
| `-c`, `--continue` | 마지막 세션 이어가기 |
| `-r`, `--resume` | 세션 id로 이어가기 |
| `--max-turns` | 에이전트 턴 상한 (기본 30) |
| `--output-format` | `plain` · `json` (기본 `plain`) |
| `--always-approve` | 쓰기/bash 확인 생략 (`--yolo`) |

플래그는 `config.Load()` 이후 설정 파일과 환경 변수를 덮어씁니다. 잘못된 `--effort`나 `--output-format`은 바로 에러입니다.

플래그 뒤 위치 문자열은 `-p`와 같습니다.

```bash
goppi -m solar-pro4 --effort high "테스트 한 개만 돌려"
```

## Inspect

```bash
goppi inspect
goppi inspect --json
```

기본값, 사용자 설정, 프로젝트 `.goppi.json`, env 중 이긴 값을 출력합니다.

```text
  version   0.5.1
  model     solar-pro4
  effort    medium
  base_url  https://api.upstage.ai/v1
  workdir   /Users/you/project
  key       goppi login
  rules     [GOPPI.md]
```

JSON 필드: `version`, `model`, `reasoning_effort`, `base_url`, `workdir`, `max_turns`, `max_tokens`, `key_source`, `has_key`, `instructions`.

## 셸 자동완성

```bash
goppi completions zsh  > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

커맨드, 플래그, 모델, effort, 출력 포맷, 세션 id를 완성합니다 (`goppi sessions delete <tab>`, `goppi -r <tab>`).

zsh는 `~/.zfunc`를 `fpath`에 넣고, 없다면 `autoload -U compinit && compinit`를 하세요.

## 내부용

`goppi complete`는 한 줄에 이름 하나씩 출력합니다. 생성된 스크립트가 동적 값에 이걸 부릅니다.

```text
goppi complete commands
goppi complete models
goppi complete efforts
goppi complete sessions
goppi complete formats
goppi complete shells
goppi complete flags
goppi complete slash
```

두 번째 인자는 접두사 필터입니다. 예: `goppi complete models so`.

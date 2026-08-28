# CLI

```text
goppi                     인터랙티브 풀스크린 TUI
goppi -p "task"           headless one-shot
goppi "task"              headless one-shot (위치 인자 == -p)
```

[Command](#command) · [Flag](#flag) · [Inspect](#inspect) · [Shell completion](#shell-completion) · [내부용](#내부용)

## Command

| Command | 동작 |
|---------|------|
| `login [--stdin] [key]` | API key 저장 |
| `logout` | 저장된 key 삭제 |
| `models` | chat model 목록 (현재 선택 표시) |
| `doctor` | key, workdir, session directory, 지시 파일 확인 |
| `init` | 없으면 `GOPPI.md` 작성 |
| `inspect [--json]` | resolved config 출력 |
| `sessions` | session 목록 (`list`가 기본) |
| `sessions delete <id>` | session 삭제 (`rm`은 alias) |
| `export [id]` | session을 Markdown으로 (생략하면 마지막) |
| `completions [zsh\|bash\|fish]` | completion script 출력 |
| `version` | `goppi <version>` 출력 |
| `help` | 이 화면 (`-h`, `--help`) |

`goppi complete <kind>`는 그 script가 부르는 내부 command입니다. [내부용](#내부용)을 보세요.

## Flag

Interactive와 headless agent(`goppi` / `goppi -p`)에 공통입니다.

| Flag | 동작 |
|------|------|
| `-p`, `--prompt` | headless prompt |
| `-m`, `--model` | `solar-pro4` · `solar-pro3` · `solar-pro2` · `solar-mini` |
| `--effort` | `none` · `minimal` · `low` · `medium` · `high` · `xhigh` · `max` |
| `-C`, `--cwd` | workdir (절대 경로로 해석) |
| `-c`, `--continue` | 마지막 session 이어가기 |
| `-r`, `--resume` | session id로 이어가기 |
| `--max-turns` | agent turn 상한 (기본 30) |
| `--output-format` | `plain` · `json` (기본 `plain`) |
| `--always-approve` | 쓰기/bash 확인 생략 (`--yolo`) |

Flag는 `config.Load()` 이후 config file과 env를 덮어씁니다. 잘못된 `--effort`나 `--output-format`은 바로 에러입니다.

Flag 뒤 위치 문자열은 `-p`와 같습니다.

```bash
goppi -m solar-pro4 --effort high "테스트 한 개만 돌려"
```

## Inspect

```bash
goppi inspect
goppi inspect --json
```

default, user config, 프로젝트 `.goppi.json`, env 중 이긴 값을 출력합니다.

```text
  version   0.6.1
  model     solar-pro4
  effort    medium
  base_url  https://api.upstage.ai/v1
  workdir   /Users/you/project
  key       goppi login
  rules     [GOPPI.md]
```

JSON field: `version`, `model`, `reasoning_effort`, `base_url`, `workdir`, `max_turns`, `max_tokens`, `key_source`, `has_key`, `instructions`.

## Shell completion

```bash
goppi completions zsh  > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

Command, flag, model, effort, output format, session id를 완성합니다 (`goppi sessions delete <tab>`, `goppi -r <tab>`).

zsh는 `~/.zfunc`를 `fpath`에 넣고, 없다면 `autoload -U compinit && compinit`를 하세요.

## 내부용

`goppi complete`는 한 줄에 이름 하나씩 출력합니다. 생성된 script가 동적 값에 이걸 부릅니다.

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

두 번째 인자는 prefix filter입니다. 예: `goppi complete models so`.

# goppi (고삐)

로컬에서 돌아가는 에이전트 하네스 CLI입니다. LLM이 파일을 읽고 고치고, 셸을 실행하는 루프를 직접 붙잡습니다.

의존성 없이 Go 표준 라이브러리만 씁니다. 프로바이더는 Anthropic과 OpenAI-compatible 둘 다 됩니다.

## 설치

```bash
go install github.com/sspzoa/goppi/cmd/goppi@latest
# 또는
make install
```

로컬에서:

```bash
make build
./bin/goppi
```

## 설정

API 키는 환경변수로 넣습니다.

```bash
export ANTHROPIC_API_KEY=sk-ant-...
# 또는 OpenAI / OpenRouter / Ollama 등
export OPENAI_API_KEY=sk-...
export GOPPI_PROVIDER=openai
export GOPPI_BASE_URL=https://openrouter.ai/api/v1
export GOPPI_MODEL=anthropic/claude-sonnet-4.5
```

선택 파일:

- `~/.config/goppi/config.json`
- 프로젝트 `.goppi.json`

```json
{
  "provider": "anthropic",
  "model": "claude-sonnet-4-5",
  "max_turns": 30
}
```

## 사용

```bash
goppi                          # REPL
goppi "이 레포 구조를 설명해줘"   # 한 번 실행
goppi --continue               # 마지막 세션 이어서
goppi --provider openai --model gpt-4.1 -C ./some/dir
```

REPL 명령: `/help` `/model` `/provider` `/tools` `/new` `/quit`

## 툴

| 툴 | 하는 일 |
| --- | --- |
| `read_file` | 줄 번호 붙여 읽기 |
| `write_file` | 파일 생성/덮어쓰기 |
| `edit_file` | 유일한 한 구간만 치환 |
| `glob` | `**` 패턴으로 파일 찾기 |
| `grep` | 정규식으로 내용 검색 |
| `bash` | workdir에서 셸 실행 |

## 레이아웃

```
cmd/goppi          엔트리
internal/agent     툴 루프
internal/provider  Anthropic / OpenAI
internal/tools     파일·셸
internal/repl      REPL과 원샷
internal/config    env + json
internal/session   마지막 대화 저장
```

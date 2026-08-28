# 설정

goppi는 다음 순서로 합칩니다. 내장 기본값, `~/.config/goppi/config.json`, cwd의 `.goppi.json`, 그다음 환경 변수. `goppi`에 넘긴 플래그가 마지막에 이깁니다. `goppi inspect`가 이긴 값을 보여 줍니다.

[기본값](#기본값) · [파일](#파일) · [환경 변수](#환경-변수) · [모델](#모델) · [프로젝트 지시](#프로젝트-지시)

## 기본값

| 키 | 기본값 |
|----|--------|
| `model` | `solar-pro4` |
| `reasoning_effort` | `medium` (`solar-mini`는 필드를 생략) |
| `max_turns` | `30` |
| `max_tokens` | `32768` |
| `base_url` | `https://api.upstage.ai/v1` |
| `workdir` | 현재 디렉터리 (절대 경로로 저장) |

`solar-pro4`는 effort가 `none` 또는 `minimal`이 아니면 reasoning을 합니다. 툴+스트림에서 필드를 빼면 추적이 자주 빠져서, goppi는 기본으로 `medium`을 보냅니다.

## 파일

사용자 설정(선택), `~/.config/goppi/config.json`:

```json
{
  "model": "solar-pro4",
  "reasoning_effort": "medium",
  "max_turns": 30,
  "max_tokens": 32768
}
```

[`config.example.json`](../../config.example.json)을 참고하세요. 프로젝트 덮어쓰기는 작업 디렉터리의 `.goppi.json`(같은 스키마)입니다. 나중 파일이 앞 키를 덮습니다.

지원 키: `api_key`, `base_url`, `model`, `reasoning_effort`, `max_turns`, `workdir`, `max_tokens`, `prompt_cache_key`, `always_approve`.

`goppi login` 자격 증명은 별도 파일 `~/.config/goppi/credentials.json`(`0600`)입니다. 커밋되는 `.goppi.json`에 키를 넣지 마세요.

세션: `~/.local/share/goppi/sessions/<id>.json`과 `last` 포인터. 데이터 루트는 `GOPPI_DATA_DIR`로 바꿉니다. [세션](sessions.md)을 보세요.

## 환경 변수

| 변수 | 동작 |
|------|------|
| `UPSTAGE_API_KEY` | API 키 |
| `GOPPI_API_KEY` | API 키 (대체) |
| `GOPPI_MODEL` / `UPSTAGE_MODEL` | 모델 (둘 다 있으면 `UPSTAGE_MODEL`) |
| `GOPPI_EFFORT` / `UPSTAGE_REASONING_EFFORT` | reasoning effort |
| `GOPPI_BASE_URL` | API 베이스 (끝 슬래시 없어도 됨) |
| `GOPPI_WORKDIR` | 작업 디렉터리 |
| `GOPPI_ALWAYS_APPROVE` | `1`이면 쓰기/bash 확인 생략 |
| `GOPPI_DATA_DIR` | 세션 저장 루트 |
| `GOPPI_TUI` | `0`이면 라인 REPL |
| `NO_COLOR` | 라인 UI에서 ANSI 끄기 |

키 해석은 [인증](authentication.md#해석-순서)에 있습니다.

## 모델

| 모델 | 설명 |
|------|------|
| `solar-pro4` | 기본. 512K 컨텍스트. 끄지 않으면 reasoning |
| `solar-pro3` | 이전 플래그십. reasoning은 `medium` / `high` |
| `solar-pro2` | 이전 세대. reasoning 토큰은 있으나 보이는 추적 없음 |
| `solar-mini` | 빠름. `reasoning_effort`는 지우고 보내지 않음 |

TUI에서는 `/model`, 아니면 `-m` / `GOPPI_MODEL`. `goppi models`가 목록과 현재 선택을 보여 줍니다.

effort 값: `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`. 모르는 값은 normalize에서 실패합니다.

Chat Completions는 스트림, `reasoning_effort`, 툴, `parallel_tool_calls`, `prompt_cache_key`, `max_tokens`를 씁니다. 샘플링(`temperature`, `top_p`)과 `response_format`은 API 기본값입니다. 현재 백엔드: [Generate](https://console.upstage.ai/docs/capabilities/generate).

## 프로젝트 지시

매 턴 workdir에서 이 순서로 읽습니다.

1. `GOPPI.md`
2. `AGENTS.md`
3. `.goppi/instructions.md`

본문을 이어 붙여 시스템 프롬프트의 `Project instructions:` 아래에 넣습니다. 빈 파일은 건너뜁니다.

`goppi init`은 `GOPPI.md` 스텁을 쓰고, 이미 있으면 아무것도 하지 않습니다. 짧게: 이 레포가 무엇인지, 빌드·테스트 방법, 건드리지 말 것. 긴 파일은 매 턴 토큰을 씁니다.

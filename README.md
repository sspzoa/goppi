# goppi (고삐)

[Upstage Solar](https://console.upstage.ai/ko/docs/getting-started)에 맞춘 로컬 에이전트 하네스입니다. 채팅은 `solar-pro4`, 문서는 Document Parse / OCR입니다.

## 시작

1. [console.upstage.ai](https://console.upstage.ai)에서 API 키를 만듭니다.
2. 키를 넣고 실행합니다.

```bash
export UPSTAGE_API_KEY=up_...
make build
./bin/goppi
./bin/goppi "이 레포 구조를 설명해줘"
```

기본 엔드포인트는 `https://api.upstage.ai/v1` 입니다. 콘솔 Getting Started의 OpenAI SDK 예시와 같은 주소입니다.

## 모델

| 모델 | 용도 |
| --- | --- |
| `solar-pro4` | 기본. 에이전트·문서·코딩. reasoning 기본 on, 512K 컨텍스트 |
| `solar-pro3` | 이전 플래그십 |
| `solar-pro2` | 이전 세대 |
| `solar-mini` | 빠른 응답. `reasoning_effort`를 보내면 400 |

```bash
goppi --model solar-pro4 --effort low
goppi --model solar-mini
```

`solar-pro4`는 `reasoning_effort`를 생략하면 reasoning이 켜집니다. 끄려면 `--effort none`.

## 문서

PDF, HWP, HWPX, DOCX, PPTX, XLSX, 이미지(JPEG, PNG, BMP, HEIC, TIFF)는 바이너리로 읽지 말고 툴로 넘기세요.

- `document_parse` — 레이아웃 유지 Markdown. RAG/요약/계약서에 맞음
- `document_ocr` — 좌표 없는 순수 텍스트

## 설정

환경변수 우선:

```bash
export UPSTAGE_API_KEY=up_...
export GOPPI_MODEL=solar-pro4
export GOPPI_EFFORT=low          # 선택
```

또는 `~/.config/goppi/config.json` / 프로젝트 `.goppi.json`:

```json
{
  "model": "solar-pro4",
  "reasoning_effort": "low",
  "max_turns": 30,
  "max_tokens": 32768
}
```

`max_tokens` 기본은 32768입니다. Solar Pro 4는 reasoning 토큰이 `completion_tokens`에 포함되므로 답을 비우지 않게 여유를 둡니다.

세션마다 `prompt_cache_key`를 붙여 멀티턴 캐시를 씁니다. `--continue`는 마지막 키를 이어 받습니다.

## REPL

```
고삐 goppi  0.2.0
  solar    upstage / solar-pro4
  effort   default (solar-pro4는 on)
  workdir  /path

›
```

`/model` `/effort` `/tools` `/new` `/quit`

## 레이아웃

```
cmd/goppi
internal/upstage     Solar Chat + Document Parse/OCR
internal/provider    Chat Completions 매핑
internal/agent       툴 루프
internal/tools       파일·셸·문서
internal/repl
internal/config
internal/session
```

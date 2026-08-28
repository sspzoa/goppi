# Tools

에이전트는 tool로 workdir을 보고 바꿉니다. 호출 시점은 model이 정합니다. Spec은 매 chat turn에 나가고, 결과는 `role=tool` message로 돌아옵니다.

[파일과 shell](#파일과-shell) · [문서](#문서) · [권한](#권한) · [Generate가 이미 하는 일](#generate가-이미-하는-일)

## 파일과 shell

| Tool | 동작 | 먼저 묻는지 |
|------|------|-------------|
| `read_file` | UTF-8 파일, 줄 번호. 긴 파일은 `offset` / `limit` (상한 512 KiB) | 아니요 |
| `write_file` | 만들거나 덮어쓰기 | 예 |
| `edit_file` | `old_string`이 정확히 한 번인 곳만 교체 | 예 |
| `glob` | pattern으로 경로 찾기 | 아니요 |
| `grep` | 파일 내용 검색 | 아니요 |
| `bash` | workdir에서 shell (git, test, 빌드) | 예 |

`edit_file`은 정확히 한 번 맞아야 합니다. 유일하지 않으면 `old_string`을 넓히거나 줄이세요. 경로는 workdir 상대 또는 절대이고, workdir 안에서 해석됩니다.

`bash`는 workdir에서 시작합니다. 오래 사는 server는 여기서 켜지 마세요. System prompt도 같습니다.

## 문서

문서 tool은 현재 backend의 parse/OCR API를 씁니다 (chat과 같은 API key). Upload 상한은 50 MB입니다.

| Tool | 동작 |
|------|------|
| `document_parse` | PDF / HWP / HWPX / DOCX / PPTX / XLSX / TIFF / 이미지 → 레이아웃 있는 Markdown. 문서 기본값. `mode`: `auto`(기본) · `standard` · `enhanced`. `ocr`: `auto`(기본) · `force` |
| `document_ocr` | 레이아웃이 필요 없을 때 순수 텍스트만 |

Binary byte를 추측하거나 `pdftotext`로 돌리기보다 `document_parse`를 쓰세요. Parse / OCR은 chat과 다른 endpoint를 tool로 묶은 것입니다.

파싱 결과는 약 20만 자에서 잘립니다. 스캔이면 `ocr=force`.

## 권한

`tools.Dangerous`는 `bash`, `write_file`, `edit_file`입니다.

| 상황 | 동작 |
|------|------|
| TUI | 허용/거부 panel (`y` / `enter` 허용, `n` / `esc` 거부) |
| Line REPL | stderr에 `allow? [y/N]` |
| `--always-approve` / `--yolo` / `GOPPI_ALWAYS_APPROVE=1` | 묻지 않음 |
| `--output-format json` 또는 비 TTY | `--always-approve` 없으면 거부 |

거부는 model에 `permission denied: <name>`으로 돌아가서, 그 부작용 없이 이어갈 수 있습니다.

## Generate가 이미 하는 일

Chat Completions는 stream, `reasoning_effort`, tool, `parallel_tool_calls`, `prompt_cache_key`, `max_tokens`를 씁니다. Sampling(`temperature`, `top_p`)과 `response_format`은 API default입니다. Embeddings, classify, information extraction, 비동기 parse는 연결되어 있지 않습니다.

현재 backend: [Generate](https://console.upstage.ai/docs/capabilities/generate), [Document Parse](https://console.upstage.ai/docs/capabilities/document-parse).

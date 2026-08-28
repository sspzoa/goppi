# Changelog

## 0.6.1 — 2026-08-28

문서·제품 카피를 한국형 하네스로 다시 씀.

- 독자 foundation model / sovereign AI에 대응하는 자리로 브랜딩
- 제품 용어(TUI, session, reasoning, headless, slash command)는 영어 유지
- CLI help, TUI 태그, GitHub description을 같은 축으로 맞춤

## 0.6.0 — 2026-08-28

TUI 전면 재설계. 거터(gutter) 기반 트랜스크립트.

- 모든 블록에 1글자 거터 기호(`❯` 나, `●` 고삐, `✻` 생각, `✓`/`✗`/스피너 툴, `·` 시스템) + 행잉 인덴트. 라벨 줄 제거
- 툴은 테두리 박스 대신 한 줄 + 결과 요약. 실행 중엔 트랜스크립트 안에서 스피너
- reasoning은 스트리밍 중 꼬리 3줄, 끝나면 한 줄로 접힘 (`ctrl+o` 토글)
- 중앙 모달 폐지. 권한/종료 확인은 입력창 자리의 패널로 교체
- 모델/effort 선택을 자동완성 목록으로 일원화 (피커 패널 제거, 현재 값 미리 선택, enter 한 번으로 적용)
- 도움말(`?`, `/help`)은 트랜스크립트에 출력
- 헤더 1줄로 축소, 입력창은 1~5줄 자동 성장

## 0.5.3 — 2026-08-28

- README/문서 이미지와 업스테이지 전용 브랜딩 제거
- 고삐·하네스 톤으로 카피·TUI 색(주홍/먹/한지) 정리
- 현재 기본 백엔드는 Upstage Solar로 유지
- 사용자 문서를 한국어로 정리

## 0.5.2 — 2026-08-28

- 제품 문서: README, 유저 가이드, TUI 스크린샷, 워드마크

## 0.5.1 — 2026-08-28

- TUI: `tab` / `shift+tab`으로 슬래시 명령, 모델, effort 완성
- 셸: `goppi completions`가 플래그, 모델, 세션 지원 (zsh/bash/fish)

## 0.5.0 — 2026-08-28

출시형 코딩 에이전트 CLI 스타일의 풀스크린 TUI.

- 마우스 스크롤, 스트리밍 reasoning, 툴 카드가 있는 알트 스크린 채팅
- 권한, 모델, effort 오버레이
- 여러 줄 입력 (`ctrl+j`), 프롬프트 히스토리, 슬래시 명령
- `GOPPI_TUI=0`이면 예전 라인 REPL 유지

## 0.4.0 — 2026-08-28

출시형 코딩 에이전트 CLI에 맞춘 제품 표면.

- `login` / `logout`이 Upstage API 키를 `~/.config/goppi/credentials.json`에 저장
- `models`, `doctor`, `inspect`, `init`, `version`
- 이름 있는 세션: `sessions list|delete`, `-c` / `-r`, `export`
- `GOPPI.md` / `AGENTS.md` 프로젝트 지시
- 헤드리스 `-p`, `--output-format json`, `--always-approve`
- `bash`, `write_file`, `edit_file` 권한 확인
- 기본 `reasoning_effort=medium`으로 툴과 함께 reasoning이 켜지게

## 0.3.0 — 2026-08-28

- SSE로 `delta.reasoning`과 `delta.content` 스트림
- 업스테이지 톤의 터미널 크롬

## 0.2.0 — 2026-08-28

- Solar 채팅 백엔드 (기본 `solar-pro4`)
- Document Parse / OCR 툴

## 0.1.0 — 2026-08-28

- 첫 로컬 에이전트 루프

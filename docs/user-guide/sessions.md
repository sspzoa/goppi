# 세션

인터랙티브든 헤드리스든 트랜스크립트를 남겨서 이어갈 수 있습니다. 세션은 JSON 파일과 `-r` / `--resume`에 넘기는 id입니다.

[이어가기](#이어가기) · [내보내기](#내보내기) · [저장](#저장) · [캐시 키](#prompt-cache-키)

## 이어가기

```bash
goppi -c              # 마지막 세션 (`last` 포인터)
goppi -r 7a3f2c18     # id로
goppi sessions        # 목록
goppi sessions delete 7a3f2c18
```

`-c`와 `-r`은 `-p`와 같이 쓸 수 있습니다. 헤드리스 이어가기:

```bash
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

TUI에서는 `/new`(또는 `ctrl+n`)가 새 id와 prompt-cache 키를 만듭니다. `/sessions`는 최근 제목을 보여 줍니다. 이전 파일은 지울 때까지 디스크에 남습니다.

목록 열: id, 로컬 `01-02 15:04`, 제목(첫 사용자 줄, 60 룬으로 자름).

## 내보내기

```bash
goppi export          # 마지막 세션을 Markdown으로
goppi export 7a3f2c18
```

내보내기에 포함되는 것:

- 제목, id, 모델, workdir, 갱신 시각
- user / assistant 턴
- reasoning은 `<details>` 블록
- 툴 호출은 ` ```tool <name> ` 펜스

파일로 남기려면 `goppi export > session.md`.

## 저장

파일은 `~/.local/share/goppi/`(또는 `GOPPI_DATA_DIR`) 아래입니다.

```text
sessions/<id>.json
last                 # 최신 id 포인터 (일반 텍스트)
```

각 JSON은 `id`, `title`, `updated_at`, `workdir`, `model`, `reasoning_effort`, `prompt_cache_key`, `messages`를 담습니다. 예전 `last.json`(이름 붙이기 전)은 첫 `-c`에서 이전합니다.

id는 16자리 hex입니다. 제목은 비어 있지 않은 첫 사용자 메시지입니다.

세션을 지우면 JSON이 삭제됩니다. 그게 `last` 포인터였으면 포인터도 지웁니다.

## Prompt-cache 키

새 세션은 `prompt_cache_key=goppi-<16 hex>`를 받아서, 현재 백엔드가 같은 세션의 접두사를 재사용할 수 있습니다. `/new`는 새 키를 만들고, `-c` / `-r`은 저장된 키를 다시 씁니다.

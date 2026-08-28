# Sessions

Interactive든 headless든 transcript를 남겨서 이어갈 수 있습니다. Session은 JSON 파일과 `-r` / `--resume`에 넘기는 id입니다.

[이어가기](#이어가기) · [Export](#export) · [저장](#저장) · [Prompt-cache key](#prompt-cache-key)

## 이어가기

```bash
goppi -c              # 마지막 session (`last` pointer)
goppi -r 7a3f2c18     # id로
goppi sessions        # 목록
goppi sessions delete 7a3f2c18
```

`-c`와 `-r`은 `-p`와 같이 쓸 수 있습니다. Headless 이어가기:

```bash
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

TUI에서는 `/new`(또는 `ctrl+n`)가 새 id와 prompt-cache key를 만듭니다. `/sessions`는 최근 title을 보여 줍니다. 이전 파일은 지울 때까지 disk에 남습니다.

목록 열: id, 로컬 `01-02 15:04`, title(첫 user 줄, 60 rune으로 자름).

## Export

```bash
goppi export          # 마지막 session을 Markdown으로
goppi export 7a3f2c18
```

Export에 포함되는 것:

- title, id, model, workdir, 갱신 시각
- user / assistant turn
- reasoning은 `<details>` block
- tool call은 ` ```tool <name> ` fence

파일로 남기려면 `goppi export > session.md`.

## 저장

파일은 `~/.local/share/goppi/`(또는 `GOPPI_DATA_DIR`) 아래입니다.

```text
sessions/<id>.json
last                 # 최신 id pointer (plain text)
```

각 JSON은 `id`, `title`, `updated_at`, `workdir`, `model`, `reasoning_effort`, `prompt_cache_key`, `messages`를 담습니다. 예전 `last.json`(이름 붙이기 전)은 첫 `-c`에서 이전합니다.

id는 16자리 hex입니다. Title은 비어 있지 않은 첫 user message입니다.

Session을 지우면 JSON이 삭제됩니다. 그게 `last` pointer였으면 pointer도 지웁니다.

## Prompt-cache key

새 session은 `prompt_cache_key=goppi-<16 hex>`를 받아서, 현재 backend가 같은 session의 prefix를 재사용할 수 있습니다. `/new`는 새 key를 만들고, `-c` / `-r`은 저장된 key를 다시 씁니다.

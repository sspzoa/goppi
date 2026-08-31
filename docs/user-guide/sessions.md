# Sessions

Interactive든 headless든 transcript를 남겨서 이어갈 수 있습니다. Session은
JSON 파일과 `-r` / `--resume`에 넘기는 id입니다. user·assistant·tool
message가 붙을 때마다 atomic으로 씁니다. 턴이 끝나기 전에 프로세스가 죽어도
`-c`로 그 지점부터 이어갑니다. 쓰기에 실패하면 턴은 `session save:` 로
끝나고, 메모리의 답은 남습니다. ACP `session/prompt` · `session/close`도
같은 오류를 돌려줍니다.

[Resume](#resume) ·
[Export](#export) ·
[Storage](#storage) ·
[Prompt-cache key](#prompt-cache-key)

## Resume

```bash
goppi -c              # 마지막 session (`last` pointer)
goppi -r 7a3f2c18     # id 또는 유일한 prefix
goppi sessions        # 목록
goppi sessions delete 7a3f2c18
```

같은 id는 한 프로세스만 잡습니다 (unix flock, Windows `LockFileEx`). 다른
`goppi`가 이미 이어간 세션은 `-c` / `-r` / `/sessions` / ACP `session/load` · `session/resume`가
`already in use`로 거절합니다. lock 파일이 symlink면 잡지 않습니다.
`/new`·종료·삭제가 lock을 풉니다. 프로세스가 죽으면 kernel이 풉니다.

`-c`는 `last` pointer를 봅니다. pointer가 없거나 그 파일이 없거나 symlink면
남은 세션 중 가장 최근 것을 쓰고 pointer를 고칩니다. `../` 같은 잘못된
pointer는 거부합니다. session JSON이 symlink면 `-r`이 거절합니다. `-c`와
`-r`은 `-p`와 같이 쓸 수 있습니다. Headless 이어가기:

```bash
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

에디터 ACP는 `session/list`로 같은 목록을 보고, `session/load`는 transcript를
다시 보내며, `session/resume`은 복구만 합니다. `session/new`의 `cwd`와
load·resume에 넘긴 `cwd`는 절대 경로여야 합니다. load·resume의 `cwd`는 저장된
workdir와 같아야 하고, 없으면 저장된 경로를 씁니다. ACP
`additionalDirectories`는 session JSON `extra_dirs`에 남아 load·resume에서
클라이언트가 다시 안 넘겨도 복원됩니다. `session/close`로 활성
세션의 tool을 닫습니다. `session/delete`는 JSON·export·worktree를 지웁니다.
없는 id는 성공합니다. 활성 세션이면 저장하지 않고 닫은 뒤 지웁니다.

TUI에서는 `/new`(또는 `ctrl+n`)가 지금 transcript를 저장한 뒤 새 id와
prompt-cache key를 만듭니다. `/sessions`는 목록을 고르고,
`/sessions 7a3f2c18`처럼 prefix로 이어갑니다. 지금 transcript는 먼저
저장합니다. `/delete`는 지금 세션(또는 id)을 지우고 새 id를 만듭니다. Line
REPL도 같습니다. 이전 파일은 지울 때까지 disk에 남습니다. 파일은 8 MiB를
넘기면 읽거나 쓰지 않습니다. 깨진 JSON은 목록에서 빠지고, `goppi doctor`는
실패합니다. 파일 이름이 id입니다.

목록 열: id, 로컬 `01-02 15:04`, title(첫 user 줄, 60 rune, 키 패턴은
`[redacted]`).

## Export

```bash
goppi export          # 마지막 session을 Markdown으로 (stdout)
goppi export 7a3f2c18
```

TUI·REPL에서는 `/export`가 지금 세션을 `$GOPPI_DATA_DIR/exports/<id>.md`
(없으면 `~/.local/share/goppi/exports/`)에 씁니다. `/export 7a3f2c18`은 그
id(또는 prefix)입니다. 생성 중에도 됩니다.

Export에 포함되는 것:

- title, id, model, workdir, 갱신 시각, last·Σ 토큰
- user / assistant turn
- reasoning은 `<details>` block
- tool call은 ` ```tool <name> ` fence

파일로 남기려면 `/export` 또는 `goppi export > session.md`. stdout·파일 모두
키 패턴은 `[redacted]`입니다.

## Storage

파일은 `~/.local/share/goppi/`(또는 `GOPPI_DATA_DIR`) 아래입니다.

```text
sessions/<id>.json
exports/<id>.md      # /export
last                 # 최신 id pointer (plain text)
```

각 JSON은 `id`, `title`, `updated_at`, `workdir`, `model`,
`reasoning_effort`, `prompt_cache_key`, `mode`, `todos`, `usage`,
`total_usage`, `messages`를 담습니다. `usage`는 마지막 turn, `total_usage`는
그 세션 합입니다. 이어가면 `/status`와 export가 그 값을 다시 보여 줍니다.
예전 `last.json`(이름 붙이기 전)은 첫 `-c`에서 이전합니다. 쓰기 전 id가
없으면 만들지 않습니다(테스트). TUI·REPL·`-p`는 첫 prompt 전에 id를 고릅니다.

id는 16자리 hex(`^[0-9a-f]{16}$`)입니다. `-r` / `delete` / `export`는 그
형식이 아니면 거부합니다. Title은 비어 있지 않은 첫 user message입니다.

Session을 지우면 JSON과 `exports/<id>.md`가 삭제됩니다. 그게 `last`
pointer였으면 pointer도 지웁니다. 연결된 git worktree도 같이 지웁니다.

## Prompt-cache key

새 session은 `prompt_cache_key=goppi-<16 hex>`를 받아서, 현재 backend가 같은
session의 prefix를 재사용할 수 있습니다. `/new`는 새 key를 만들고, `-c` /
`-r`은 저장된 key를 다시 씁니다.

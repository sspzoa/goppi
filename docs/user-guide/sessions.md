# Sessions

Each interactive or headless run persists the transcript so you can continue it. A session is a JSON file plus an id you pass to `-r` / `--resume`.

[Resume](#resume) · [Export](#export) · [Storage](#storage) · [Cache key](#prompt-cache-key)

## Resume

```bash
goppi -c              # last session (the `last` pointer)
goppi -r 7a3f2c18     # by id
goppi sessions        # list
goppi sessions delete 7a3f2c18
```

`-c` and `-r` work with or without `-p`. Headless continue:

```bash
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

Inside the TUI, `/new` (or `ctrl+n`) starts a fresh id and prompt-cache key. `/sessions` shows recent titles. The previous file stays on disk until you delete it.

Listing columns: id, local `01-02 15:04`, title (first user line, truncated to 60 runes).

## Export

```bash
goppi export          # last session as Markdown
goppi export 7a3f2c18
```

The export includes:

- title, id, model, workdir, updated timestamp
- user / assistant turns
- reasoning in a `<details>` block
- tool calls as ` ```tool <name> ` fences

Redirect to a file when you want a durable note: `goppi export > session.md`.

## Storage

Files live under `~/.local/share/goppi/` (or `GOPPI_DATA_DIR`):

```text
sessions/<id>.json
last                 # pointer to the latest id (plain text)
```

Each JSON file stores `id`, `title`, `updated_at`, `workdir`, `model`, `reasoning_effort`, `prompt_cache_key`, and `messages`. An old `last.json` (pre-named-sessions) is migrated on first `-c`.

Ids are 16 hex characters. Title is the first non-empty user message.

Deleting a session removes its JSON. If it was the `last` pointer, that pointer is removed too.

## Prompt-cache key

New sessions get `prompt_cache_key=goppi-<16 hex>`. Upstage can reuse the prefix across turns in the same session. `/new` mints a new key; `-c` / `-r` reuse the stored one.

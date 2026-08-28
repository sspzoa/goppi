# Tools

The agent inspects and changes the working directory through tools. Solar decides when to call them. Specs go out on every chat turn; results come back as `role=tool` messages.

[Files and shell](#files-and-shell) · [Documents](#documents) · [Permissions](#permissions) · [What Generate already does](#what-generate-already-does)

## Files and shell

| Tool | Action | Asks first |
|------|--------|------------|
| `read_file` | UTF-8 file, numbered lines. `offset` / `limit` for long files (cap 512 KiB) | no |
| `write_file` | Create or overwrite | yes |
| `edit_file` | Replace one exact occurrence of `old_string` | yes |
| `glob` | Find paths by pattern | no |
| `grep` | Search file contents | no |
| `bash` | Shell in the workdir (git, tests, builds) | yes |

`edit_file` must match exactly one occurrence — widen or shrink `old_string` if it is not unique. Paths may be workdir-relative or absolute; they resolve inside the workdir.

`bash` starts in the workdir. Do not start long-lived servers from it. The system prompt says the same.

## Documents

Upstage Document APIs, same API key as chat. Max upload is 50 MB.

| Tool | Action |
|------|--------|
| `document_parse` | PDF / HWP / HWPX / DOCX / PPTX / XLSX / TIFF / images → Markdown with layout. Default for documents. `mode`: `auto` (default) · `standard` · `enhanced`. `ocr`: `auto` (default) · `force` |
| `document_ocr` | Plain text only, when layout does not matter |

Prefer `document_parse` over guessing from binary bytes or piping through `pdftotext`. These are **not** Solar Generate features — they are Parse / OCR Capabilities wired as tools so the chat model can call them.

Parse output is truncated around 200k characters. For scans, set `ocr=force`.

## Permissions

`tools.Dangerous` is `bash`, `write_file`, `edit_file`.

| Context | Behavior |
|---------|----------|
| TUI | Allow/deny modal (`y` / `enter` allow, `n` / `esc` deny) |
| Line REPL | `allow? [y/N]` on stderr |
| `--always-approve` / `--yolo` / `GOPPI_ALWAYS_APPROVE=1` | No ask |
| `--output-format json` or non-TTY | Denied unless `--always-approve` |

A deny returns `permission denied: <name>` to the model so it can continue without that side effect.

## What Generate already does

Chat Completions is used with stream, `reasoning_effort`, tools, `parallel_tool_calls`, `prompt_cache_key`, and `max_tokens`. Sampling (`temperature`, `top_p`) and `response_format` are left at API defaults. Embeddings, classify, information extraction, and async parse are not wired.

See [Generate](https://console.upstage.ai/docs/capabilities/generate) and [Document Parse](https://console.upstage.ai/docs/capabilities/document-parse).

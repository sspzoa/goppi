# TUI

`goppi` on a TTY opens an alt-screen chat. Mouse wheel scrolls the transcript. Streaming reasoning, tool cards, and permission overlays stay in one frame.

![goppi TUI](../assets/tui.png)

Force the old line REPL with `GOPPI_TUI=0`.

[Layout](#layout) · [Keyboard](#keyboard) · [Slash commands](#slash-commands) · [Autocomplete](#autocomplete) · [Permissions](#permissions) · [Streaming](#streaming)

## Layout

| Region | Contents |
|--------|----------|
| Header | `goppi` · model · effort · workdir · session id · token count · version |
| Transcript | User turns, dim reasoning, assistant text, tool cards |
| Input | Multiline prompt (`ctrl+j` for newline) |
| Footer | `enter` send · `/` commands · `tab` complete · `?` help |
| Overlays | Help, permission, model picker, effort picker |

Tool cards show `running` / `ok` / `fail` plus a one-line summary (command, path, or line count).

## Keyboard

| Key | Action |
|-----|--------|
| `enter` | Send. If the line is still a slash prefix (`/mo`), complete first |
| `tab` / `shift+tab` | Cycle slash commands, models, or effort values |
| `↑` `↓` | Move the suggestion list, or walk prompt history |
| `ctrl+j` | Insert a newline |
| `ctrl+c` | Cancel the in-flight turn, or ask to quit when idle |
| `ctrl+d` | Quit when the input is empty |
| `ctrl+n` | New session (new id and prompt-cache key) |
| `ctrl+l` | Jump to the bottom of the transcript |
| `pgup` `pgdn` | Scroll |
| `?` | Help overlay (empty input) |
| `esc` | Close the current overlay |

The first `ctrl+c` does **not** kill the process. Headless `-p` uses a signal context; the TUI does not, so a stray interrupt does not tear down the alt screen.

## Slash commands

Type `/` and press `tab`. Primary commands:

| Command | Action |
|---------|--------|
| `/help` | Help overlay (`/?` is an alias) |
| `/model [name]` | Open the model picker, or set `solar-pro4` / `solar-pro3` / `solar-pro2` / `solar-mini` |
| `/effort [level]` | Open the effort picker, or set `none` … `max` |
| `/new` | Reset the session (`/clear` is an alias) |
| `/tools` | List registered tools |
| `/sessions` | Recent sessions |
| `/status` | Model, effort, workdir, session id |
| `/quit` | Quit (`/exit`, `/q`) |

`/model so` + `tab` becomes `/model solar-pro4`. `/effort med` + `tab` becomes `/effort medium`.

`/new` mints a new session id and `prompt_cache_key`. The previous transcript stays on disk; this does not delete it.

## Autocomplete

The TUI and the shell scripts share `internal/complete`. Completing `/` walks the same catalog as `goppi complete slash`.

- Empty `/` lists primary commands (aliases stay hidden until you type them).
- After `/model ` or `/effort `, `tab` walks values.
- `enter` on an incomplete prefix applies the selected item first, then sends only when the line is ready.

## Permissions

`bash`, `write_file`, and `edit_file` open an allow/deny modal. `y` / `enter` allow. `n` / `esc` deny.

Skip prompts with `--always-approve` (alias `--yolo`) or `GOPPI_ALWAYS_APPROVE=1`. Headless JSON and non-TTY deny those tools unless you pass `--always-approve`. Read-only tools (`read_file`, `glob`, `grep`, document parse/OCR) never ask.

## Streaming

Solar streams SSE with `stream=true`. `delta.reasoning` renders dim and italic; `delta.content` is the visible answer. Tool calls flush as cards when they start and update when they finish.

Default effort is `medium` so reasoning stays on with tools + stream. `solar-mini` omits `reasoning_effort`. See [Configuration](configuration.md#models).

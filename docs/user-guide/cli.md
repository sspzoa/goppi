# CLI

```text
goppi                     Interactive fullscreen TUI
goppi -p "task"           Headless one-shot
goppi "task"              Headless one-shot (positional == -p)
```

[Commands](#commands) · [Flags](#flags) · [Inspect](#inspect) · [Completions](#shell-completions) · [Plumbing](#plumbing)

## Commands

| Command | Action |
|---------|--------|
| `login [--stdin] [key]` | Save an Upstage API key |
| `logout` | Remove the saved key |
| `models` | List Solar chat models (current marked) |
| `doctor` | Check key, workdir, session dir, instructions |
| `init` | Write `GOPPI.md` if missing |
| `inspect [--json]` | Show resolved config |
| `sessions` | List sessions (`list` is the default) |
| `sessions delete <id>` | Delete a session (`rm` is an alias) |
| `export [id]` | Session as Markdown (last if omitted) |
| `completions [zsh\|bash\|fish]` | Print a completion script |
| `version` | Print `goppi <version>` |
| `help` | This surface (`-h`, `--help`) |

`goppi complete <kind>` is plumbing for those scripts. See [Plumbing](#plumbing).

## Flags

These apply to the interactive and headless agent (`goppi` / `goppi -p`):

| Flag | Action |
|------|--------|
| `-p`, `--prompt` | Headless prompt |
| `-m`, `--model` | `solar-pro4` · `solar-pro3` · `solar-pro2` · `solar-mini` |
| `--effort` | `none` · `minimal` · `low` · `medium` · `high` · `xhigh` · `max` |
| `-C`, `--cwd` | Working directory (resolved to an absolute path) |
| `-c`, `--continue` | Resume the last session |
| `-r`, `--resume` | Resume by session id |
| `--max-turns` | Max agent turns (default 30) |
| `--output-format` | `plain` · `json` (default `plain`) |
| `--always-approve` | Skip write/bash prompts (`--yolo`) |

Flags override config files and environment after `config.Load()`. Invalid `--effort` or `--output-format` is a hard error.

A positional string after flags is the same as `-p`:

```bash
goppi -m solar-pro4 --effort high "테스트 한 개만 돌려"
```

## Inspect

```bash
goppi inspect
goppi inspect --json
```

Prints the values that won after defaults, user config, project `.goppi.json`, and env:

```text
  version   0.5.1
  model     solar-pro4
  effort    medium
  base_url  https://api.upstage.ai/v1
  workdir   /Users/you/project
  key       goppi login
  rules     [GOPPI.md]
```

JSON fields: `version`, `model`, `reasoning_effort`, `base_url`, `workdir`, `max_turns`, `max_tokens`, `key_source`, `has_key`, `instructions`.

## Shell completions

```bash
goppi completions zsh  > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

Completes commands, flags, models, effort, output formats, and session ids (`goppi sessions delete <tab>`, `goppi -r <tab>`).

zsh: add `fpath` for `~/.zfunc` and `autoload -U compinit && compinit` if you do not already.

## Plumbing

`goppi complete` prints one name per line. The generated scripts call it for dynamic values.

```text
goppi complete commands
goppi complete models
goppi complete efforts
goppi complete sessions
goppi complete formats
goppi complete shells
goppi complete flags
goppi complete slash
```

An optional second argument is a prefix filter: `goppi complete models so`.

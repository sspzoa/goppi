# goppi (고삐)

Upstage Solar coding agent. Fullscreen TUI, headless for scripts, sessions you can resume.

```bash
curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
goppi login
goppi
```

## Install

```bash
go install github.com/sspzoa/goppi/cmd/goppi@latest
# or
make build && ./bin/goppi
```

Get an API key from [console.upstage.ai](https://console.upstage.ai), then:

```bash
goppi login
# or
export UPSTAGE_API_KEY=up_...
```

## Usage

```bash
goppi                          # fullscreen TUI
goppi -p "이 레포 구조를 설명해줘" # headless
goppi -p "요약해" --output-format json
goppi --always-approve -p "테스트 돌려"

goppi models
goppi doctor
goppi inspect --json
goppi init                     # write GOPPI.md
goppi sessions
goppi -c                       # continue last
goppi -r <id>                  # resume
goppi export                   # last session as Markdown
```

Default model is `solar-pro4` with `reasoning_effort=medium`. Documents go through `document_parse` / `document_ocr`.

`bash`, `write_file`, `edit_file` ask before running unless `--always-approve`.

## Project instructions

The agent reads, in order:

- `GOPPI.md`
- `AGENTS.md`
- `.goppi/instructions.md`

## Completions

```bash
goppi completions zsh > ~/.zfunc/_goppi
goppi completions bash > /usr/local/etc/bash_completion.d/goppi
goppi completions fish > ~/.config/fish/completions/goppi.fish
```

Inside the TUI, `/` then `tab` completes `/model`, `/effort`, and their values.

## Layout

```
cmd/goppi
internal/upstage     Solar Chat + Document Parse/OCR
internal/agent       tool loop
internal/tui         fullscreen chat (mouse, overlays)
internal/tools       files, bash, documents
internal/session     named transcripts
internal/instructions
```

In the TUI: `enter` sends, `tab` completes `/` commands, `ctrl+j` newline, `?` keys, `/model` and `/effort` open pickers. `bash` / writes still ask unless `--always-approve`. `GOPPI_TUI=0` falls back to the line REPL.

AGPL-3.0. See [CHANGELOG.md](CHANGELOG.md).

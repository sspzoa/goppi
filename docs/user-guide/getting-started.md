# Getting started

goppi (고삐) is a local coding agent on [Upstage Solar](https://console.upstage.ai/docs/capabilities/generate). It reads the working tree, edits files, runs shell commands, and parses office documents — interactively in a fullscreen TUI, or headlessly in scripts.

[Install](#install) · [API key](#api-key) · [First session](#first-session) · [Headless](#headless) · [Next](#next)

## Install

Go 1.27+ is required. The installer puts `goppi` on your `PATH` via `go install`:

```bash
curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
goppi version
```

Or build this tree:

```bash
git clone https://github.com/sspzoa/goppi.git
cd goppi
make build
./bin/goppi version
```

`go install github.com/sspzoa/goppi/cmd/goppi@latest` is the same as the installer. Make sure `$(go env GOPATH)/bin` (usually `~/go/bin`) is on `PATH`.

## API key

Create a key at [console.upstage.ai](https://console.upstage.ai). Then either:

```bash
goppi login
```

or:

```bash
export UPSTAGE_API_KEY=up_...
```

`goppi login` writes `~/.config/goppi/credentials.json` with mode `0600`. It is a local key store — not OAuth, and it does not open a browser.

Check the machine with `goppi doctor`. Details: [Authentication](authentication.md).

## First session

```bash
cd your-project
goppi
```

On a TTY this opens the fullscreen TUI. Useful first prompts:

```text
이 레포 구조를 설명해줘
테스트가 어디서 도는지 찾고 한 개만 돌려
README를 현재 코드에 맞게 고쳐
```

PDF / HWP / Office 파일은 에이전트에게 경로를 주면 `document_parse`로 넘깁니다.

Leave with `/quit`, or `ctrl+c` then `enter`.

Project conventions belong in `GOPPI.md` so they load on every turn:

```bash
goppi init
```

See [project instructions](configuration.md#project-instructions).

## Headless

Same agent, no alt screen:

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi -p "요약해" --output-format json
```

Write and bash tools are denied in non-TTY / JSON unless you pass `--always-approve`. See [Headless](headless.md).

## Next

- [TUI keys and slash commands](tui.md)
- [CLI reference](cli.md)
- [Configuration](configuration.md)

# Authentication

goppi talks to `https://api.upstage.ai/v1` with a personal API key. There is no browser login and no OAuth redirect.

[Save a key](#save-a-key) · [Resolution order](#resolution-order) · [Logout](#logout) · [Doctor](#doctor)

## Save a key

Create a key in the [Upstage console](https://console.upstage.ai), then:

```bash
goppi login
```

The command prompts on stdin. Non-interactive options:

```bash
goppi login --stdin          # read the key from stdin
goppi login up_...           # pass the key as an argument (visible in shell history)
export UPSTAGE_API_KEY=up_...
goppi login                  # copies the env var into the credentials file
```

The file is `~/.config/goppi/credentials.json`, mode `0600`:

```json
{
  "api_key": "up_..."
}
```

Do not commit this file. `goppi login` only stores the key; it does not validate it against the API until the next chat request.

## Resolution order

`cfg.ResolveAPIKey()` picks the first non-empty source:

1. `api_key` in `~/.config/goppi/config.json` or `.goppi.json`
2. `UPSTAGE_API_KEY`
3. `GOPPI_API_KEY`
4. `goppi login` credentials file

`goppi inspect` prints which source won (`key_source`). Prefer the env var in CI, and `goppi login` on a laptop.

## Logout

```bash
goppi logout
```

Deletes the credentials file. Environment variables are left alone.

## Doctor

```bash
goppi doctor
```

Checks that a key exists, the workdir is readable, the session directory can be created, and whether `GOPPI.md` / `AGENTS.md` is present. A missing key is the usual first-run failure:

```text
API 키가 없습니다. goppi login 을 실행하거나 UPSTAGE_API_KEY 를 설정하세요
  https://console.upstage.ai
```

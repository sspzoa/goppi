# Authentication

Browser login과 OAuth redirect는 없습니다. `goppi login`이 로컬 API key를
저장합니다. 기본 backend는 Upstage (`https://api.upstage.ai/v1`)입니다.

[Store a key](#store-a-key) ·
[Resolution order](#resolution-order) ·
[Logout](#logout) ·
[Doctor](#doctor)

## Store a key

[Upstage console](https://console.upstage.ai)에서 key를 만든 뒤:

```bash
goppi login
```

명령은 stdin으로 묻습니다. 비대화형 옵션:

```bash
goppi login --stdin          # stdin에서 key를 읽음
goppi login --offline        # API 확인 없이 저장 (공기갭)
# 위치 인자로 키를 넘기면 거절한다 (ps·shell history에 남으므로)
export UPSTAGE_API_KEY=up_...
goppi login                  # env를 credentials 파일로 복사
```

파일은 `~/.config/goppi/credentials.json`, mode `0600`. 그 경로가 symlink면
읽지 않고, `login`은 링크를 지운 뒤 일반 파일로 씁니다.

```json
{
  "api_key": "up_..."
}
```

이 파일은 커밋하지 마세요. `goppi login`은 저장 전에 `GET /models`로 키를
확인하고, 그 경로가 없으면 1토큰 chat으로 확인합니다. 401/403이면 파일을
쓰지 않습니다. 공기갭은 `--offline`. 이미 저장된 키가 깨지면 chat이
`API 401`과 `goppi login`을 안내합니다.

## Resolution order

`cfg.ResolveAPIKey()`는 비어 있지 않은 첫 소스를 고릅니다.

1. `~/.config/goppi/config.json`의 `api_key` (프로젝트 `.goppi.json`의 `api_key`는 무시)
2. `UPSTAGE_API_KEY`
3. `OPENAI_API_KEY`
4. `GOPPI_API_KEY`
5. `goppi login` credentials 파일

`goppi inspect`가 이긴 source(`key_source`)를 출력합니다. CI에서는 env,
노트북에서는 `goppi login`을 쓰면 됩니다.

## Logout

```bash
goppi logout
```

credentials 파일을 지웁니다. env는 그대로 둡니다.

## Doctor

```bash
goppi doctor
```

key 존재(값은 안 찍고 source만), `credentials.json` mode `0600`(symlink면
실패), `config.json`에 `api_key`가 있으면 그 mode, config·data 디렉터리
world-writable·symlink 여부, `sessions/` · `exports/` · `worktrees/`
symlink, `sessions/*.json`·`exports/*.md`·`last` mode, 깨진 session JSON,
workdir 읽기·쓰기, session directory 생성·쓰기를 확인합니다. 이 중
하나라도 실패하면 종료 코드 1입니다. `GOPPI.md` 없음과 `always_approve`는
경고만 합니다. `--fix`는 열린 키·세션·export 파일을 `0600`,
world-writable 디렉터리를 `0700`으로 되돌립니다. symlink는 `--fix`로
지우지 않습니다. `--online`은 저장된 키를 API에 확인합니다.

```text
  ✗ api key  missing — goppi login
doctor: 1 check(s) failed
```

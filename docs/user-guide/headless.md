# Headless

TTY를 가져가지 않습니다. Script, CI, 다른 프로그램에서 쓰세요.

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi "테스트 한 개만 돌려"
goppi -p "요약해" --output-format json
goppi --always-approve -p "go test ./..."
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

위치 문자열은 `-p`와 같습니다. Session은 그대로 `~/.local/share/goppi/sessions/`에 저장되므로 이후에 `-c` / `-r`이 됩니다.

[JSON](#json) · [권한](#권한) · [Signal](#signal) · [CI](#ci)

## JSON

`--output-format json`은 turn이 끝나면 stdout에 object 하나를 출력합니다. 에이전트는 조용합니다 (TUI 없음, stream printer 없음).

```json
{
  "text": "...",
  "reasoning": "...",
  "usage": {
    "InputTokens": 0,
    "OutputTokens": 0,
    "ReasoningTokens": 0
  },
  "session_id": "..."
}
```

`text` / `reasoning`은 마지막 assistant message입니다. 진행·tool log는 나가지 않으니 stdout을 JSON으로 파싱하세요.

`--output-format`은 `plain` 또는 `json`만 됩니다. 그 외는 에러입니다.

## 권한

위험한 tool(`bash`, `write_file`, `edit_file`)은 `--always-approve`(또는 `--yolo` / `GOPPI_ALWAYS_APPROVE=1`)가 없으면 거부됩니다. 파이프된 CI가 `allow? [y/N]`에 멈추지 않게 하려는 동작입니다.

읽기 전용 tool은 그대로 돌아갑니다.

## Signal

`ctrl+c` / SIGINT는 진행 중인 HTTP request를 취소합니다. Headless `-p`는 `signal.NotifyContext`로 감쌉니다. Turn이 정상 끝나면 session이 저장되고, request 중 취소면 마지막 turn이 안 남을 수 있습니다.

Interactive TUI는 그 signal context를 쓰지 **않습니다**. [TUI](tui.md#키보드)를 보세요.

## CI

```yaml
- name: goppi
  env:
    UPSTAGE_API_KEY: ${{ secrets.UPSTAGE_API_KEY }}
  run: |
    goppi --always-approve --output-format json -p "go test ./... 가 어디서 도는지 찾고 한 패키지만 돌려"
```

기본 model이 바뀌면 안 되면 고정하세요. `-m solar-pro4 --effort medium`.

`goppi complete models`와 `goppi complete sessions`는 한 줄에 이름 하나씩 출력합니다. 생성된 zsh/bash/fish script가 동적 값에 이걸 부릅니다. 직접 만든 wrapper에서도 쓸 수 있습니다.

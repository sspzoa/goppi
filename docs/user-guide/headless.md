# 헤드리스

TTY를 가져가지 않습니다. 스크립트, CI, 다른 프로그램에서 쓰세요.

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi "테스트 한 개만 돌려"
goppi -p "요약해" --output-format json
goppi --always-approve -p "go test ./..."
goppi -c -p "이어서, 방금 파일을 커밋 메시지 초안으로"
```

위치 문자열은 `-p`와 같습니다. 세션은 그대로 `~/.local/share/goppi/sessions/`에 저장되므로 이후에 `-c` / `-r`이 됩니다.

[JSON](#json) · [권한](#권한) · [시그널](#시그널) · [CI](#ci)

## JSON

`--output-format json`은 턴이 끝나면 stdout에 객체 하나를 출력합니다. 에이전트는 조용합니다 (TUI 없음, 스트림 프린터 없음).

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

`text` / `reasoning`은 마지막 assistant 메시지입니다. 진행·툴 로그는 나가지 않으니 stdout을 JSON으로 파싱하세요.

`--output-format`은 `plain` 또는 `json`만 됩니다. 그 외는 에러입니다.

## 권한

위험한 툴(`bash`, `write_file`, `edit_file`)은 `--always-approve`(또는 `--yolo` / `GOPPI_ALWAYS_APPROVE=1`)가 없으면 거부됩니다. 파이프된 CI가 `allow? [y/N]`에 멈추지 않게 하려는 동작입니다.

읽기 전용 툴은 그대로 돌아갑니다.

## 시그널

`ctrl+c` / SIGINT는 진행 중인 HTTP 요청을 취소합니다. 헤드리스 `-p`는 `signal.NotifyContext`로 감쌉니다. 턴이 정상 끝나면 세션이 저장되고, 요청 중 취소면 마지막 턴이 안 남을 수 있습니다.

인터랙티브 TUI는 그 시그널 컨텍스트를 쓰지 **않습니다**. [TUI](tui.md#키보드)를 보세요.

## CI

```yaml
- name: goppi
  env:
    UPSTAGE_API_KEY: ${{ secrets.UPSTAGE_API_KEY }}
  run: |
    goppi --always-approve --output-format json -p "go test ./... 가 어디서 도는지 찾고 한 패키지만 돌려"
```

기본 모델이 바뀌면 안 되면 고정하세요. `-m solar-pro4 --effort medium`.

`goppi complete models`와 `goppi complete sessions`는 한 줄에 이름 하나씩 출력합니다. 생성된 zsh/bash/fish 스크립트가 동적 값에 이걸 부릅니다. 직접 만든 래퍼에서도 쓸 수 있습니다.

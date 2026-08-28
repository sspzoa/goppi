# 시작하기

고삐는 에이전트에 고삐를 채우는 로컬 하네스입니다. 작업 트리를 읽고, 파일을 고치고, 셸을 돌리고, 문서를 파싱합니다. 풀스크린 TUI로 쓰거나 스크립트에서 헤드리스로 돌립니다. 바이너리 이름은 `goppi`입니다.

현재 기본 백엔드는 [Upstage Solar](https://console.upstage.ai/docs/capabilities/generate)입니다.

[설치](#설치) · [API 키](#api-키) · [첫 세션](#첫-세션) · [헤드리스](#헤드리스) · [다음](#다음)

## 설치

Go 1.27+가 필요합니다. 설치 스크립트는 `go install`로 `goppi`를 `PATH`에 올립니다.

```bash
curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
goppi version
```

이 트리를 직접 빌드하려면:

```bash
git clone https://github.com/sspzoa/goppi.git
cd goppi
make build
./bin/goppi version
```

`go install github.com/sspzoa/goppi/cmd/goppi@latest`는 설치 스크립트와 같습니다. `$(go env GOPATH)/bin`(보통 `~/go/bin`)이 `PATH`에 있어야 합니다.

## API 키

현재 백엔드인 [console.upstage.ai](https://console.upstage.ai)에서 키를 만든 뒤:

```bash
goppi login
```

또는:

```bash
export UPSTAGE_API_KEY=up_...
```

`goppi login`은 `~/.config/goppi/credentials.json`을 모드 `0600`으로 씁니다. 로컬 키 저장소이며 OAuth가 아니고 브라우저를 열지 않습니다.

머신은 `goppi doctor`로 확인하세요. 자세한 내용은 [인증](authentication.md)입니다.

## 첫 세션

```bash
cd your-project
goppi
```

TTY면 풀스크린 TUI가 열립니다. 처음 물어보기 좋은 것:

```text
이 레포 구조를 설명해줘
테스트가 어디서 도는지 찾고 한 개만 돌려
README를 현재 코드에 맞게 고쳐
```

PDF / HWP / Office 파일은 경로를 주면 `document_parse`로 넘깁니다.

나갈 때는 `/quit`, 또는 `ctrl+c` 다음 `enter`.

프로젝트 규칙은 `GOPPI.md`에 두면 매 턴 읽습니다.

```bash
goppi init
```

[프로젝트 지시](configuration.md#프로젝트-지시)를 보세요.

## 헤드리스

같은 에이전트, 알트 스크린 없음:

```bash
goppi -p "이 레포 구조를 설명해줘"
goppi -p "요약해" --output-format json
```

쓰기·bash 툴은 비 TTY / JSON에서 `--always-approve` 없이 거부됩니다. [헤드리스](headless.md)를 보세요.

## 다음

- [TUI 키와 슬래시 명령](tui.md)
- [CLI 레퍼런스](cli.md)
- [설정](configuration.md)

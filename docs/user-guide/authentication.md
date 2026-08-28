# 인증

브라우저 로그인과 OAuth 리다이렉트는 없습니다. `goppi login`이 로컬 API 키를 저장합니다. 현재 백엔드는 Upstage (`https://api.upstage.ai/v1`)입니다.

[키 저장](#키-저장) · [해석 순서](#해석-순서) · [로그아웃](#로그아웃) · [Doctor](#doctor)

## 키 저장

현재 백엔드인 [Upstage 콘솔](https://console.upstage.ai)에서 키를 만든 뒤:

```bash
goppi login
```

명령은 stdin으로 묻습니다. 비대화형 옵션:

```bash
goppi login --stdin          # stdin에서 키를 읽음
goppi login up_...           # 인자로 전달 (셸 히스토리에 남음)
export UPSTAGE_API_KEY=up_...
goppi login                  # 환경 변수를 credentials 파일로 복사
```

파일은 `~/.config/goppi/credentials.json`, 모드 `0600`:

```json
{
  "api_key": "up_..."
}
```

이 파일은 커밋하지 마세요. `goppi login`은 키만 저장하고, 다음 채팅 요청 전까지 API에 검증하지 않습니다.

## 해석 순서

`cfg.ResolveAPIKey()`는 비어 있지 않은 첫 소스를 고릅니다.

1. `~/.config/goppi/config.json` 또는 `.goppi.json`의 `api_key`
2. `UPSTAGE_API_KEY`
3. `GOPPI_API_KEY`
4. `goppi login` credentials 파일

`goppi inspect`가 이긴 소스(`key_source`)를 출력합니다. CI에서는 환경 변수, 노트북에서는 `goppi login`을 쓰면 됩니다.

## 로그아웃

```bash
goppi logout
```

credentials 파일을 지웁니다. 환경 변수는 그대로 둡니다.

## Doctor

```bash
goppi doctor
```

키 존재, workdir 읽기, 세션 디렉터리 생성, `GOPPI.md` / `AGENTS.md` 여부를 확인합니다. 키가 없으면 보통 이렇게 실패합니다.

```text
API 키가 없습니다. goppi login 을 실행하거나 UPSTAGE_API_KEY 를 설정하세요
  https://console.upstage.ai
```

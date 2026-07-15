# QQQShare

설치 없이 실행하는 임시 LAN 파일 공유 앱입니다. 한 PC에서 QQQShare를 실행하면 같은 Wi-Fi의 PC·Mac·모바일은 브라우저로 접속할 수 있습니다. 기본 공개시간은 1분이며 만료된 파일은 웹에서 즉시 차단된 뒤 비공개 보관함으로 이동합니다.

## 사용자 실행

Windows는 `QQQShare.exe`, macOS는 `QQQShare.app`을 실행하세요. 브라우저가 자동으로 열리며 다음 형태의 주소가 표시됩니다.

```text
192.168.0.28:46327/qqq
```

`주소 복사` 버튼은 임의 접근 토큰이 포함된 전체 URL을 복사합니다. 앱 실행마다 포트와 토큰이 변경됩니다.

데이터 위치:

- Windows: `%LOCALAPPDATA%\QQQShare\Shared`, `%LOCALAPPDATA%\QQQShare\Expired`
- macOS: `~/Library/Application Support/QQQShare/Shared`, `Expired`

## CLI 옵션

### AI/에이전트 MVP

```powershell
QQQShare.exe publish --expires 5m --json .\dist
QQQShare.exe inspect --json "http://192.168.0.28:46327/qqq?t=..."
QQQShare.exe receive --output .\received --json "http://192.168.0.28:46327/qqq?t=..."
```

- `publish`: 파일을 안전한 스테이징 폴더로 복사하고, 폴더는 ZIP으로 만든 뒤 백그라운드 게시
- `inspect`: 다운로드 전 파일명·크기·SHA-256·만료시간 확인
- `receive`: `.part`로 다운로드하고 SHA-256 검증 성공 후 최종 파일로 변경
- `--json`: MCP, Skill, npm/pip 래퍼가 읽을 수 있는 안정적인 JSON 반환

MVP의 `inspect`와 `receive`는 SSRF 방어를 위해 사설 LAN 또는 localhost IP 주소만 허용합니다.

```text
QQQShare --expires 5m --port 4177 --dir D:\QQQShare --max-mb 2048
```

- `--expires`: 기본 공개시간, `1s`~`24h`
- `--port`: 고정 포트. 생략하면 OS가 빈 랜덤 포트를 선택
- `--dir`: 데이터 폴더 재정의
- `--max-mb`: 파일 하나의 업로드 한도
- `--no-open`: 브라우저 자동 실행 안 함
- `--token`: 자동화 테스트용 고정 토큰

## 개발 및 릴리스

프로젝트 로컬 Go 도구체인으로:

```powershell
.tools\go\bin\go.exe test ./...
.\scripts\release.ps1 -Version 0.2.0
```

`dist`에 Windows x64/ARM64 포터블 ZIP과 macOS Intel/Apple Silicon 앱 ZIP, SHA-256 체크섬이 만들어집니다.

macOS 공개 배포는 Apple Developer 인증서가 있는 Mac에서 `scripts/sign-macos.sh`로 코드 서명·공증해야 합니다. 서명하지 않은 테스트 빌드는 Gatekeeper 경고가 표시될 수 있습니다.

## 향후 Claude/MCP

네이티브 API를 바탕으로 아래 MCP 명령을 제공할 예정입니다.

```text
create_share(files, expires_in)
get_share_url(share_id)
list_active_shares()
stop_share(share_id)
```

AI가 파일을 공유하기 전에는 대상 경로, 권한, 만료시간을 사용자에게 확인해야 합니다.

보안 정책은 [SECURITY.md](SECURITY.md)를 참고하세요.

# QQQShare

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg)](CONTRIBUTING.md)

**사람과 AI를 위한 만료형 아티팩트 교환 도구**

Publish. Verify. Expire.

QQQShare는 로컬 파일을 같은 네트워크에서 잠시 받을 수 있는 읽기 전용 URL로 게시합니다. 계정이나 클라우드 업로드 없이 동작하며, 설정한 시간이 지나면 공유가 자동 종료됩니다.

## 포터블 앱 사용법

1. Releases에서 Windows ZIP을 받고 압축을 풉니다.
2. `QQQShare.exe`를 실행합니다.
3. 자동으로 열린 소유자 화면에 파일을 놓습니다.
4. 주소를 복사하거나 QR 코드를 휴대폰으로 스캔합니다.
5. 같은 Wi-Fi의 다른 기기에서 파일을 받습니다.

받는 사람은 조회와 다운로드만 할 수 있습니다. 업로드, 공유 시간 변경, 파일 폐기, 전체 종료는 실행한 PC의 소유자 화면에서만 가능합니다.

> Windows 방화벽 질문이 표시되면 동일한 사설 네트워크에서 사용할 때만 `개인 네트워크`를 허용하세요. 공용 Wi-Fi에서는 사용하지 않는 것을 권장합니다.

## AI·CLI 사용법

```powershell
# 파일 또는 폴더를 5분간 게시 (`ownerUrl`은 외부에 공유하지 마세요)
QQQShare.exe publish --expires 5m --json .\report.pdf .\results

# 다른 기기/에이전트에서 메타데이터 확인
QQQShare.exe inspect --json "http://192.168.0.28:46327/qqq?t=..."

# 다운로드 후 SHA-256 검증
QQQShare.exe receive --output .\received --json "http://192.168.0.28:46327/qqq?t=..."

# 게시 프로세스 즉시 종료
QQQShare.exe revoke --json "http://127.0.0.1:46327/qqq?t=OWNER_TOKEN"
```

`publish --json` 결과 예시:

```json
{
  "schema": "qqqshare-publish/v1",
  "artifactId": "art_example",
  "url": "http://192.168.0.28:46327/qqq?t=READ_ONLY_TOKEN",
  "ownerUrl": "http://127.0.0.1:46327/qqq?t=OWNER_TOKEN",
  "scope": "lan",
  "expiresAt": 1784097000000
}
```

- `url`: 전달 가능한 읽기 전용 URL
- `ownerUrl`: 로컬 폐기용 관리 URL. 채팅이나 로그에 공유하지 마세요.
- `scope`: 현재 MVP는 `lan`만 지원
- 공유 시간: `1s`부터 `24h`
- 폴더: 게시 전에 ZIP으로 묶음
- 수신: 임시 `.part` 파일에 저장하고 SHA-256 검증 후 완료

## 데스크톱 서버 옵션

```text
QQQShare.exe --expires 1m --port 4177 --dir D:\QQQShare --max-mb 2048 --no-open
```

포트를 생략하면 실행할 때마다 빈 랜덤 포트를 사용합니다. 기본 데이터 경로:

- Windows: `%LOCALAPPDATA%\QQQShare\Shared`, `Expired`
- macOS: `~/Library/Application Support/QQQShare/Shared`, `Expired`

## 보안 모델

- 인터넷이 아닌 같은 LAN의 사설 IPv4 주소만 AI 수신 명령에서 허용
- 실행마다 소유자 토큰과 읽기 전용 공유 토큰을 별도로 생성
- API 응답, 페이지, QR 코드 캐시 금지
- 만료 또는 수동 폐기 시 `Expired`로 이동하고 신규 다운로드 차단
- 파일별 SHA-256 제공 및 CLI 수신 검증
- 현재 전송은 HTTP이므로 신뢰할 수 있는 개인 LAN에서만 사용

자세한 내용과 취약점 신고 방법은 [SECURITY.md](SECURITY.md)를 참고하세요.

## 개발 및 릴리스

```powershell
.tools\go\bin\go.exe test ./...
.\scripts\release.ps1 -Version 0.3.0
```

GitHub Actions는 태그를 푸시하면 Windows x64/ARM64와 macOS Intel/Apple Silicon 포터블 ZIP 및 체크섬을 Release에 게시합니다.

## 향후 인터페이스

동일한 QQQShare Core 위에 MCP 서버, Claude/Codex Skill, npm/Python 래퍼를 얇게 제공할 예정입니다. 핵심 도구 계약은 `publish`, `inspect`, `receive`, `revoke`입니다.

## 오픈소스

QQQShare는 [MIT License](LICENSE)로 공개됩니다. 상업적 사용, 수정, 재배포가 가능하며 라이선스와 저작권 고지를 유지해야 합니다. 외부 구성요소의 고지는 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)를 참고하세요.

- 기여 방법: [CONTRIBUTING.md](CONTRIBUTING.md)
- 커뮤니티 행동강령: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- 지원 범위: [SUPPORT.md](SUPPORT.md)
- 보안 신고: [SECURITY.md](SECURITY.md)

# openfortivpn-gmail-passer

[openfortivpn](https://github.com/adrienverge/openfortivpn)를 사용하여 Gmail 2차 인증을 자동화하는 Mac 전용 CLI 툴

---

## 사용법

### 1. openfortivpn을 설치한다.

```
brew install openfortivpn
```

### 2. openfortivpn 설정 파일 생성

`~/.config/gvpn` 디렉토리 아래에 `config` 파일 생성하여 아래 내용 작성

```
host = vpn-gateway (Domain or IP)
port = 8443
username = username
password = password
trusted-cert = 신뢰하는 인증서
```

> [!TIP]
> 만약 `trusted-cert` 값을 모르겠다면 나머지 값만 채운 후 `sudo openfortivpn -c ~/.config/gvpn/config` 를 실행하면 에러 메시지에서 `trusted-cert` 값을 알 수 있다.

### 3. Gmail API 설정 진행

아래 설정을 순차적으로 진행.

- [API 사용 설정](https://developers.google.com/workspace/gmail/api/quickstart/go?hl=ko#enable_the_api)
- [OAuth 동의화면 구설](https://developers.google.com/workspace/gmail/api/quickstart/go?hl=ko#configure_the_oauth_consent_screen)
- [데스크톱 애플리케이션의 사용자 인증 정보 승인](https://developers.google.com/workspace/gmail/api/quickstart/go?hl=ko#authorize_credentials_for_a_desktop_application)

설정을 완료한 후 credentials.json 파일을 다운로드 하여 `~/.config/gvpn/` 아래에 저장

### 4. gvpn 실행

먼저 gvpn 실행파일을 `/usr/local/bin`에 복사한다.

이후 gvpn 바이너리를 sudo 권한으로 실행한다.

```
sudo gvpn
```

### 5. Gmail OAuth 인증 후 Token 생성

실행을 하면 나오는 URL에 접속하여 Gmail 권한을 승인해준다.

승인 후 redirect가 localhost로 되어있으므로 웹 페이지가 나타나지 않을텐데, 이 떼 redirect URL의 Query parameter 중 code 부분을 복사하여 터미널에 붙여넣기 한다.

### 6. 완료

모든 설정이 완료되었다면 자동으로 Gmail에서 2차 인증 번호를 가져온 뒤 openfortivpn을 실행하게 된다.

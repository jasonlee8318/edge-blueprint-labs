# 3회차 — EdgeX Foundry 환경 구성

이 회차의 목표는 **EdgeX Foundry 스택 자체를 띄우고 GUI로 확인하는 법을 익히는 것**입니다.
여기서 띄운 환경은 이후 회차(6강 등)가 계속 재사용합니다.

## 사전 준비

1. Docker Desktop 설치·실행 (메모리 6GB 이상 권장)
2. `docker compose pull` (여러 서비스가 함께 뜨므로 최초 다운로드가 다소 걸립니다)

## 사용하는 파일

`docker-compose.yml`은 EdgeX Foundry 공식 저장소(edgex-compose, v4.0.2 안정 릴리스)의
`docker-compose-no-secty.yml`을 그대로 받아온 것입니다 — 보안(Vault/Kong) 없이
개발/학습용으로 빠르게 띄우는 구성입니다. 직접 수정하지 마세요.

## 실행

```bash
docker compose up -d
docker compose ps
```

포함된 서비스: core-data, core-metadata, core-command, core-keeper(레지스트리),
database(Postgres), mqtt-broker(내부 메시지버스), device-virtual, device-rest,
app-rules-engine, rules-engine, support-notifications, support-scheduler, **ui**.

## EdgeX UI로 확인하기

브라우저에서 http://localhost:4000 접속. GUI로 할 수 있는 것:

- **Devices**: 등록된 디바이스와 최근 상태 확인
- **Events / Readings**: Core Data로 들어온 실제 데이터를 실시간 조회
- **Device Profiles**: 프로필 조회·임포트
- **Add Device**: 새 디바이스를 코드 없이 등록

디바이스가 등록돼 있지 않아도, device-virtual이 기본 제공하는 Random-* 디바이스들이
이미 값을 만들어내고 있습니다 — Events 메뉴에서 데이터가 쌓이는 걸 바로 확인할 수 있습니다.

## 주요 포트

| 서비스 | 포트 | 용도 |
|---|---|---|
| ui | 4000 | 웹 UI |
| core-data | 59880 | 이벤트/리딩 REST API |
| core-metadata | 59881 | 디바이스/프로필 REST API |
| core-command | 59882 | 커맨드 REST API |
| core-keeper | 59890 | 레지스트리·공통 설정 |
| device-virtual | 59900 | 가상 디바이스 서비스 |
| device-rest | 59986 | 외부 REST 푸시 수신 (127.0.0.1만) |

## 자주 겪는 문제

- **일부 컨테이너가 Exited** → `core-common-config-bootstrapper`는 공통 설정을
  keeper에 등록한 뒤 정상적으로 종료됩니다. 이건 정상입니다.
- **UI가 안 열림** → `docker compose logs ui`로 core-metadata/core-data 연결 확인.
  전체 기동에 30초~1분 정도 걸릴 수 있습니다.
- **포트 충돌** → compose 파일에서 해당 서비스의 왼쪽(published) 포트만 변경
- **전체 초기화** → `docker compose down -v`

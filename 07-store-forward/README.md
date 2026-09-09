# 7강 — "Cloud가 10초 동안 끊겼다면 데이터는 어떻게 처리할 것인가?"
## EdgeX 내장 Store & Forward로 구현

원고의 Python 코드는 `local_storage` 리스트와 `retry_local_data()` 함수로
재전송 로직을 직접 구현했습니다. 이번 버전에서는 **그 로직을 손으로 짜지
않습니다** — EdgeX App Functions SDK가 이 패턴을 "Store and Forward"라는
이름으로 이미 내장하고 있기 때문입니다.

```
HTTPSender(url, mimeType, persistOnError=true)
  → 전송 실패 시 SDK가 자동으로 DB(Postgres)에 저장
  → Writable.StoreAndForward.RetryInterval마다 SDK가 자동 재전송
```

즉, "Network Recovery → Store & Forward Start → RETRY SUCCESS" 였던 원고의
로그들이 전부 **SDK가 백그라운드에서 알아서 처리**합니다.

## 시작 전 필수 조건

1. **3회차(`03-edgex-setup`)가 실행 중이어야 합니다.**
2. **Motor01 디바이스가 등록돼 있어야 합니다.** 6강을 이미 진행하셨다면
   그대로 재사용합니다. 처음이라면 `06-stream-pipeline/README.md`의
   1단계(EdgeX UI에서 디바이스 프로필 임포트 + Motor01 등록)를 먼저 하세요.

## 실행

```bash
docker compose up -d --build
```

최초 빌드 시 Go 1.25 툴체인과 모듈을 받느라 시간이 좀 걸릴 수 있습니다.

## 실습 — 네트워크 장애 재현

원고에서는 파이썬 변수(`network_available = False`)로 장애를 흉내 냈지만,
여기서는 **실제로 컨테이너를 멈춰서** 장애를 재현합니다.

```bash
# 1. 정상 동작 확인 — mock-cloud 로그에 데이터가 찍히는지 확인
docker compose logs -f mock-cloud

# 2. (다른 터미널에서) 센서 시뮬레이터 실행
cd injector
pip install requests --break-system-packages   # 최초 1회
python3 sensor_injector.py

# 3. 네트워크 장애 발생 — Cloud 컨테이너를 정지
docker compose stop mock-cloud

# 4. 이 상태로 10초 이상 기다리면서 App Service 로그를 확인
docker compose logs -f edge-store-forward
# HTTPSender 전송 실패 로그가 보이지만, 서비스는 죽지 않고 계속 동작합니다.

# 5. 네트워크 복구
docker compose start mock-cloud

# 6. RetryInterval(10초) 이내에 자동으로 재전송되는 것을 확인
docker compose logs -f mock-cloud
# 장애 중 쌓였던 데이터가 한꺼번에 도착하는 걸 볼 수 있습니다.
```

## 파일 구성

| 파일 | 역할 |
|---|---|
| `app/main.go` | 검증→분류(이상/정상)→압축→전송 파이프라인 |
| `app/res/configuration.yaml` | Store&Forward, Postgres 연동 설정 |
| `mock-cloud/server.py` | 정지 가능한 가짜 클라우드 엔드포인트 |

## 알아두실 점 (정확성/버전 관련)

- 이번 회차는 **App Functions SDK v4.0.2**를 씁니다 (6강의 v3.1.1과 다름).
  이유: Store&Forward 기능이 저장할 DB로 v3 SDK는 **Redis만** 지원하는데,
  3회차 스택은 Redis가 없고 **Postgres**를 씁니다. v4 SDK부터 Postgres
  지원이 추가되어, 이번 회차는 반드시 v4를 써야 스택과 맞습니다.
- v4 SDK는 Go 1.25 이상을 요구합니다(v3는 1.21 이상). Dockerfile의
  빌드 이미지가 `golang:1.25-bookworm`인 이유입니다.
- 이 코드는 EdgeX 공식 SDK 소스(v4.0.2 태그)를 직접 내려받아 타입과
  메서드 시그니처를 확인하며 작성했지만, 이 저장소를 만든 샌드박스
  환경은 Go 1.25 툴체인 설치 자체가 막혀 있어 **최종 `go build` 성공까지는
  확인하지 못했습니다.** `docker compose up --build` 시 정상 인터넷
  환경에서 새로 컴파일되니 문제없이 될 가능성이 높지만, 첫 실행 시
  컴파일 오류가 나면 알려주세요 — 바로 고쳐드리겠습니다.

## 자주 겪는 문제

- **App Service가 시작 직후 죽음** → 로그에서 Postgres 연결 오류 확인.
  `docker compose ps`(03 폴더)로 database 컨테이너가 Up인지 확인
- **재전송이 안 됨** → `Writable.StoreAndForward.Enabled: true`인지,
  `HTTPSender`의 `persistOnError` 인자가 `true`인지 확인(이미 반영되어 있음)
- **전체 초기화** → `docker compose down`(07) → 필요시 03도 `down -v`

## ⚠️ 수정 이력

최초 버전은 Temperature/Vibration이 **한 이벤트 안에 같이 온다고 가정**했지만,
device-rest는 POST 한 번당 Event 한 개(리딩 1개)만 만듭니다. 두 값을 따로
POST하면 절대 한 이벤트에 같이 오지 않아 파이프라인이 항상 멈춰 있었습니다.
지금 버전은 각 리딩의 **최근값을 저장**해뒀다가 매 이벤트마다 최신 쌍으로
판단하도록 고쳤습니다.

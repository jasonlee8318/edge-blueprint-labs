# 6강 — 실시간 스트림 데이터 처리 / 전처리·캐싱 구조 설계
## 진짜 EdgeX Foundry 위에서 실행하는 버전

Sensor → Buffer → Window Processing → Event Detection → Cache 파이프라인을
**Go로 작성한 EdgeX App Service**로 구현합니다. Motor01이라는 가상 센서가
device-rest를 통해 실제 EdgeX Core Data에 온도/진동 값을 보내고, 우리 App
Service가 Core Data → Message Bus를 거쳐 이 이벤트를 실시간으로 받아 처리합니다.

## 시작 전 필수 조건

**3회차(`03-edgex-setup`)가 먼저 실행 중이어야 합니다.**

```bash
cd ../03-edgex-setup
docker compose up -d
docker compose ps   # 모든 서비스가 떠 있는지 확인
```

## 1단계 — Motor01 디바이스 등록 (EdgeX UI, GUI로 진행)

1. 브라우저에서 http://localhost:4000 접속 (EdgeX UI)
2. **Device Profiles → Import** → `device-profile/motor-vibration-sensor.yaml` 업로드
3. **Devices → Add Device**
   - Name: `Motor01`
   - Profile: `Motor-Vibration-Sensor`
   - Service: `device-rest`
   - Protocol: `other` (빈 값으로 두면 됩니다)
4. Devices 목록에 Motor01이 보이면 완료

## 2단계 — App Service(우리 파이프라인) 기동

```bash
docker compose up -d --build
```

최초 빌드 시 Go 모듈을 내려받느라 1~2분 걸릴 수 있습니다.
`docker compose logs -f edge-stream-pipeline`으로 "edge-stream-pipeline 시작..." 로그를 확인하세요.

## 3단계 — 센서 시뮬레이터 실행

```bash
cd injector
pip install requests --break-system-packages   # 최초 1회
python3 sensor_injector.py
```

1초마다 Motor01의 Temperature/Vibration 값을 device-rest로 밀어 넣습니다.
EdgeX UI의 **Events** 메뉴에서 실제로 Core Data에 쌓이는 걸 확인할 수 있습니다.

## 4단계 — 파이프라인 결과 확인 (Dashboard 역할)

```bash
curl http://localhost:59750/dashboard
```

```json
{"status":"ANOMALY","avg_temperature":81.2,"avg_vibration":5.4,"age_seconds":0.8,"stale":false}
```

- `status`: WAITING(5건 미만) / NORMAL / ANOMALY
- `stale`: TTL(5초)을 넘겨 오래된 결과인지 여부 — 인젝터를 잠시 멈추고
  다시 curl 해보면 `stale: true`로 바뀌는 걸 확인할 수 있습니다 (2-4강 TTL 개념)

## 파일 구성

| 파일 | 역할 |
|---|---|
| `app/main.go` | 파이프라인 로직 (App Functions SDK, Go) |
| `app/res/configuration.yaml` | App Service 설정 |
| `app/Dockerfile` | 멀티 스테이지 빌드 |
| `device-profile/motor-vibration-sensor.yaml` | Motor01 디바이스 프로필 |
| `injector/sensor_injector.py` | 센서 시뮬레이터 (REST로 값 주입) |

## 알아두실 점 (정확성 관련)

- App Functions SDK는 **v3.1.1**을 사용합니다. 03에서 띄우는 EdgeX 스택 이미지(v4.0.2)보다
  한 메이저 버전 낮은데, 이 저장소를 만든 샌드박스 환경의 Go 버전(1.22) 제약 때문입니다.
  v4 SDK는 Go 1.25 이상을 요구합니다. Event/Reading의 JSON 스키마는 v3-v4 간 호환되므로
  동작에는 문제가 없지만, 만약 Go 1.25 이상을 쓰실 수 있다면 `app/go.mod`에서
  `app-functions-sdk-go/v4 v4.0.2`로 올려서 스택 버전과 맞추셔도 됩니다.
- `go.sum`은 저장소에 포함하지 않았습니다. `docker compose up --build` 시점에
  Dockerfile 안에서 `go mod tidy`가 새로 받습니다(정상 인터넷 환경 필요).

## 자주 겪는 문제

- **App Service가 계속 재시작됨** → `docker compose logs edge-stream-pipeline`에서
  `keeper.http://edgex-core-keeper:59890` 연결 오류가 보이면 03 스택이 완전히
  기동되기 전입니다. `docker compose ps`(03 폴더에서)로 core-keeper가 Up인지 확인 후 재시도
- **Motor01 이벤트가 안 들어옴** → 1단계에서 디바이스 등록을 건너뛰었을 가능성.
  EdgeX UI의 Devices 메뉴에서 Motor01이 있는지, injector 로그에 200 응답이 오는지 확인
- **네트워크 연결 실패(`edgex_edgex-network not found`)** → 03을 먼저 `up -d` 하지 않은 경우
- **전체 초기화** → `docker compose down`(06에서) → `docker compose down -v`(03에서)

## ⚠️ 수정 이력

최초 버전은 Temperature/Vibration이 **한 이벤트 안에 같이 온다고 가정**했지만,
device-rest는 POST 한 번당 Event 한 개(리딩 1개)만 만듭니다. 두 값을 따로
POST하면 절대 한 이벤트에 같이 오지 않아 파이프라인이 항상 멈춰 있었습니다.
지금 버전은 각 리딩의 **최근값을 저장**해뒀다가 매 이벤트마다 최신 쌍으로
판단하도록 고쳤습니다.

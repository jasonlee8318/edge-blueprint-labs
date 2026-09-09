# 4강 — 엣지 플랫폼 연동 설계 (MQTT/REST)

디바이스가 플랫폼(EdgeX)에 연결되는 방식이 REST 하나만 있는 게 아니라는
걸 확인하는 회차입니다. 6강에서는 `device-rest`로 REST 연동을 했는데,
이번엔 같은 역할을 하는 **`device-mqtt`**로 MQTT 연동을 추가합니다.

## 실제로 검증한 내용

`device-mqtt-go`의 실제 소스(`onIncomingDataReceived`)를 확인해
토픽 규칙을 정확히 맞췄고, 로컬 MQTT 브로커로 injector 스크립트가
실제로 올바른 토픽·페이로드를 발행하는 것까지 확인했습니다.

- 토픽 형식: `incoming/data/<디바이스명>/<리소스명>`
- 페이로드: 리소스가 1개뿐이면 순수 텍스트 값 (JSON 아님)

## 시작 전 필수 조건

**3회차(`03-edgex-setup`)가 먼저 실행 중이어야 합니다.**

```bash
cd ../03-edgex-setup
docker compose up -d
```

## 1단계 — device-mqtt 서비스 기동

```bash
docker compose up -d
```

3회차 스택에는 mqtt-broker(브로커 자체)는 있지만, 그 브로커로 들어오는
데이터를 EdgeX Event로 변환해주는 `device-mqtt` 서비스는 없어서
이번 회차에서 추가합니다.

## 2단계 — MotorMQTT01 디바이스 등록 (EdgeX UI)

1. http://localhost:4000 접속
2. **Device Profiles → Import** → `device-profile/motor-mqtt-sensor.yaml` 업로드
   (제출 후 목록에서 `Motor-MQTT-Sensor`가 보이면 성공 — `linkedDeviceCount: 0`은
   "프로필만 등록됐고 아직 이걸 쓰는 디바이스는 없다"는 정상 상태입니다)
3. **Devices → Add Device** 클릭 → **Add Device Wizard**가 5단계로 진행됩니다

| 단계 | 화면에서 할 일 |
|---|---|
| ① SelectDeviceService | 서비스 목록(`device-rest`/`device-virtual`/`device-mqtt`)에서 **`device-mqtt`** 체크 → Next |
| ② SelectDeviceProfile | 방금 임포트한 **`Motor-MQTT-Sensor`** 선택 → Next |
| ③ DevicePrimary | Device Name에 **`MotorMQTT01`** 입력 (injector 스크립트의 이름과 반드시 일치해야 함) → Next |
| ④ CreateAutoEvent | 건너뛰어도 됩니다 — injector가 값을 직접 밀어 넣으므로 자동 주기 조회는 불필요 → Next |
| ⑤ CreateDeviceProtocol | 화면에 Schema/Host/Port/User/Password/ClientId/CommandTopic 입력란이 보이지만, **전부 빈칸으로 두고 Submit**해도 됩니다(아래 설명 참고) |

> 우측 상단에 뜨는 "refresh success!" 알림들은 화면이 최신 데이터를
> 자동으로 다시 불러올 때 뜨는 정상 알림입니다 — 무시해도 됩니다.

**⑤단계에서 필드를 비워도 되는 이유(실제 소스로 검증)**: `device-mqtt-go`
코드는 이 화면의 필드 중 `CommandTopic`만 읽는데, 그마저도 Core Command로
이 디바이스에 명령을 보낼 때만 쓰입니다. MotorMQTT01은 값을 발행만 하는
읽기 전용 센서라 명령을 받을 일이 없으므로 비워도 무방합니다. 나머지
필드(Schema/Host/Port/User/Password/ClientId)는 코드에서 아예 읽지
않습니다 — MQTT 브로커 접속 정보는 디바이스별이 아니라 `docker-compose.yml`의
`MQTTBROKERINFO_HOST`/`MQTTBROKERINFO_PORT`로 서비스 전체에 이미
설정되어 있기 때문입니다.

완료 후 **Devices** 목록에 `MotorMQTT01`이 보이고, Device Profile
화면에서 `Motor-MQTT-Sensor`의 `linkedDeviceCount`가 0 → 1로 바뀌어
있으면 등록이 정상적으로 끝난 것입니다.

## 3단계 — 센서 시뮬레이터 실행

```bash
cd injector
pip install paho-mqtt --break-system-packages   # 최초 1회
python3 mqtt_injector.py
```

터미널에 `rc=0`으로 출력되면 **브로커까지는 발행이 성공**한 겁니다.
다만 이건 "보냈다"는 증거일 뿐, "EdgeX가 실제로 받았다"는 증거는
아직 아닙니다 — 4단계에서 별도로 확인합니다.

## 4단계 — 실제로 수신됐는지 확인

**방법 A — curl로 직접 확인 (가장 빠름)**

```bash
curl "http://localhost:59880/api/v3/event/device/name/MotorMQTT01?limit=5"
```

Core Data(포트 59880)의 공식 API입니다. MotorMQTT01의 최근 이벤트
5건이 JSON으로 출력되면 수신이 확인된 것입니다. 빈 배열(`[]`)이거나
404가 나오면 아직 이벤트가 안 쌓인 것이니, 2단계(디바이스 등록)를
다시 확인하세요.

**방법 B — EdgeX UI로 확인**

`http://localhost:4000` → 왼쪽 메뉴 **DataCenter** → MotorMQTT01의
Temperature/Vibration 값이 계속 새로 쌓이는지 확인합니다. 6강에서
봤던 Motor01(REST) 이벤트와 나란히 보일 겁니다 —
**프로토콜은 다르지만 Core Data 안에서는 똑같은 형태로 저장됩니다.**

## REST와 무엇이 같고 다른가

| | device-rest (6강) | device-mqtt (4강) |
|---|---|---|
| 디바이스 → 플랫폼 | HTTP POST | MQTT publish |
| 누가 연결을 먼저 여는가 | 디바이스가 그때그때 요청 | 디바이스는 브로커에 미리 연결해두고 발행만 |
| 적합한 상황 | 가끔 보내는 데이터, 방화벽 뒤 디바이스 | 배터리 제약 있는 디바이스, 다대다 발행/구독 |
| Core Data 도착 후 | **동일** (Event/Reading) | **동일** (Event/Reading) |

즉 "플랫폼 앞단에서 어떤 프로토콜을 쓰느냐"는 다르지만, **일단 EdgeX
안으로 들어오면 6·7·10강에서 만든 App Service가 프로토콜과 무관하게
똑같이 처리할 수 있습니다.** 원한다면 6강의 `edge-stream-pipeline`
App Service의 `FilterFor` 목록에 `MotorMQTT01`을 추가해 두 프로토콜의
데이터를 하나의 파이프라인에서 함께 처리해볼 수도 있습니다.

## 자주 겪는 문제

- **device-mqtt가 계속 재시작됨** → 로그에서 `edgex-core-keeper` 연결
  오류 확인. 03 스택이 완전히 뜬 뒤 재시도
- **이벤트가 안 들어옴** → 2단계 디바이스 등록을 건너뛰었을 가능성.
  `curl "http://localhost:59880/api/v3/event/device/name/MotorMQTT01?limit=5"`로
  빈 배열이 나오는지 먼저 확인하고, EdgeX UI Metadata → Device에서
  MotorMQTT01이 등록돼 있는지 확인
- **전체 초기화** → `docker compose down`

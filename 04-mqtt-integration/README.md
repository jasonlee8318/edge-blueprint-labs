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
3. **Devices → Add Device**
   - Name: `MotorMQTT01`
   - Profile: `Motor-MQTT-Sensor`
   - Service: `device-mqtt`
   - Protocol: `mqtt` (CommandTopic은 비워둬도 됩니다 — 이 실습은 수신만 다룹니다)

## 3단계 — 센서 시뮬레이터 실행

```bash
cd injector
pip install paho-mqtt --break-system-packages   # 최초 1회
python3 mqtt_injector.py
```

EdgeX UI의 **Events** 메뉴에서 MotorMQTT01 이벤트가 실제로 쌓이는지
확인하세요. 6강에서 봤던 Motor01(REST) 이벤트와 나란히 보일 겁니다 —
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
  EdgeX UI의 Devices 메뉴에서 MotorMQTT01 확인
- **전체 초기화** → `docker compose down`

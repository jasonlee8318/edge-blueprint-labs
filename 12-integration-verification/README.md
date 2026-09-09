# 12강 — 플랫폼–데이터–AI 통합 설계 검증

새로 배우는 개념은 없습니다. **3~11강에서 만든 조각들이 하나의
파이프라인으로 실제로 맞물려 도는지** 검증하는 캡스톤 회차입니다.

```
Motor01(REST)         MotorMQTT01(MQTT)
     │                       │
     └──────────┬────────────┘  (4강: 두 프로토콜 모두 Core Data로 수렴)
                │
     ┌──────────┼──────────┬──────────────┐
     ▼          ▼          ▼              ▼
  5강 분류   6강 규칙판정  10강 AI판정   7강 압축+Store&Forward
  /status    /dashboard   /dashboard       │
                │            │             ▼
                └─────┬──────┘         mock-cloud
                      ▼                (장애 시 자동 저장→재전송)
              8강 Grafana 시각화
                      │
                      ▼
              11강 AI 판정 → Core Command → MotorAlarm01 자동 제어
```

같은 센서 데이터가 **여러 App Service에 동시에 소비되고, 그 결과가
다시 시각화·제어로 이어지는** 전체 흐름이 이번 회차의 검증 대상입니다.

## 사전 준비 — 지금까지 만든 걸 전부 켭니다

```bash
cd ../03-edgex-setup          && docker compose up -d
cd ../06-stream-pipeline      && docker compose up -d --build   # Motor01 등록 확인
cd ../04-mqtt-integration     && docker compose up -d            # MotorMQTT01 등록 확인
cd ../05-edge-pipeline        && docker compose up -d --build
cd ../07-store-forward        && docker compose up -d --build
cd ../10-tflite-deploy        && docker compose up -d --build
cd ../08-visualization        && docker compose up -d
cd ../11-automated-control    # MotorAlarm01 등록 후 python3 auto_control.py 는 아래에서
```

```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

아래 컨테이너가 전부 Up 상태여야 합니다:
`edgex-core-data`, `edgex-core-metadata`, `edgex-core-keeper`,
`edgex-core-command`, `edgex-device-rest`, `edgex-device-mqtt`, `ui`,
`edgex-sensor-classifier`, `edgex-stream-pipeline`, `edgex-store-forward`,
`mock-cloud`, `edgex-tflite-infer`, `grafana`

## 검증 체크리스트

이 실습 자체는 **체크리스트를 하나씩 확인하는 것**입니다. 9강과 같은
방식으로, 통과 여부를 스스로 판정합니다.

### 1. 데이터 소스 검증 (3·4강)

```bash
cd ../06-stream-pipeline/injector && python3 sensor_injector.py &
cd ../04-mqtt-integration/injector && python3 mqtt_injector.py &
```

- [ ] EdgeX UI(http://localhost:4000) → Events 메뉴에 **Motor01(REST)과
      MotorMQTT01(MQTT) 이벤트가 둘 다** 쌓이는가?
- [ ] 두 이벤트가 프로토콜은 다른데 Core Data 안에서는 같은 형태
      (Event/Reading)로 보이는가?

### 2. 분류 파이프라인(5강) 검증

```bash
curl http://localhost:59749/status
```

- [ ] `counts.total`이 계속 늘어나는가?
- [ ] `last_status.stage`가 상황에 따라 `LOCAL_STORE`/`PERIODIC_SEND`/`IMMEDIATE_SEND`로 바뀌는가?

### 3. 규칙 기반 처리(6강) 검증

```bash
curl http://localhost:59750/dashboard
```

- [ ] `status`가 WAITING → NORMAL/ANOMALY로 바뀌는가?
- [ ] `stale`이 false인가? (센서가 계속 도는 동안)

### 4. AI 기반 처리(10강) 검증

```bash
curl http://localhost:59752/dashboard
```

- [ ] `anomaly_score`가 0~1 사이 값으로 나오는가?
- [ ] 온도/진동이 높은 구간에서 `status: ANOMALY`로 바뀌는가?
- [ ] **6강과 10강의 판정이 항상 같지는 않다는 것을 확인** — 규칙은
      "온도만 90 넘으면 이상", AI는 "온도+진동의 조합 패턴"을 보므로
      경계값 근처에서 둘의 판단이 갈릴 수 있습니다.

### 5. 시각화(8강) 검증

- [ ] Grafana(http://localhost:3000) 대시보드에서 6강·10강 패널의 값이
      5초 새로고침마다 바뀌는가?
- [ ] 10강 Gauge 패널이 `anomaly_score` 0.5 기준으로 색이 바뀌는가?

### 6. 자동 제어(11강) 검증

```bash
cd ../11-automated-control
python3 auto_control.py
```

- [ ] AI 판정이 ANOMALY로 바뀔 때 `AlarmSiren`이 실제로 `true`로
      바뀌는가? (EdgeX UI의 MotorAlarm01 디바이스에서도 확인)
- [ ] 판정이 NORMAL로 돌아오면 `AlarmSiren`도 `false`로 돌아오는가?

### 7. 장애 대응(7강) 검증

```bash
cd ../07-store-forward
docker compose stop mock-cloud
# 10초 이상 대기
docker compose logs edge-store-forward | tail -20
docker compose start mock-cloud
docker compose logs -f mock-cloud
```

- [ ] mock-cloud 정지 중에도 `edge-store-forward`가 죽지 않고 계속 도는가?
- [ ] mock-cloud 재기동 후 밀린 데이터가 자동으로 도착하는가?

### 8. 플랫폼 안정성 검증

```bash
docker compose restart edge-stream-pipeline   # 06 폴더에서
```

- [ ] 재시작 후 몇 초 안에 다시 정상 판정이 나오는가? (Motor01 등록 정보가
      core-metadata에 남아있어 재등록 없이 복구되는지 확인)

## 최종 산출물

체크리스트 8개 섹션(총 18개 항목) 중 통과한 개수를 세어 제출하거나,
실패한 항목이 있다면 어느 조각(3/4/5/6/7/8/10/11강 중)의 문제인지
로그로 추적해보는 것이 이 회차의 과제입니다.

## 자주 겪는 문제

- **포트가 이미 사용 중** → 다른 회차를 이미 개별적으로 실행해봤다면
  컨테이너가 이미 떠 있는 것입니다. `docker ps`로 확인 후 새로 띄울
  필요 없이 그대로 사용하면 됩니다.
- **일부 서비스에만 이벤트가 도착** → 각 서비스가 올바른 디바이스를
  구독하고 있는지 `docker compose logs`로 개별 확인

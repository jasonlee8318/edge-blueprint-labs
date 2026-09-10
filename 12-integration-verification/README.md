# 12강 — 플랫폼–데이터–AI 통합 설계 검증

새로 배우는 개념은 없습니다. **3~11강에서 만든 조각들이 하나의
플랫폼 위에서 실제로 맞물려 도는지** 검증하는 캡스톤 회차입니다.

> 각 회차가 서로 종속되지 않도록, 회차마다 **독립된 디바이스 이름**을
> 씁니다. 같은 Motor01 이벤트를 여러 서비스가 나눠 보는 구조가 아니라,
> 각 App Service가 **자기 몫의 디바이스를 각자 구독**하는 구조입니다.

```
Motor01(REST, 5·7강 공용)   Motor02(REST, 6강)   Motor03(REST, 10강)   MotorMQTT01(MQTT, 4강)
     │                            │                     │                     │
     ▼                            ▼                     ▼                     ▼
5강 분류 + 7강 압축/전송      6강 규칙판정          10강 AI판정          (4강 자체 검증용)
/status, mock-cloud           /dashboard            /dashboard
     │                            │                     │
     └────────────────────────────┴─────────────────────┘
                                   ▼
                          8강 Grafana 시각화
                          (6강·10강 엔드포인트 연결)
                                   │
                                   ▼
                   11강: 10강 AI 판정을 읽어 Core Command로
                         MotorAlarm01(액추에이터) 자동 제어
```

6강과 10강은 서로 다른 디바이스(Motor02/Motor03)를 보므로, "같은 이벤트를
동시에 두 가지 방식으로 판정"하는 건 아닙니다. 대신 **같은 분포의 데이터를
각자 독립적으로 판정**하며, 두 injector를 함께 돌려 규칙 기반과 AI 기반의
판정 경향을 나란히 비교해볼 수 있습니다.

같은 성격의 센서 데이터가 **여러 App Service에 독립적으로 소비되고, 그
결과가 다시 시각화·제어로 이어지는** 전체 흐름이 이번 회차의 검증 대상입니다.

## 사전 준비 — 지금까지 만든 걸 전부 켭니다

```bash
cd ../03-edgex-setup          && docker compose up -d
cd ../05-edge-pipeline        && docker compose up -d --build   # Motor01 등록
cd ../06-stream-pipeline      && docker compose up -d --build   # Motor02 등록
cd ../07-store-forward        && docker compose up -d --build   # Motor01 재사용
cd ../10-tflite-deploy        && docker compose up -d --build   # Motor03 등록
cd ../04-mqtt-integration     && docker compose up -d            # MotorMQTT01 등록
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
cd ../05-edge-pipeline/injector && python3 sensor_injector.py &   # Motor01(REST)
cd ../04-mqtt-integration/injector && python3 mqtt_injector.py &  # MotorMQTT01(MQTT)
```

- [ ] EdgeX UI(http://localhost:4000) → DataCenter → Event 메뉴에 **Motor01(REST)과
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
- [ ] **6강(Motor02, 규칙 기반)과 10강(Motor03, AI 기반)의 판정 경향을
      비교** — 두 injector를 함께 돌려보면, 규칙은 "온도만 90 넘으면
      이상"으로 단순 판정하고 AI는 "온도+진동의 조합 패턴"을 보고
      판정하는 차이가 있어, 비슷한 값 구간에서도 두 방식의 판단이
      갈릴 수 있습니다. (서로 다른 디바이스를 구독하므로 완전히
      동일한 이벤트를 비교하는 것은 아닙니다.)

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

- [ ] 재시작 후 몇 초 안에 다시 정상 판정이 나오는가? (Motor02 등록 정보가
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

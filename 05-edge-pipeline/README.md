# 5강 — 엣지 센서 데이터 처리 파이프라인

Raw Data → Validation → Filtering → Aggregation → Anomaly Detection →
Transmission Classification(즉시전송/주기전송/Local저장/별도처리)을
Motor01의 실시간 이벤트에 대해 계속 수행하는 EdgeX Go App Service입니다.

## 시작 전 필수 조건

1. **3회차(`03-edgex-setup`)가 실행 중이어야 합니다.**
2. **Motor01 디바이스가 등록돼 있어야 합니다** (6강에서 이미 했다면 재사용).

## 실행

```bash
docker compose up -d --build
```

```bash
cd ../06-stream-pipeline/injector
python3 sensor_injector.py
```

## 결과 확인

```bash
curl http://localhost:59749/status
```

```json
{
  "counts": {"total": 12, "invalid": 1, "normal": 9, "anomaly": 2, "summary_sent": 1},
  "last_status": {"stage": "PERIODIC_SEND", "count": 5, "avg_temperature": 74.2, ...}
}
```

- `counts`: 원고 8번(최종 처리 흐름 요약)의 실시간 버전
- `last_status.stage`: 가장 최근 이벤트가 4가지 중 어디로 분류됐는지
  (`INVALID_DATA` / `IMMEDIATE_SEND` / `LOCAL_STORE` / `PERIODIC_SEND`)

## 6강·7강과 무엇이 다른가

- **6강**: 5건 이동평균으로 상태(NORMAL/ANOMALY)를 "판단"하는 데 집중
- **7강**: 판단된 결과를 "전송·저장"하는 방법(Store&Forward)에 집중
- **5강**: 그 둘 사이 — 원시 데이터 각각을 **어느 목적지로 보낼지 분류**하는 로직 자체에 집중

세 회차가 내용상 겹치는 부분이 있는 건 자연스럽습니다. 실제로 EdgeX
App Service를 설계할 때도 "분류(5강) → 판단(6강) → 전송(7강)"을
하나의 파이프라인으로 합치는 것이 정상적인 발전 방향입니다.

## ⚠️ 수정 이력

최초 설계 단계에서는 Temperature/Vibration이 한 이벤트 안에 같이
온다고 가정했지만, device-rest는 POST 한 번당 Event 한 개(리딩
1개)만 만듭니다. 이 버전은 처음부터 최근값 캐시 패턴으로 만들었습니다
(6·7·10강도 같은 이유로 사후 수정했습니다).

## 자주 겪는 문제

- **counts.invalid만 계속 올라감** → Motor01 등록이 안 됐거나
  injector가 다른 디바이스로 전송 중인지 확인
- **전체 초기화** → `docker compose down`

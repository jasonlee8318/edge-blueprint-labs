# 5강 — 엣지 센서 데이터 처리 파이프라인

Raw Data → Validation → Filtering → Aggregation → Anomaly Detection →
Transmission Classification(즉시전송/주기전송/Local저장/별도처리)을
Motor01의 실시간 이벤트에 대해 계속 수행하는 EdgeX Go App Service입니다.


## ⚠️ 긴급 수정 — v3 SDK로는 이 스택에 연결이 안 됩니다

App Functions SDK **v3.1.1**을 쓰고 있었는데, 이 버전이 참조하는
`go-mod-configuration`은 레지스트리 타입으로 `consul`만 지원하고
`keeper`는 지원하지 않습니다(실제 소스 `factory.go`에서 확인 — `keeper`
케이스가 아예 없고 default에서 `unknown configuration client type`
에러를 던집니다). 그런데 3회차 스택은 Consul이 아니라 core-keeper를
씁니다. 즉 v3 SDK로 만든 이 App Service는 **애초에 이 스택에 연결될
수 없는 구조**였습니다.

**App Functions SDK v4.0.2로 업그레이드**해서 해결했습니다(go.mod,
main.go의 import 경로, Dockerfile의 Go 버전(1.25) 전부 반영됨).
7강이 처음부터 v4를 썼던 게 우연히 맞는 선택이었습니다.
v4로 올리면서 **API도 같이 바뀌었습니다** — 라우트 등록이 `AddRoute`
(표준 net/http)에서 `AddCustomRoute`(Echo 프레임워크)로 변경됐습니다.
핸들러 시그니처가 `func(w http.ResponseWriter, r *http.Request)`에서
`func(c echo.Context) error`로, 인증 여부를 나타내는 두 번째 인자
(`interfaces.Unauthenticated`)가 새로 추가됐습니다. 이 코드에는 이미
반영되어 있습니다.


## 시작 전 필수 조건

**3회차(`03-edgex-setup`)가 먼저 실행 중이어야 합니다.**

```bash
cd ../03-edgex-setup
docker compose up -d
```

## 1단계 — Motor01 디바이스 등록 (EdgeX UI)

> 7강을 이미 진행하셨다면 Motor01이 그대로 남아있으니 이 단계는
> 건너뛰어도 됩니다. (6강은 별도의 Motor02를 쓰므로 여기 해당 안 됩니다.)
> 저장소를 새로 받았거나 3회차를 새로 띄우셨다면 다시 등록해야 합니다.

1. `http://localhost:4000` 접속
2. **Metadata → Device Profile → Add Profile** → 이 폴더의
   `device-profile/motor01-vibration-sensor.yaml`을 화면에 드래그 → Submit
3. **Metadata → Device → Add Device Wizard**

| 단계 | 입력값 |
|---|---|
| ① SelectDeviceService | `device-rest` 체크 → Next |
| ② SelectDeviceProfile | `Motor01-Vibration-Sensor` 선택 → Next |
| ③ DevicePrimary | Device Name: **`Motor01`** (반드시 이 이름) → Next |
| ④ CreateAutoEvent | 건너뛰고 Next |
| ⑤ CreateDeviceProtocol | 필드 전부 빈칸으로 두고 Submit |

완료 후 **Metadata → Device** 목록에 `Motor01`이 보이면 성공입니다.

## 2단계 — App Service 빌드·기동

```bash
docker compose up -d --build
```

## 3단계 — 센서 데이터 주입

```bash
cd injector
pip install requests --break-system-packages   # 최초 1회
python3 sensor_injector.py
```

## 4단계 — 결과 확인

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

- **counts.total이 계속 0** → Motor01 미등록이 가장 흔한 원인입니다.
  Metadata → Device에서 `Motor01`이 있는지, 이름이 정확히 일치하는지
  확인하세요 (`MotorMQTT01`은 4강용 디바이스라 이름이 다릅니다 — 헷갈리지
  않도록 주의)
- **counts.invalid만 계속 올라감** → Motor01 등록이 안 됐거나
  injector가 다른 디바이스로 전송 중인지 확인
- **전체 초기화** → `docker compose down`

# 8강 — 데이터 활용 및 시각화 (Grafana)

6강·10강 App Service의 `/dashboard` 엔드포인트를 Grafana로 실시간
시각화합니다. 별도 DB나 익스포터 없이, **Grafana가 직접 두 서비스의
JSON 응답을 주기적으로 읽어와** 화면에 뿌리는 구조입니다.

## 이 회차에서 실제로 확인한 것

Grafana의 예전 "JSON API" 플러그인(marcusolsson-json-datasource)은
2027년 지원 종료가 예고되어 있어서, Grafana 공식 문서가 권장하는
후속 플러그인인 **Infinity**(`yesoreyeram-infinity-datasource`)를
대신 사용합니다. provisioning 설정 형식은 공식 문서 기준으로 맞췄습니다.

## 시작 전 필수 조건

1. **3회차(`03-edgex-setup`)**가 실행 중이어야 합니다.
2. **6강(`06-stream-pipeline`)**과 **10강(`10-tflite-deploy`)** App
   Service가 떠 있어야 볼 데이터가 있습니다 (둘 다 없어도 Grafana 자체는
   뜨지만 대시보드가 비어 보입니다).
3. 두 App Service의 센서 시뮬레이터(`sensor_injector.py`)가 돌고 있으면
   더 실감납니다.

## 실행

```bash
docker compose up -d
```

최초 기동 시 Infinity 플러그인을 설치하느라 조금 걸릴 수 있습니다.

## 1단계 — Grafana 접속

http://localhost:3000 → 기본 계정 `admin` / `admin` (최초 로그인 시
비밀번호 변경 요청이 뜨면 그대로 진행하거나 건너뛰어도 됩니다).

**Connections → Data sources**에서 `Infinity`가 이미 등록되어 있는지
확인하세요 (provisioning으로 자동 추가됨).

## 2단계 — 규칙 기반 판정(6강) 패널 만들기

1. 왼쪽 메뉴 **Dashboards → New → New Dashboard → Add visualization**
2. Data source: **Infinity** 선택
3. 쿼리 설정:
   - Type: `JSON`
   - Parser: `Backend`
   - Source: `URL`
   - URL: `http://edgex-stream-pipeline:59750/dashboard`
   - Format: `Table`
4. 오른쪽 패널 타입을 **Stat**으로 변경 → `status`, `avg_temperature`,
   `avg_vibration` 필드가 카드로 보이는지 확인
5. 우측 상단 새로고침 주기를 **5s**로 설정 → 값이 실시간으로 바뀌는지 확인

## 3단계 — AI 판정(10강) 패널 추가

같은 방식으로 패널을 하나 더 추가합니다.

- URL: `http://edgex-tflite-infer:59752/dashboard`
- 이번엔 패널 타입을 **Gauge**로 설정하고 `anomaly_score` 필드를 선택
- Gauge의 Threshold를 0.5로 설정하면, 값이 0.5를 넘을 때 색이 바뀌는
  것까지 확인할 수 있습니다 (10강의 `scoreThresh`와 동일한 기준)

## 4단계 — 저장

우측 상단 **Save dashboard**로 저장하면, 이후에는 Dashboards 목록에서
바로 다시 열 수 있습니다.

## 왜 시계열 그래프가 아니라 Stat/Gauge인가

`/dashboard` 엔드포인트는 "가장 최근 결과 하나"만 돌려줍니다(6강의
Cache 개념과 동일 — 매번 새로 계산하지 않고 최신 값만 들고 있음).
그래서 과거 이력을 선으로 잇는 시계열 그래프보다는, **지금 이 순간의
상태를 보여주는 Stat/Gauge**가 이 데이터 구조에 더 맞습니다. 만약
시계열 그래프까지 만들고 싶다면, 값을 시간별로 쌓아두는 저장소(DB)가
추가로 필요합니다 — 이건 7강의 Postgres를 재활용해 확장해볼 수 있는
심화 주제입니다.

## 자주 겪는 문제

- **Infinity 데이터소스가 안 보임** → `docker compose logs grafana`에서
  플러그인 설치가 끝났는지 확인 (컨테이너 재시작 필요할 수 있음)
- **쿼리에서 404/connection refused** → 6강/10강 App Service가 떠 있는지,
  URL의 컨테이너 이름·포트가 맞는지 확인
- **패널에 값이 안 보임(빈 테이블)** → Format을 `Table`로 바꿨는지,
  Parser가 `Backend`인지 확인. `/dashboard`가 평평한 JSON 객체 하나를
  반환하므로 Backend 파서가 이를 1행짜리 표로 변환합니다
- **전체 초기화** → `docker compose down`

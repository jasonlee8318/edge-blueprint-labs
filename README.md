# 엣지에서 시작되는 스마트 서비스 청사진 — 실습 저장소

모든 실습은 **EdgeX Foundry 위에서** 진행합니다 (예외적으로 로직만 확인하는
경우가 아니라면). 3회차에서 EdgeX 스택을 한 번 띄우고, 이후 회차들은 그 위에
필요한 App Service/Device를 얹는 방식으로 이어집니다.

## 실습 파일 내려받기

1. 이 저장소를 ZIP 다운로드하거나 `git clone`
2. Docker Desktop 설치·실행 (메모리 6GB 이상 권장)

## 회차별 실습

| 폴더 | 회차 | 내용 |
|---|---|---|
| `03-edgex-setup/` | EdgeX Foundry 환경 구성 | 공식 compose로 EdgeX 스택 기동, UI로 확인 |
| `04-mqtt-integration/` | 엣지 플랫폼 연동 설계 (MQTT/REST) | device-mqtt 추가, REST와 MQTT 두 프로토콜 연동 비교 |
| `05-edge-pipeline/` | 엣지 센서 데이터 처리 파이프라인 | 검증→필터링→집계→이상탐지→전송정책 분류 |
| `06-stream-pipeline/` | 실시간 스트림 처리 / 전처리·캐싱 구조 설계 | Motor01 가상 센서 + Go App Service로 Buffer/Window/Event/Cache 구현 |
| `07-store-forward/` | Cloud 장애 대응 / Store & Forward | EdgeX 내장 Store&Forward로 네트워크 장애 시 자동 저장·재전송 구현 |
| `08-visualization/` | 데이터 활용 및 시각화 | Grafana + Infinity 데이터소스로 App Service 결과 실시간 시각화 |
| `09-model-quantization/` | Edge AI 모델 경량화 | scikit-learn 모델 학습 + FP16 양자화 (EdgeX와 무관한 별도 환경) |
| `10-tflite-training/` | TFLite 변환 (1/2) | TensorFlow 학습 + TFLite 변환 (EdgeX와 무관한 별도 환경) |
| `10-tflite-deploy/` | TFLite 변환 (2/2) | 진짜 TFLite 인터프리터로 Motor01을 실시간 추론하는 App Service |
| `11-automated-control/` | 실시간 분석 및 의사결정 | AI 판정 결과를 Core Command로 실제 액추에이터에 반영하는 자동 제어 |
| `12-integration-verification/` | 플랫폼–데이터–AI 통합 설계 검증 | 3~11강 조각을 전부 켜고 하나의 파이프라인으로 검증하는 캡스톤 |

1, 2회차는 실습이 없는 회차입니다 (원본 강의계획서 기준).

## ⚠️ 알려진 수정 이력

5·6·7·10강은 모두 device-rest를 통해 Temperature/Vibration을 받는데,
device-rest는 **POST 한 번당 Event 한 개(리딩 1개)** 만 만듭니다. 두 값을
따로 POST하면 한 이벤트에 절대 같이 오지 않는다는 걸 뒤늦게 확인해서,
모든 App Service가 "최근값을 각각 저장했다가 매 이벤트마다 최신 쌍으로
판단"하는 패턴으로 되어 있습니다. 각 회차 README의 "수정 이력" 항목에
자세한 설명이 있습니다.

## 참고 — 모든 회차가 EdgeX 위에서 도는 건 아닙니다

이 과정의 기본 원칙은 "실습은 EdgeX Foundry 위에서"이지만, **EdgeX가 관여할
지점이 없는 회차**(예: 9강의 모델 학습·양자화)는 예외적으로 별도의 가벼운
환경을 씁니다. EdgeX는 데이터 수집·라우팅 플랫폼이지 ML 학습 도구가 아니기
때문입니다. 새 회차를 추가할 때마다 "이게 EdgeX의 역할과 맞닿아 있는가"를
먼저 판단합니다.

## 회차 진행 순서

```bash
cd 03-edgex-setup
docker compose up -d        # EdgeX 스택 기동 (이후 회차 내내 켜둔 채로 진행)

cd ../06-stream-pipeline
docker compose up -d --build   # Motor01 디바이스 최초 등록 (다른 회차가 재사용)

cd ../04-mqtt-integration && docker compose up -d
cd ../05-edge-pipeline    && docker compose up -d --build
cd ../07-store-forward    && docker compose up -d --build
cd ../10-tflite-training  # (별도 환경) 모델 학습·변환
cd ../10-tflite-deploy    && docker compose up -d --build
cd ../08-visualization    && docker compose up -d
cd ../11-automated-control  # MotorAlarm01 등록 후 python3 auto_control.py
cd ../12-integration-verification  # 전체 통합 검증
```

3회차 스택은 특별한 이유가 없다면 **과정 내내 계속 켜둔 상태로 진행**합니다.
매 회차 새로 올리지 않습니다 — 그 위에 이번 주 실습만 얹는 구조입니다.

## 전체 초기화가 필요할 때

```bash
cd 06-stream-pipeline && docker compose down
cd ../03-edgex-setup && docker compose down -v
```

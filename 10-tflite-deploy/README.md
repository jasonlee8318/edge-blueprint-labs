# 10강 (2/2) — 진짜 TFLite 모델을 EdgeX 위에서 추론

Motor01의 실시간 온도/진동 값을 **실제 TensorFlow Lite 인터프리터**로
추론합니다. 지금까지(6강)의 `temp >= 90` 같은 단순 임계치 규칙을
**학습된 신경망**으로 교체하는 회차입니다.


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

## 이 방식이 실제로 되는지 미리 검증했습니다

`go-tflite`(mattn/go-tflite)는 TensorFlow Lite C API 공유 라이브러리가
필요한데, 이 저장소를 만드는 과정에서 실제로:

1. `motor_anomaly_model.tflite`를 진짜로 학습·변환하고
2. Python `tf.lite.Interpreter`로 추론 → `0.6263745427...`
3. 같은 모델을 Go `go-tflite`로 추론 → `0.62637454`

두 값이 소수점까지 일치하는 것을 확인했습니다. `app/model/`에 들어있는
`.tflite` 파일이 바로 이때 검증에 사용한 실물입니다.

## 시작 전 필수 조건

1. **3회차(`03-edgex-setup`)가 실행 중이어야 합니다.**
2. **Motor01 디바이스가 등록돼 있어야 합니다** (6강에서 이미 했다면 재사용).

## 실행

```bash
docker compose up -d --build
```

빌드 단계에서 go-tflite의 "buildkit"(사전 컴파일된 TensorFlow Lite C
라이브러리)을 GitHub Release에서 내려받습니다 — Bazel로 TensorFlow
전체를 빌드하지 않아서 빌드 시간이 오래 걸리지 않습니다.

## 결과 확인

```bash
cd injector
pip install requests --break-system-packages   # 최초 1회
python3 sensor_injector.py   # Motor01 값 주입

# 다른 터미널에서
curl http://localhost:59752/dashboard
```

```json
{"temperature":91.3,"vibration":7.2,"anomaly_score":0.6194,"status":"ANOMALY"}
```

`status`가 이제 규칙이 아니라 **신경망 추론 결과**로 결정됩니다.

## 알아두실 점

- App Functions SDK는 6강과 동일하게 **v3.1.1**을 사용합니다(Go 1.22 호환).
- TFLite 인터프리터의 `Invoke()`는 스레드 세이프하지 않아 mutex로 감쌌습니다.
- 이 App Service는 6강 App Service와 **별개의 컨테이너**로 같은 Motor01
  이벤트를 각자 구독합니다. 두 방식(규칙 기반 vs AI 기반)의 판정을
  나란히 비교해볼 수 있습니다.

## 자주 겪는 문제

- **빌드 중 buildkit 다운로드 실패** → 교육장 방화벽이 GitHub Release
  자산(`release-assets.githubusercontent.com`)을 막고 있을 수 있습니다.
- **"cannot load model"** → `app/model/motor_anomaly_model.tflite`가
  이미지에 제대로 복사됐는지 (`docker compose build --no-cache`로 재시도)
- **전체 초기화** → `docker compose down`

## ⚠️ 수정 이력

최초 버전은 Temperature/Vibration이 **한 이벤트 안에 같이 온다고 가정**했지만,
device-rest는 POST 한 번당 Event 한 개(리딩 1개)만 만듭니다. 두 값을 따로
POST하면 절대 한 이벤트에 같이 오지 않아 파이프라인이 항상 멈춰 있었습니다.
지금 버전은 각 리딩의 **최근값을 저장**해뒀다가 매 이벤트마다 최신 쌍으로
판단하도록 고쳤습니다.

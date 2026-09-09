# 11강 — 실시간 분석 및 의사결정 (AI 결과 기반 자동 제어)

10강에서 만든 AI 판정을 "보여주기"만 했다면, 이번엔 그 판정으로
**실제 디바이스를 제어**합니다. AI가 ANOMALY로 판단하면 알람 사이렌을
켜고, NORMAL로 돌아오면 끕니다.

## 이 회차의 핵심 — Core Command

지금까지는 EdgeX 안으로 데이터가 **들어오는** 방향만 다뤘습니다
(Core Data로 센서값 수신). 이번엔 반대 방향, **밖에서 안으로 명령을
내리는** Core Command를 처음 씁니다.

EdgeX 공식 문서는 Core Command를 이렇게 설명합니다: "다른 마이크로서비스뿐
아니라, 디바이스를 제어해야 하는 어떤 외부 시스템에서도" 호출할 수 있다고요.
그래서 이번 컨트롤러(`auto_control.py`)는 **EdgeX App Service가 아니라
평범한 외부 파이썬 스크립트**입니다 — Core Command의 실제 쓰임새를
그대로 보여주는 구성입니다.

## 검증한 내용

`device-sdk-go`의 실제 소스(`writeDeviceResource`)를 확인해 PUT
요청 형식을 정확히 맞췄고, Core Command의 실제 라우트
(`/api/v3/device/name/{name}/{command}`)도 소스에서 확인했습니다.

## 시작 전 필수 조건

1. **3회차(`03-edgex-setup`)**, **10강(`10-tflite-deploy`)**이 떠 있어야 합니다.
2. 10강의 센서 시뮬레이터(`06-stream-pipeline/injector/sensor_injector.py`)가
   돌고 있어야 AI 판정이 계속 갱신됩니다.

## 1단계 — 알람 액추에이터 등록 (EdgeX UI)

1. http://localhost:4000 접속
2. **Device Profiles → Import** → `device-profile/motor-alarm-actuator.yaml` 업로드
3. **Devices → Add Device**
   - Name: `MotorAlarm01`
   - Profile: `Motor-Alarm-Actuator`
   - Service: `device-virtual` (이미 3회차 스택에 떠 있는 걸 그대로 사용)
   - Protocol: `other` (빈 값으로 두면 됩니다)

## 2단계 — 컨트롤러 실행

```bash
pip install requests --break-system-packages   # 최초 1회
python3 auto_control.py
```

출력 예시:
```
AI 판정=NORMAL (변화 없음, 알람 상태 유지: OFF)
AI 판정=ANOMALY → 알람 ON (PUT 응답=200, 실제 반영값=true)
AI 판정=ANOMALY (변화 없음, 알람 상태 유지: ON)
AI 판정=NORMAL → 알람 OFF (PUT 응답=200, 실제 반영값=false)
```

## 3단계 — EdgeX UI에서 직접 확인

**Devices → MotorAlarm01**에서 `AlarmSiren` 값이 AI 판정에 따라
실시간으로 true/false 바뀌는 걸 GUI로도 확인할 수 있습니다.

## 왜 "PUT 후 다시 GET"까지 하는가

`set_alarm()`으로 값을 쓴 뒤 바로 `get_alarm_state()`로 다시 읽어서
확인하는 이유는, **명령이 실제로 반영됐는지 검증하는 습관**을 보여주기
위해서입니다. 실무에서는 네트워크 문제나 디바이스 응답 지연으로 PUT이
성공(200)해도 실제 상태 변경이 지연될 수 있어, 이렇게 확인하는 패턴이
일반적입니다.

## 자주 겪는 문제

- **`AlarmSiren` PUT이 실패함** → 1단계에서 디바이스 등록을 건너뛰었을
  가능성. EdgeX UI의 Devices 메뉴에서 MotorAlarm01 확인
- **AI 판정이 계속 WAITING** → 10강 App Service와 센서 시뮬레이터가
  둘 다 떠 있는지 확인
- **연결 오류** → 03/10강 스택이 켜져 있는지, 포트(59882, 59752)가
  맞는지 확인

# ============================================================
# edge-auto-control
# 11강: 실시간 분석 및 의사결정 — AI 판정 결과로 자동 제어
#
# 10강 App Service(edge-tflite-infer)의 /dashboard에서 AI 판정을
# 계속 읽어와, ANOMALY면 EdgeX Core Command로 실제 알람 사이렌
# (MotorAlarm01 액추에이터)을 켜고, NORMAL이면 끕니다.
#
# 이 스크립트는 EdgeX App Service가 아니라 평범한 외부 프로그램입니다 —
# 그리고 그게 핵심입니다. EdgeX 공식 문서가 Core Command를 이렇게
# 설명합니다: "다른 애플리케이션이나, 디바이스를 제어해야 하는 외부
# 시스템 어디서든" 호출할 수 있다고요. 즉 EdgeX 내부 코드가 아니어도
# 누구나 REST 호출 몇 번으로 디바이스를 제어할 수 있다는 걸 보여줍니다.
#
# 검증된 실제 API (device-sdk-go의 writeDeviceResource 소스 확인):
#   PUT http://<core-command>/api/v3/device/name/<device>/<resource>
#   body: {"<resourceName>": "value(문자열)"}
# ============================================================
import time
import requests

CORE_COMMAND_URL = "http://localhost:59882/api/v3/device/name"
AI_DASHBOARD_URL = "http://localhost:59752/dashboard"  # 10강 App Service
ALARM_DEVICE = "MotorAlarm01"
ALARM_RESOURCE = "AlarmSiren"

POLL_INTERVAL = 2  # 초


def get_ai_status():
    """10강 App Service에서 가장 최근 AI 판정 결과를 읽어옵니다."""
    resp = requests.get(AI_DASHBOARD_URL, timeout=3)
    resp.raise_for_status()
    return resp.json()


def set_alarm(on: bool):
    """Core Command로 실제 알람 사이렌 디바이스에 값을 씁니다 (PUT)."""
    url = f"{CORE_COMMAND_URL}/{ALARM_DEVICE}/{ALARM_RESOURCE}"
    body = {ALARM_RESOURCE: "true" if on else "false"}
    resp = requests.put(url, json=body, timeout=3)
    return resp.status_code


def get_alarm_state():
    """방금 쓴 값이 실제로 반영됐는지 Core Command로 다시 읽어(GET) 확인합니다."""
    url = f"{CORE_COMMAND_URL}/{ALARM_DEVICE}/{ALARM_RESOURCE}"
    resp = requests.get(url, timeout=3)
    resp.raise_for_status()
    data = resp.json()
    reading = data["event"]["readings"][0]["value"]
    return reading


if __name__ == "__main__":
    print("edge-auto-control 시작 — 2초마다 AI 판정을 확인해 알람을 제어합니다.")
    print("(먼저 EdgeX UI에서 Motor-Alarm-Actuator 프로필을 임포트하고")
    print(" device-virtual 서비스에 'MotorAlarm01' 디바이스를 등록해야 합니다.)")

    last_state = None
    while True:
        try:
            ai = get_ai_status()
            status = ai.get("status", "WAITING")
            should_alarm = status == "ANOMALY"

            if should_alarm != last_state:
                code = set_alarm(should_alarm)
                confirmed = get_alarm_state()
                print(f"AI 판정={status} → 알람 {'ON' if should_alarm else 'OFF'} "
                      f"(PUT 응답={code}, 실제 반영값={confirmed})")
                last_state = should_alarm
            else:
                print(f"AI 판정={status} (변화 없음, 알람 상태 유지: "
                      f"{'ON' if last_state else 'OFF'})")

        except requests.RequestException as e:
            print("연결 오류 — 10강 App Service 또는 Core Command가 떠 있는지 확인:", e)

        time.sleep(POLL_INTERVAL)

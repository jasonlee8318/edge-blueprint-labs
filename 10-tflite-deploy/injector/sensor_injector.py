# ============================================================
# Motor01 센서 시뮬레이터
# (6강의 injector와 동일한 스크립트입니다 — 10강 실습을 다른 폴더로
#  안 넘어가고 진행할 수 있도록 이 폴더에도 그대로 복사해뒀습니다.)
# device-rest 서비스(POST /api/v3/resource/{device}/{resource})로
# 실제 EdgeX 이벤트를 1초마다 주입합니다. 진짜 센서가 있다면 이 스크립트
# 자리에 그 센서의 통신 코드가 들어간다고 생각하면 됩니다.
#
# 검증된 실제 API: device-rest-go의 RestHandler가
#   POST /api/v3/resource/<deviceName>/<resourceName>
#   body: 순수 텍스트 값 (예: "73.4")
# 을 받아 Core Data로 전달합니다.
#
# ⚠️ 온도와 진동을 "따로" POST합니다 — 이게 중요한 이유는, EdgeX가 이 둘을
# 하나로 묶어주지 않고 각각 별도의 이벤트로 만들기 때문입니다. Go App
# Service 쪽 코드가 "최근값 캐시" 패턴을 쓰는 이유가 바로 이 동작 때문입니다.
# ============================================================
import random
import time
import requests

DEVICE_REST_URL = "http://localhost:59986/api/v3/resource"
DEVICE_NAME = "Motor01"


def push(resource: str, value: float):
    """온도 또는 진동 값 하나를 device-rest로 보냅니다.
    반환하는 상태 코드가 200이면 EdgeX가 정상적으로 받아들인 것입니다."""
    url = f"{DEVICE_REST_URL}/{DEVICE_NAME}/{resource}"
    resp = requests.post(url, data=str(value), headers={"Content-Type": "text/plain"})
    return resp.status_code


def generate():
    """실제 센서 대신 임의의 값을 만듭니다.
    범위(60~95, 2.0~8.0)는 원고의 예시 데이터 범위와 맞췄습니다."""
    return {
        "temperature": round(random.uniform(60, 95), 1),
        "vibration": round(random.uniform(2.0, 8.0), 1),
    }


if __name__ == "__main__":
    print(f"Motor01 시뮬레이터 시작 — {DEVICE_REST_URL} 로 1초마다 전송합니다.")
    print("(먼저 EdgeX UI에서 Motor-Vibration-Sensor 프로필을 임포트하고")
    print(" device-rest 서비스에 'Motor01' 디바이스를 등록해야 합니다.)")

    for i in range(60):
        data = generate()
        # 두 번의 별도 POST — 각각 독립된 EdgeX 이벤트를 만듭니다.
        t_status = push("Temperature", data["temperature"])
        v_status = push("Vibration", data["vibration"])
        print(f"[{i+1:02d}] temp={data['temperature']}(→{t_status})  "
              f"vibration={data['vibration']}(→{v_status})")
        time.sleep(1)  # "1초마다 지속 발생"이라는 원고 시나리오를 그대로 재현

# ============================================================
# MotorMQTT01 센서 시뮬레이터
# device-mqtt 서비스가 구독하는 토픽으로 실제 EdgeX 이벤트를 발행합니다.
#
# 검증된 실제 규칙 (device-mqtt-go의 onIncomingDataReceived 소스 확인):
#   토픽: incoming/data/<디바이스명>/<리소스명>
#   페이로드: 리소스가 1개뿐이면 순수 텍스트 값으로 인식
#
# 6강의 sensor_injector.py(REST 방식)와 비교해보면 좋습니다 —
# 같은 데이터를 다른 프로토콜(MQTT vs REST)로 넣는 차이만 있고,
# EdgeX 안에서는 결국 똑같은 Event/Reading으로 변환됩니다.
# ============================================================
import random
import time

import paho.mqtt.client as mqtt

BROKER_HOST = "localhost"
BROKER_PORT = 1883  # 03-edgex-setup이 노출해둔 mqtt-broker 포트
DEVICE_NAME = "MotorMQTT01"


def publish(client, resource: str, value: float):
    topic = f"incoming/data/{DEVICE_NAME}/{resource}"
    result = client.publish(topic, str(value))
    return topic, result.rc  # rc == 0 이면 발행 성공


def generate():
    return {
        "temperature": round(random.uniform(60, 95), 1),
        "vibration": round(random.uniform(2.0, 8.0), 1),
    }


if __name__ == "__main__":
    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
    client.connect(BROKER_HOST, BROKER_PORT, 60)

    print(f"MotorMQTT01 시뮬레이터 시작 — {BROKER_HOST}:{BROKER_PORT} 로 1초마다 발행합니다.")
    print("(먼저 EdgeX UI에서 Motor-MQTT-Sensor 프로필을 임포트하고")
    print(" device-mqtt 서비스에 'MotorMQTT01' 디바이스를 등록해야 합니다.)")

    for i in range(60):
        data = generate()
        t_topic, t_rc = publish(client, "Temperature", data["temperature"])
        v_topic, v_rc = publish(client, "Vibration", data["vibration"])
        print(f"[{i+1:02d}] temp={data['temperature']}(rc={t_rc})  "
              f"vibration={data['vibration']}(rc={v_rc})")
        time.sleep(1)

    client.disconnect()

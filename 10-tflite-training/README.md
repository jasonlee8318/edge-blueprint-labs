# 10강 (1/2) — TensorFlow/Keras 학습 및 TFLite 변환

EdgeX와 무관한 별도 환경입니다. TensorFlow로 모델을 학습하고
TFLite로 변환하는 것 자체는 9강과 마찬가지로 순수 ML 작업입니다.

## 실행

```bash
docker compose pull   # tensorflow-notebook은 용량이 큽니다 — 미리 받아두세요
docker compose up -d
```

http://localhost:8888/?token=tflite 접속 → `work/tflite_training.ipynb` 실행.

## 다음 단계

이 노트북이 만드는 `motor_anomaly_model.tflite`는 `../10-tflite-deploy/app/model/`에
**이미 검증된 버전이 들어있어** 바로 다음 단계로 넘어가도 됩니다. 직접 학습한
결과로 교체하고 싶다면 다운로드해서 같은 경로에 덮어쓰세요.

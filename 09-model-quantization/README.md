# 9강 — Edge AI 모델 경량화 (Quantization)

설비 이상탐지 모델을 학습하고, Weight를 FP64 → FP16으로 낮춰 크기와
정확도 변화를 확인합니다. **EdgeX Foundry와는 무관한 별도 환경**입니다 —
모델 학습·양자화는 플랫폼이 아니라 ML 도구(scikit-learn)의 영역이라
3회차 스택을 켤 필요가 없습니다.

## 사전 준비

1. Docker Desktop 설치·실행
2. 이 폴더에서 `docker compose pull`

## 실행

```bash
docker compose up -d
```

브라우저에서 http://localhost:8888/?token=quant 접속 →
`work/quantization_lab.ipynb`를 열고 순서대로 실행합니다.

## 원본 코드에서 다듬은 점

제공된 답안 로직은 그대로 유지했고, 마지막에 **"심화" 셀을 하나 추가**했습니다.
원본 비교(865B → 245B, 71.7% 감소)에는 "정밀도 감소 효과"와
"sklearn 객체 메타데이터 제거 효과"가 섞여 있어서, coef/intercept만
동일한 형태(dict)로 저장해 순수하게 정밀도 효과만 비교하는 셀입니다.
이 부분은 몰라도 원 실습 진행에는 지장이 없고, 정확한 개념 설명을
원하실 때 참고하시면 됩니다.

## 자주 겪는 문제

- **8888 포트 충돌** → compose에서 왼쪽 포트만 변경
- **scikit-learn import 오류** → `jupyter/scipy-notebook` 이미지에는
  기본 포함이라 정상적으로는 발생하지 않습니다. 다른 이미지로 바뀌지
  않았는지 확인하세요.
- **초기화** → `docker compose down` 후 `up -d`

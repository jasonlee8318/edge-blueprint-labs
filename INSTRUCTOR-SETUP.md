# 강사용 — 깃허브 업로드 절차

## 1. 저장소 만들기

1. https://github.com/new 접속 (jasonlee8318 계정)
2. Repository name: **edge-blueprint-labs** (다른 이름을 쓰면 각 회차
   README의 링크·클론 명령을 함께 바꿔주세요)
3. Public 선택
4. "Add a README file" 체크는 **해제** (이미 README.md가 있어 충돌 방지)
5. Create repository

## 2. 업로드 — 방법 A: 웹에서 드래그 (Git 몰라도 됨)

1. 만들어진 저장소 화면에서 **uploading an existing file** 클릭
2. zip을 압축 해제한 `edge-blueprint-labs` 폴더 **안의 내용물**
   (`03-edgex-setup`, `05-edge-pipeline`, … , `README.md`, `.gitattributes`)을
   통째로 드래그 — 폴더 자체가 아니라 안의 파일들을 끌어야 저장소
   루트에 바로 놓입니다.
3. Commit changes

## 3. 업로드 — 방법 B: Git 명령

```bash
cd edge-blueprint-labs
git init
git add .
git commit -m "엣지 플랫폼 실습 초기 등록"
git branch -M main
git remote add origin https://github.com/jasonlee8318/edge-blueprint-labs.git
git push -u origin main
```

## 4. 업로드 후 확인

- 저장소 첫 화면에 루트 README.md가 렌더링되는지
- `10-tflite-deploy/app/model/motor_anomaly_model.tflite`처럼 바이너리
  파일도 깨지지 않고 올라갔는지 (Git이 텍스트로 오인해 손상시키는
  경우가 드물게 있어 다운로드해서 크기 확인 권장 — 정상 크기는 약 2.4KB)
- Code → Download ZIP으로 직접 한 번 받아 구조 확인

## 5. 수강생 공지문

> 1. Docker Desktop 설치 후 실행 (Windows는 설치 시 WSL2 옵션 체크,
>    메모리 6GB 이상 설정)
> 2. 실습 파일 내려받기: https://github.com/jasonlee8318/edge-blueprint-labs
>    → Code 버튼 → Download ZIP → 압축 해제 (경로에 한글/공백 없는 곳 권장)
> 3. `03-edgex-setup` 폴더에서 `docker compose pull` → `docker compose up -d`
>    (이후 회차는 이 스택을 계속 켜둔 채로 진행)

## 6. 진행 순서 (수업 순서 그대로)

```
03-edgex-setup           EdgeX 스택 기동 (과정 내내 유지)
  └─ 06-stream-pipeline   Motor01 디바이스 최초 등록 (EdgeX UI)
       ├─ 05-edge-pipeline
       ├─ 07-store-forward
       └─ 10-tflite-training → 10-tflite-deploy
            └─ 09-model-quantization  (EdgeX와 무관, 독립 진행 가능)
                 └─ 12-integration-verification  (전부 켠 뒤 캡스톤 검증)
```

Motor01 디바이스 등록은 6강에서 한 번만 하면 이후 회차가 전부
재사용합니다(`06-stream-pipeline/README.md`의 1단계 참고).

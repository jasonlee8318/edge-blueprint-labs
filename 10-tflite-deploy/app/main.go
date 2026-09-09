// ============================================================
// edge-tflite-infer
// 10강: Motor01 센서 값을 실제 TFLite 모델로 추론하는 EdgeX App Service
//
// go-tflite(mattn/go-tflite)로 진짜 TensorFlow Lite C API를 링크해서 씁니다.
// 이 코드는 샌드박스에서 실제로 학습한 모델(motor_anomaly_model.tflite)로
// Python Interpreter와 Go Interpreter의 출력이 소수점까지 일치하는 것을
// 확인한 뒤 작성했습니다 (0.6263745427... == 0.62637454).
//
// 6강과 구조는 비슷하지만 결정적으로 다른 점 하나: 6강은 "규칙"(온도 임계치)으로
// 판단했다면, 이 서비스는 "학습된 신경망"으로 판단합니다. 같은 Motor01 데이터를
// 두 가지 방식으로 각각 판정해보고 결과를 비교하는 것이 12강의 과제입니다.
// ============================================================

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"

	// go-tflite — TensorFlow Lite의 C API를 cgo로 감싼 바인딩입니다.
	// Dockerfile이 사전 컴파일된 라이브러리(buildkit)를 받아 링크해줍니다.
	tflite "github.com/mattn/go-tflite"
)

const (
	serviceKey  = "edge-tflite-infer"
	deviceName  = "Motor01"
	modelPath   = "./model/motor_anomaly_model.tflite" // 10-tflite-training에서 만든 모델
	scoreThresh = 0.5                                  // 이 값 이상이면 ANOMALY (원고의 sigmoid 출력 임계치)
)

// --------------------------------------------
// 1. TFLite 인터프리터 — 서비스 시작 시 딱 한 번만 로드해서 계속 재사용합니다.
//    (매 추론마다 모델을 다시 읽으면 느리고 불필요합니다.)
//    interpreter.Invoke()는 스레드 세이프하지 않다고 문서화되어 있어,
//    여러 요청이 동시에 들어와도 한 번에 하나씩만 추론하도록 mutex로 감쌉니다.
// --------------------------------------------

var (
	interpreterMu sync.Mutex
	interpreter   *tflite.Interpreter

	cacheMu sync.Mutex
	cache   = map[string]interface{}{"status": "WAITING"} // /dashboard가 반환할 최신 결과
)

func main() {
	service, ok := pkg.NewAppService(serviceKey)
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient()

	// ── 모델 로딩 3단계 (원고 5~8번: 변환→저장→로딩→Tensor 확인에 해당) ──
	// 1) 파일에서 모델 구조 읽기
	model := tflite.NewModelFromFile(modelPath)
	if model == nil {
		lc.Errorf("모델을 로드하지 못했습니다: %s", modelPath)
		os.Exit(-1)
	}
	defer model.Delete() // C 라이브러리가 할당한 메모리라 Go의 GC가 자동으로 못 치웁니다.

	// 2) 인터프리터 옵션 (스레드 수 등 세부 설정 — 여기서는 기본값 사용)
	options := tflite.NewInterpreterOptions()
	defer options.Delete()

	// 3) 실제 추론을 실행할 인터프리터 생성
	interpreter = tflite.NewInterpreter(model, options)
	defer interpreter.Delete()

	// 입력/출력 텐서를 위한 메모리를 미리 할당합니다 (매 추론마다 할 필요 없음).
	if status := interpreter.AllocateTensors(); status != tflite.OK {
		lc.Errorf("AllocateTensors 실패: %v", status)
		os.Exit(-1)
	}
	lc.Info("TFLite 모델 로드 완료 — Motor01 추론 준비됨")

	// 파이프라인: Motor01 이벤트만 통과시키고 runInference에서 실제 추론을 수행합니다.
	err := service.SetDefaultFunctionsPipeline(
		transforms.NewFilterFor([]string{deviceName}).FilterByDeviceName,
		runInference,
	)
	if err != nil {
		lc.Errorf("파이프라인 설정 실패: %s", err.Error())
		os.Exit(-1)
	}

	if err := service.AddRoute("/dashboard", dashboardHandler, http.MethodGet); err != nil {
		lc.Errorf("/dashboard 라우트 등록 실패: %s", err.Error())
		os.Exit(-1)
	}

	if err := service.Run(); err != nil {
		lc.Errorf("Run 실패: %s", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}

// --------------------------------------------
// 2. Motor01 이벤트 -> 정규화 -> TFLite 추론 -> NORMAL/ANOMALY 판정
//    (10강 원고의 9~13번 단계에 정확히 대응)
//
// ⚠️ device-rest는 POST 한 번당 Event 한 개(리딩 1개)를 만듭니다.
//    Temperature/Vibration이 한 이벤트에 같이 오지 않으므로, 최근 값을
//    저장해뒀다가 매 이벤트마다 최신 쌍으로 판단합니다.
// --------------------------------------------

var (
	sensorMu          sync.Mutex
	lastTemp, lastVib float64
	haveTemp, haveVib bool
)

func runInference(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	event, ok := data.(dtos.Event)
	if !ok {
		return false, nil
	}

	sensorMu.Lock()
	for _, r := range event.Readings {
		switch r.ResourceName {
		case "Temperature":
			if v, err := strconv.ParseFloat(r.Value, 64); err == nil {
				lastTemp, haveTemp = v, true
			}
		case "Vibration":
			if v, err := strconv.ParseFloat(r.Value, 64); err == nil {
				lastVib, haveVib = v, true
			}
		}
	}
	temp, vib, gotTemp, gotVib := lastTemp, lastVib, haveTemp, haveVib
	sensorMu.Unlock()

	if !gotTemp || !gotVib {
		return false, nil
	}

	// ── 정규화: 학습할 때와 반드시 같은 방식으로 스케일을 맞춰야 합니다 ──
	// (10-tflite-training 노트북의 X_scaled[:,0]/100, X_scaled[:,1]/10과 동일)
	// 학습 때와 다르게 정규화하면 모델이 엉뚱한 값을 보게 되어 추론이 무의미해집니다.
	inputTemp := float32(temp / 100.0)
	inputVib := float32(vib / 10.0)

	// ── 실제 추론 3단계: 입력 텐서에 값 쓰기 -> Invoke -> 출력 텐서 읽기 ──
	interpreterMu.Lock()
	input := interpreter.GetInputTensor(0) // 이 모델은 입력이 [온도, 진동] 2개짜리 벡터 하나입니다.
	fs := input.Float32s()
	fs[0] = inputTemp
	fs[1] = inputVib

	interpreter.Invoke() // 신경망의 순전파(forward pass)가 여기서 실행됩니다.

	output := interpreter.GetOutputTensor(0)
	score := output.Float32s()[0] // sigmoid 출력이라 0~1 사이 값 (1에 가까울수록 이상)
	interpreterMu.Unlock()

	status := "NORMAL"
	if score >= scoreThresh {
		status = "ANOMALY"
		ctx.LoggingClient().Warnf(">>> ALERT 설비 이상 가능성 감지 (score=%.4f, temp=%.1f, vibration=%.1f)",
			score, temp, vib)
	}

	cacheMu.Lock()
	cache = map[string]interface{}{
		"temperature":   temp,
		"vibration":     vib,
		"anomaly_score": round4(float64(score)),
		"status":        status,
	}
	cacheMu.Unlock()

	return true, nil
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	cacheMu.Lock()
	resp := cache
	cacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}

// ============================================================
// edge-store-forward
// 7강: "Cloud가 10초 동안 끊겼다면 데이터는 어떻게 처리할 것인가?"
//
// 이 실습의 핵심은 원고의 로직(local_storage 리스트 + retry_local_data())을
// 손으로 짜지 않는다는 점입니다. EdgeX App Functions SDK는 이 패턴을
// "Store and Forward"라는 이름으로 이미 내장하고 있습니다.
//
//   HTTPSender(url, mimeType, persistOnError=true)
//   -> 전송 실패 시 SDK가 자동으로 DB(Postgres)에 저장
//   -> Writable.StoreAndForward.RetryInterval마다 SDK가 자동으로 재전송 시도
//
// 즉, 원고의 9번 함수(retry_local_data)가 통째로 필요 없어집니다.
// 이게 "왜 플랫폼을 쓰는가"에 대한 이번 강의의 답입니다.
// ============================================================

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"

	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
)

const serviceKey = "edge-store-forward"

// --------------------------------------------
// 1. 설정값 (원고와 동일)
// --------------------------------------------

const (
	summarySize     = 5
	tempThreshold   = 90.0
	vibrationThresh = 7.0
	deviceName      = "Motor01"
	mockCloudURLEnv = "MOCK_CLOUD_URL" // 컨테이너 환경변수로 목적지를 바꿀 수 있게 함
)

// --------------------------------------------
// 2. 정상 데이터 Buffer — 5건 모이면 Summary를 만들고 비웁니다
// --------------------------------------------

var (
	mu           sync.Mutex
	normalBuffer []map[string]float64
)

func main() {
	service, ok := pkg.NewAppService(serviceKey)
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient()

	// mock-cloud 주소는 환경변수로 주입받되, 없으면 기본값(컨테이너 이름 기반)을 씁니다.
	mockCloudURL := os.Getenv(mockCloudURLEnv)
	if mockCloudURL == "" {
		mockCloudURL = "http://mock-cloud:8090/ingest"
	}

	// 파이프라인은 4단계입니다. SDK가 제공하는 조립식 블록(transforms)을
	// 우리가 만든 분류 함수 뒤에 이어 붙이는 방식으로 동작합니다.
	//   1) FilterByDeviceName : Motor01 이벤트만 통과
	//   2) classifyAndBuild   : 검증→분류→집계 (우리가 직접 작성한 로직)
	//   3) CompressWithGZIP   : 전송 직전 데이터를 gzip으로 압축 (원고 6번 "데이터 압축")
	//   4) HTTPSender.HTTPPost: mock-cloud로 전송. 세 번째 인자 true가
	//      "실패하면 SDK가 알아서 저장해뒀다가 나중에 재전송하라"는 스위치입니다.
	//      (=Store and Forward. 이게 이번 강의의 핵심 기능입니다.)
	err := service.SetDefaultFunctionsPipeline(
		transforms.NewFilterFor([]string{deviceName}).FilterByDeviceName,
		classifyAndBuild,
		transforms.NewCompression().CompressWithGZIP,
		transforms.NewHTTPSender(mockCloudURL, "application/json", true).HTTPPost,
	)
	if err != nil {
		lc.Errorf("파이프라인 설정 실패: %s", err.Error())
		os.Exit(-1)
	}

	lc.Infof("edge-store-forward 시작 — Cloud 엔드포인트: %s", mockCloudURL)

	if err := service.Run(); err != nil {
		lc.Errorf("Run 실패: %s", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}

// --------------------------------------------
// 4. 검증 -> 이상/정상 분류 -> (이상: 즉시 이벤트) / (정상: 5건 Summary)
//    원고의 validate() + detect_anomaly() + create_summary()에 대응
//
// ⚠️ device-rest는 POST 한 번당 Event 한 개(리딩 1개)를 만듭니다.
//    Temperature/Vibration이 한 이벤트에 같이 오지 않으므로, 최근 값을
//    저장해뒀다가 매 이벤트마다 최신 쌍으로 판단합니다.
// --------------------------------------------

var lastTemp, lastVib float64
var haveTemp, haveVib bool

// classifyAndBuild가 반환하는 (bool, interface{})의 두 번째 값(payload)이
// 그대로 다음 파이프라인 단계(압축 -> 전송)로 넘어갑니다. false를 반환하면
// 뒤 단계(압축/전송)는 아예 실행되지 않습니다.
func classifyAndBuild(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	event, ok := data.(dtos.Event)
	if !ok {
		return false, nil
	}

	mu.Lock()
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
	mu.Unlock()

	// 아직 두 값이 다 안 모였으면 처리할 게 없으므로 종료합니다.
	if !gotTemp || !gotVib {
		return false, nil
	}

	// 4-1. Validation — 허용 범위를 벗어나면 폐기 (센서 오류로 추정되는 값 필터링)
	if !(0 <= temp && temp <= 120) || !(0 <= vib && vib <= 20) {
		ctx.LoggingClient().Warn("INVALID DATA — 허용 범위를 벗어나 폐기")
		return false, nil
	}

	// 4-2. 이상 Event — 즉시 전송 (Buffer를 거치지 않고 곧바로 압축/전송 단계로)
	if temp >= tempThreshold || vib >= vibrationThresh {
		ctx.LoggingClient().Warnf(">>> ANOMALY EVENT temp=%.1f vibration=%.1f", temp, vib)
		payload, _ := json.Marshal(map[string]interface{}{
			"type":        "ANOMALY",
			"temperature": temp,
			"vibration":   vib,
		})
		return true, payload // true + 데이터 -> 다음 단계(압축)로 전달됨
	}

	// 4-3. 정상 데이터 — Buffer에 쌓고 summarySize건이 안 되면 파이프라인 종료(false)
	// (여기서 false를 반환하면 아직 보낼 게 없다는 뜻이라 압축/전송 단계가 건너뛰어집니다.)
	mu.Lock()
	normalBuffer = append(normalBuffer, map[string]float64{"temperature": temp, "vibration": vib})
	count := len(normalBuffer)

	if count < summarySize {
		mu.Unlock()
		ctx.LoggingClient().Debugf("NORMAL BUFFER : %d/%d", count, summarySize)
		return false, nil // 아직 전송할 게 없음 — 여기서 파이프라인 중단
	}

	// summarySize건이 모였으므로 통계를 낸 뒤 Buffer를 비웁니다.
	buffered := normalBuffer
	normalBuffer = nil
	mu.Unlock()

	var sumTemp, sumVib, maxTemp, maxVib float64
	for _, r := range buffered {
		sumTemp += r["temperature"]
		sumVib += r["vibration"]
		if r["temperature"] > maxTemp {
			maxTemp = r["temperature"]
		}
		if r["vibration"] > maxVib {
			maxVib = r["vibration"]
		}
	}

	summary := map[string]interface{}{
		"type":            "SUMMARY",
		"count":           count,
		"avg_temperature": round2(sumTemp / float64(count)),
		"max_temperature": maxTemp,
		"avg_vibration":   round2(sumVib / float64(count)),
		"max_vibration":   maxVib,
	}
	payload, _ := json.Marshal(summary)

	ctx.LoggingClient().Infof("SUMMARY 생성 및 전송: %+v", summary)
	return true, payload // 압축 -> 전송 단계로 넘어감 (전송 실패 시 Store&Forward 발동)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

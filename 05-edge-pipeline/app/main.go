// ============================================================
// edge-sensor-classifier
// 5강: 엣지 센서 데이터 처리 파이프라인
//
// Raw Data -> Validation -> Filtering -> Aggregation
//          -> Anomaly Detection -> Transmission Classification
//
// 원고의 파이썬 스크립트는 하드코딩된 10건을 한 번에 돌며 4개 버킷
// (invalid_data / normal_data / anomaly_events / periodic_summary)으로
// 분류만 하고 끝났습니다. 이 버전은 Motor01의 실시간 이벤트를 대상으로
// 같은 분류를 계속 수행하며, 각 버킷에 맞는 실제 동작(로그 표시)까지 합니다.
// ============================================================

package main

import (
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
)

const (
	serviceKey       = "edge-sensor-classifier" // EdgeX 안에서 이 서비스를 식별하는 이름
	deviceName       = "Motor01"                // 구독할 디바이스
	summarySize      = 5                        // 정상 데이터를 몇 건 모아 하나의 요약(Summary)으로 만들지
	tempThreshold    = 90.0                     // 이 값 이상이면 이상(단건 기준 — 6강의 "평균" 기준과 다름)
	vibrationThresh  = 7.0
	tempMin, tempMax = 0.0, 120.0 // 이 범위를 벗어나면 "허용 범위 초과"로 폐기
	vibMin, vibMax   = 0.0, 20.0
)

// --------------------------------------------
// 1. 상태 변수
//    - lastTemp/lastVib/haveTemp/haveVib: 최근값 캐시
//      (device-rest는 리딩 1개당 Event 1개를 만들기 때문에, 온도와 진동이
//       같은 이벤트에 함께 오지 않습니다. 각각 저장해뒀다가 매번 최신 쌍으로 판단합니다.)
//    - normalBuffer: 정상 판정된 데이터를 summarySize건 모을 때까지 담아두는 곳
//      (원고의 5번 "정상 데이터 Summary 생성"에 해당)
//    - counts: 지금까지 몇 건이 각 버킷으로 분류됐는지 누적 (원고 8번 최종 요약)
//    - lastStatus: 가장 최근 이벤트가 어떤 경로로 분류됐는지 (/status로 조회)
// --------------------------------------------

var (
	mu                sync.Mutex
	lastTemp, lastVib float64
	haveTemp, haveVib bool

	normalBuffer []map[string]float64

	counts = map[string]int{
		"total": 0, "invalid": 0, "normal": 0, "anomaly": 0, "summary_sent": 0,
	}

	lastStatus interface{} = map[string]interface{}{"stage": "WAITING"}
)

func main() {
	service, ok := pkg.NewAppService(serviceKey)
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient()

	// 파이프라인은 딱 두 단계입니다:
	// 1) Motor01 디바이스의 이벤트만 통과시키고 (다른 디바이스는 무시)
	// 2) classify 함수에서 검증→필터링→집계→이상탐지→분류를 전부 수행합니다.
	err := service.SetDefaultFunctionsPipeline(
		transforms.NewFilterFor([]string{deviceName}).FilterByDeviceName,
		classify,
	)
	if err != nil {
		lc.Errorf("파이프라인 설정 실패: %s", err.Error())
		os.Exit(-1)
	}

	// /status — 지금까지의 분류 통계와 가장 최근 판정 결과를 조회하는 엔드포인트
	if err := service.AddCustomRoute("/status", interfaces.Unauthenticated, statusHandler, http.MethodGet); err != nil {
		lc.Errorf("/status 라우트 등록 실패: %s", err.Error())
		os.Exit(-1)
	}

	lc.Info("edge-sensor-classifier 시작 — Motor01 이벤트를 분류합니다.")

	if err := service.Run(); err != nil {
		lc.Errorf("Run 실패: %s", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}

// --------------------------------------------
// 2. 검증 -> 필터링 -> 집계 -> 이상탐지 -> 전송정책 분류
//    원고의 4-1(검증) ~ 6(전송 정책 분류)에 정확히 대응합니다.
//    각 return 문 앞의 주석이 원고의 어느 단계인지를 표시합니다.
// --------------------------------------------

func classify(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	event, ok := data.(dtos.Event)
	if !ok {
		return false, nil
	}

	mu.Lock()
	defer mu.Unlock()

	// 이번 이벤트가 Temperature인지 Vibration인지 확인하고 최근값을 갱신합니다.
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

	// ── [1단계: 데이터 검증] 온도/진동 둘 다 아직 한 번도 안 왔으면 결측과 동일 취급 ──
	// 원고의 "temp is None" 같은 결측 상황을 실시간 스트림 버전으로 표현한 것입니다.
	if !haveTemp || !haveVib {
		counts["invalid"]++
		lastStatus = map[string]interface{}{"stage": "INVALID_DATA", "reason": "결측값 존재"}
		ctx.LoggingClient().Warn("INVALID DATA : 결측값 존재")
		return false, nil
	}
	temp, vib := lastTemp, lastVib
	counts["total"]++

	// ── [2단계: 필터링] 물리적으로 말이 안 되는 값(센서 오류로 추정)은 폐기 ──
	// 예: 진동 센서가 고장 나서 -999 같은 값을 보내는 경우를 걸러내는 역할입니다.
	if temp < tempMin || temp > tempMax || vib < vibMin || vib > vibMax {
		counts["invalid"]++
		lastStatus = map[string]interface{}{
			"stage": "INVALID_DATA", "reason": "허용 범위 초과",
			"temperature": temp, "vibration": vib,
		}
		ctx.LoggingClient().Warnf("INVALID DATA : 허용 범위 초과 (temp=%.1f, vib=%.1f)", temp, vib)
		return false, nil
	}

	// ── [3단계: 이상탐지 -> 즉시 전송] ──
	// 원고의 "이상 발생 즉시 Cloud 전송"에 해당합니다. Buffer에 쌓아 기다리지 않고
	// 바로 처리한다는 점이 아래 "정상 데이터" 경로와의 핵심 차이입니다.
	if temp >= tempThreshold || vib >= vibrationThresh {
		counts["anomaly"]++
		lastStatus = map[string]interface{}{
			"stage": "IMMEDIATE_SEND", "event_type": "ANOMALY",
			"temperature": temp, "vibration": vib,
		}
		ctx.LoggingClient().Warnf(">>> IMMEDIATE SEND (ANOMALY) temp=%.1f vib=%.1f", temp, vib)
		return true, lastStatus
	}

	// ── [4단계: 정상 데이터 -> 집계] ──
	// 정상 데이터는 한 건씩 바로 보내지 않고 일단 모아둡니다(Local 저장에 해당).
	counts["normal"]++
	normalBuffer = append(normalBuffer, map[string]float64{"temperature": temp, "vibration": vib})

	if len(normalBuffer) < summarySize {
		// 아직 summarySize건이 안 모였으므로 "Local 저장" 상태로 남겨두고 종료합니다.
		lastStatus = map[string]interface{}{
			"stage": "LOCAL_STORE", "buffered": len(normalBuffer), "of": summarySize,
		}
		ctx.LoggingClient().Debugf("LOCAL STORE : %d/%d", len(normalBuffer), summarySize)
		return false, nil
	}

	// summarySize건이 모였으므로 통계를 낸 뒤 Buffer를 비웁니다 — "주기 전송" 시점입니다.
	buffered := normalBuffer
	normalBuffer = nil
	counts["summary_sent"]++

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
		"stage":           "PERIODIC_SEND",
		"count":           len(buffered),
		"avg_temperature": round2(sumTemp / float64(len(buffered))),
		"max_temperature": maxTemp,
		"avg_vibration":   round2(sumVib / float64(len(buffered))),
		"max_vibration":   maxVib,
	}
	lastStatus = summary
	ctx.LoggingClient().Infof("PERIODIC SEND (SUMMARY) : %+v", summary)
	return true, summary
}

// --------------------------------------------
// 3. 현재까지의 분류 통계 조회 — 원고 8번(최종 처리 흐름 요약)에 대응
// --------------------------------------------

// v4 SDK는 표준 net/http가 아니라 Echo 프레임워크로 라우트를 처리합니다.
// 핸들러가 (http.ResponseWriter, *http.Request) 대신 (echo.Context) error를 받고,
// c.JSON()으로 상태 코드+JSON 응답을 한 번에 보냅니다.
func statusHandler(c echo.Context) error {
	mu.Lock()
	resp := map[string]interface{}{
		"counts":      counts,     // 지금까지 누적된 버킷별 건수
		"last_status": lastStatus, // 가장 최근 이벤트가 어디로 분류됐는지
	}
	mu.Unlock()

	return c.JSON(http.StatusOK, resp)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

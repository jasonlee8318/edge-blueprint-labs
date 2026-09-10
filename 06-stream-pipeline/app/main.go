// ============================================================
// edge-stream-pipeline
// 6강: Sensor -> Buffer -> Window Processing -> Event Detection -> Cache
// EdgeX Foundry의 App Functions SDK로 구현한 진짜 App Service입니다.
//
// device-rest가 Motor02(Motor02-Vibration-Sensor 프로필)로부터 받은
// Temperature/Vibration Reading을 Core Data -> Message Bus를 거쳐
// 이 서비스가 실시간으로 수신하면서 파이프라인을 실행합니다.
// ============================================================

package main

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	// App Functions SDK — EdgeX 위에서 도는 "App Service"를 만들 때 쓰는 표준 도구입니다.
	// pkg:        서비스 생성/실행의 진입점 (NewAppService, Run)
	// interfaces: 파이프라인 함수가 받는 컨텍스트/로거 등의 타입
	// transforms: SDK가 미리 만들어둔 재사용 가능한 파이프라인 조각들 (필터, 전송 등)
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/transforms"

	// go-mod-core-contracts — EdgeX 전체가 공유하는 데이터 모델(Event, Reading 등)입니다.
	// Core Data가 우리에게 넘겨주는 데이터는 항상 이 dtos.Event 형태입니다.
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
)

// serviceKey는 이 App Service를 EdgeX 안에서 식별하는 이름입니다.
// core-keeper(레지스트리)에 이 이름으로 등록되고, 로그에도 이 이름이 찍힙니다.
const serviceKey = "edge-stream-pipeline"

// --------------------------------------------
// 1. 설정값 (6강 원고와 동일)
// --------------------------------------------

const (
	windowSize       = 5               // 최근 몇 건을 하나의 Window(분석 범위)로 볼 것인가
	tempAnomaly      = 80.0            // 평균 온도가 이 값 이상이면 이상(ANOMALY)
	vibrationAnomaly = 5.0             // 평균 진동이 이 값 이상이면 이상(ANOMALY)
	cacheTTL         = 5 * time.Second // Cache에 저장된 결과를 "신선하다"고 볼 수 있는 최대 시간
	deviceName       = "Motor02"       // 이 서비스가 구독할 디바이스 이름
)

// --------------------------------------------
// 2. Buffer(슬라이딩 윈도우)와 Cache
//    — HTTP 요청(대시보드 조회)과 센서 이벤트 처리가 동시에 이 값들을
//      건드릴 수 있으므로 mu(mutex)로 감싸서 데이터가 깨지지 않게 합니다.
// --------------------------------------------

// reading은 Window(슬라이딩 버퍼) 안에 쌓이는 값 하나(온도+진동 한 쌍)입니다.
type reading struct {
	temperature float64
	vibration   float64
}

// cacheEntry는 "가장 최근에 계산된 결과"를 담습니다.
// 대시보드가 매번 원본 데이터를 다시 계산하지 않고 이 구조체만 읽으면 되는 게
// 바로 6강이 말하는 "Cache" 개념입니다.
type cacheEntry struct {
	Status         string  `json:"status"`                    // WAITING / NORMAL / ANOMALY
	AvgTemperature float64 `json:"avg_temperature,omitempty"` // 최근 5건 평균 온도
	AvgVibration   float64 `json:"avg_vibration,omitempty"`   // 최근 5건 평균 진동
	CachedAt       int64   `json:"cached_at"`                 // 이 결과를 계산한 시각(Unix time)
}

var (
	mu           sync.Mutex // streamBuffer, cache, lastTemp/lastVib를 보호하는 잠금장치
	streamBuffer []reading  // 슬라이딩 윈도우 — 최근 windowSize건만 유지
	cache        cacheEntry // 대시보드가 읽어가는 최신 결과
)

func main() {
	// NewAppService — EdgeX 설정 파일(res/configuration.yaml)을 읽고,
	// core-keeper에 등록하고, Message Bus 연결까지 전부 알아서 준비해줍니다.
	// 이 한 줄 뒤로는 "이 서비스는 EdgeX의 정식 구성원이다"라고 보시면 됩니다.
	service, ok := pkg.NewAppService(serviceKey)
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient() // EdgeX 표준 로거 — 다른 서비스와 같은 형식으로 로그가 남습니다.

	// 파이프라인 = 이벤트가 들어올 때마다 순서대로 실행되는 함수들의 목록입니다.
	// 1) FilterByDeviceName: Motor02이 아닌 다른 디바이스의 이벤트는 여기서 걸러냅니다.
	//    (플랫폼에는 여러 디바이스가 있을 수 있으므로, 우리 서비스와 무관한 이벤트로
	//     아래 로직이 낭비되지 않도록 막아주는 역할입니다.)
	// 2) processReading: 실제 Window/이상탐지/Cache 로직 (5번 섹션에서 정의)
	err := service.SetDefaultFunctionsPipeline(
		transforms.NewFilterFor([]string{deviceName}).FilterByDeviceName,
		processReading,
	)
	if err != nil {
		lc.Errorf("파이프라인 설정 실패: %s", err.Error())
		os.Exit(-1)
	}

	// AddRoute — 이 App Service에 우리만의 HTTP 엔드포인트를 하나 추가합니다.
	// "Dashboard가 Cache의 최신 결과를 조회한다"는 원고의 문장을 그대로 구현한 것입니다.
	if err := service.AddCustomRoute("/dashboard", interfaces.Unauthenticated, dashboardHandler, http.MethodGet); err != nil {
		lc.Errorf("/dashboard 라우트 등록 실패: %s", err.Error())
		os.Exit(-1)
	}

	lc.Info("edge-stream-pipeline 시작 — Motor02 이벤트를 기다립니다.")

	// Run()은 서비스를 계속 실행 상태로 유지합니다 (여기서 블로킹됩니다).
	// 이벤트가 들어올 때마다 위에서 설정한 파이프라인이 자동으로 호출됩니다.
	if err := service.Run(); err != nil {
		lc.Errorf("Run 실패: %s", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}

// --------------------------------------------
// 5. 파이프라인 핵심 함수 — Window 계산, 이상 탐지, Cache 갱신
//    (기존 6강 파이썬 실습의 process_window/detect_event/update_cache와 동일한 로직)
//
// ⚠️ device-rest는 POST 한 번당 Event 한 개(리딩 1개)를 만듭니다.
//    Temperature/Vibration을 따로 POST하면 절대 한 이벤트에 같이 오지 않으므로,
//    "최근에 들어온 값"을 각각 저장해뒀다가 매 이벤트마다 최신 쌍으로 판단합니다.
// --------------------------------------------

var (
	lastTemp, lastVib float64 // 가장 최근에 도착한 온도/진동 값
	haveTemp, haveVib bool    // 아직 한 번도 안 왔으면 false (서비스 시작 직후 등)
)

// processReading은 파이프라인의 두 번째 단계로, Motor02 이벤트가 들어올 때마다 호출됩니다.
// 반환값 (bool, interface{})의 bool이 false면 "여기서 파이프라인을 멈춘다"는 뜻이고,
// true면 다음 단계(있다면)로 데이터를 넘긴다는 뜻입니다. 이 서비스는 이 함수가
// 마지막 단계라서 true를 반환해도 별도로 이어지는 곳은 없습니다.
func processReading(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	// data는 원래 interface{}(무엇이든 될 수 있는 타입)로 넘어오므로,
	// 우리가 기대하는 실제 타입(dtos.Event)인지 먼저 확인(타입 단언)합니다.
	event, ok := data.(dtos.Event)
	if !ok {
		ctx.LoggingClient().Warn("dtos.Event 타입이 아닌 데이터가 들어와 건너뜁니다.")
		return false, nil
	}

	// 이번에 도착한 이벤트가 Temperature인지 Vibration인지에 따라
	// 최근값 변수를 갱신합니다. (둘 다 갱신되는 게 아니라 매번 둘 중 하나만 갱신됨)
	mu.Lock()
	for _, r := range event.Readings {
		switch r.ResourceName {
		case "Temperature":
			// EdgeX의 Reading.Value는 항상 문자열이라 float64로 직접 변환해야 합니다.
			if v, err := strconv.ParseFloat(r.Value, 64); err == nil {
				lastTemp, haveTemp = v, true
			}
		case "Vibration":
			if v, err := strconv.ParseFloat(r.Value, 64); err == nil {
				lastVib, haveVib = v, true
			}
		}
	}
	// 잠금 안에서 읽은 값을 지역 변수에 복사해두고 곧바로 잠금을 풀어줍니다.
	// (아래 로직을 잠금 상태로 오래 끌지 않기 위한 습관입니다.)
	temp, vib, gotTemp, gotVib := lastTemp, lastVib, haveTemp, haveVib
	mu.Unlock()

	// 온도/진동 둘 다 최소 한 번씩은 들어온 상태여야 다음 단계로 진행합니다.
	// 원고의 preprocess() 검증 단계에 해당합니다 — 값이 없으면 그냥 넘어갑니다.
	if !gotTemp || !gotVib {
		return false, nil
	}

	mu.Lock()
	defer mu.Unlock()

	// ── Buffer/Window: 최근 windowSize건만 유지하는 "슬라이딩 윈도우" ──
	streamBuffer = append(streamBuffer, reading{temperature: temp, vibration: vib})
	if len(streamBuffer) > windowSize {
		// 맨 앞(오래된 것)을 잘라내고 최근 것만 남깁니다 — 이게 "Sliding"의 의미입니다.
		streamBuffer = streamBuffer[len(streamBuffer)-windowSize:]
	}

	if len(streamBuffer) < windowSize {
		// 아직 5건이 안 모였으면 평균을 낼 수 없으므로 대기 상태로 표시만 하고 끝냅니다.
		cache = cacheEntry{Status: "WAITING", CachedAt: time.Now().Unix()}
		return true, nil
	}

	// ── Window 기반 분석: 최근 5건의 평균 계산 ──
	var sumTemp, sumVib float64
	for _, r := range streamBuffer {
		sumTemp += r.temperature
		sumVib += r.vibration
	}
	avgTemp := sumTemp / windowSize
	avgVib := sumVib / windowSize

	// ── 이상 Event 탐지: 평균이 기준치를 넘으면 ANOMALY ──
	status := "NORMAL"
	if avgTemp >= tempAnomaly || avgVib >= vibrationAnomaly {
		status = "ANOMALY"
		ctx.LoggingClient().Warnf(">>> ALERT 이상 상태 감지! avg_temp=%.2f avg_vibration=%.2f", avgTemp, avgVib)
	}

	// ── Cache 갱신: 방금 계산한 결과를 저장 ──
	// 대시보드가 조회할 때 이 buffer를 처음부터 다시 계산하지 않고,
	// 여기 저장된 값만 읽어가면 되게 하는 것이 캐싱의 핵심입니다.
	cache = cacheEntry{
		Status:         status,
		AvgTemperature: round2(avgTemp),
		AvgVibration:   round2(avgVib),
		CachedAt:       time.Now().Unix(),
	}

	return true, nil
}

// --------------------------------------------
// 6. Dashboard 조회 — TTL을 확인해 만료 여부를 함께 응답
//    (2-4강 핵심 메시지: 캐싱은 "저장"이 아니라 "최신성 유지 정책"이 핵심)
// --------------------------------------------

func dashboardHandler(c echo.Context) error {
	mu.Lock()
	entry := cache // 잠금 상태에서 값을 복사해온 뒤 곧바로 풀어줍니다.
	mu.Unlock()

	// 이 결과가 계산된 지 얼마나 지났는지 계산합니다.
	age := time.Since(time.Unix(entry.CachedAt, 0))

	resp := map[string]interface{}{
		"status":          entry.Status,
		"avg_temperature": entry.AvgTemperature,
		"avg_vibration":   entry.AvgVibration,
		"age_seconds":     age.Seconds(),
		// TTL(cacheTTL)보다 오래된 결과라면 "낡았다(stale)"고 표시합니다.
		// 센서가 멈추면 이 값이 true로 바뀌는 걸로 TTL 개념을 직접 확인할 수 있습니다.
		"stale": age > cacheTTL,
	}

	return c.JSON(http.StatusOK, resp)
}

// round2는 소수점 둘째 자리까지 반올림하는 작은 도우미 함수입니다.
// (0.5를 더한 뒤 정수로 자르는 건 반올림을 구현하는 흔한 방식입니다.)
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

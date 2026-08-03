package data

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// AurumRatesApi AURUM Rates 免费大宗商品 spot 数据
type AurumRatesApi struct{}

// SpotResponse AURUM Rates 接口返回结构
type SpotResponse struct {
	Status   string `json:"status"`
	Plan     string `json:"plan"`
	Currency string `json:"currency"`
	Data     *SpotData `json:"data"`
}

type SpotData struct {
	Gold      *MetalSpot `json:"gold"`
	Silver    *MetalSpot `json:"silver"`
	Platinum  *MetalSpot `json:"platinum"`
	Palladium *MetalSpot `json:"palladium"`
	CrudeWTI  *MetalSpot `json:"crude-wti"`
	CrudeBrent *MetalSpot `json:"crude-brent"`
	UpdatedAt string     `json:"updated_at"`
}

type MetalSpot struct {
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangePct  float64 `json:"change_pct"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Currency   string  `json:"currency"`
	Unit       string  `json:"unit"`
	Updated    string  `json:"updated"`
}

func (a *AurumRatesApi) fetchSpot() (*SpotResponse, error) {
	resp, err := SharedHTTPClient.SetTimeout(15 * time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get("https://aurumrates.com/api/v1/spot")
	if err != nil {
		return nil, fmt.Errorf("aurum rates request: %w", err)
	}

	var result SpotResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("aurum rates parse: %w", err)
	}
	if result.Data == nil || (result.Data.Gold == nil && result.Data.Silver == nil && result.Data.CrudeWTI == nil) {
		return nil, fmt.Errorf("aurum rates empty response")
	}
	return &result, nil
}

func codeToAurumSymbol(code string) string {
	switch strings.ToUpper(code) {
	case "XAUUSD", "XAU":
		return "gold"
	case "XAGUSD", "XAG":
		return "silver"
	case "USCL":
		return "crude-wti"
	case "USCO":
		return "crude-brent"
	default:
		return ""
	}
}

// GetQuote 获取现货实时行情
func (a *AurumRatesApi) GetQuote(code string) (*datasource.QuoteData, error) {
	symbol := codeToAurumSymbol(code)
	if symbol == "" {
		return nil, fmt.Errorf("aurum rates unsupported code: %s", code)
	}

	spot, err := a.fetchSpot()
	if err != nil {
		return nil, err
	}

	var m *MetalSpot
	switch symbol {
	case "gold":
		m = spot.Data.Gold
	case "silver":
		m = spot.Data.Silver
	case "crude-wti":
		m = spot.Data.CrudeWTI
	case "crude-brent":
		m = spot.Data.CrudeBrent
	}
	if m == nil {
		return nil, fmt.Errorf("aurum rates missing %s data", symbol)
	}

	logger.SugaredLogger.Infof("AURUM Rates quote %s: price=%.2f change=%.2f", code, m.Price, m.Change)
	return &datasource.QuoteData{
		Code:      code,
		Name:      code,
		Price:     m.Price,
		Change:    m.Change,
		ChangePct: m.ChangePct,
		High:      m.High,
		Low:       m.Low,
		Time:      time.Now(),
	}, nil
}

// GetKLine AURUM Rates 不提供 K 线，返回错误
func (a *AurumRatesApi) GetKLine(code, period string, count int) ([]datasource.KLineBar, error) {
	return nil, fmt.Errorf("aurum rates does not provide kline data")
}

// parseFloat helper
func parseAurumFloat(v interface{}) float64 {
	s := fmt.Sprintf("%v", v)
	if s == "" || s == "null" {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

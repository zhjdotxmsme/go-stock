package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EtfFlowApi fetches ETF holdings and flow data from Yahoo Finance
type EtfFlowApi struct {
	client *http.Client
	mu     sync.RWMutex
	cache  map[string]*etfFlowCacheEntry
}

type etfFlowCacheEntry struct {
	Data      *EtfFlowData
	CreatedAt time.Time
}

// EtfFlowData represents ETF holdings and flow information
type EtfFlowData struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	Holdings     float64 `json:"holdings"`      // 持仓量（盎司/桶等）
	HoldingsUnit string  `json:"holdings_unit"`
	Nav          float64 `json:"nav"`           // 单位净值
	TotalAssets  float64 `json:"total_assets"`  // 总资产规模
	Flow1Day     float64 `json:"flow_1d"`       // 1日资金流向（估算）
	Flow1Week    float64 `json:"flow_1w"`       // 1周资金流向
	Flow1Month   float64 `json:"flow_1m"`      // 1月资金流向
	UpdatedAt    time.Time
}

// NewEtfFlowApi creates a new ETF flow API client
func NewEtfFlowApi() *EtfFlowApi {
	return &EtfFlowApi{
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  make(map[string]*etfFlowCacheEntry),
	}
}

// GetEtfFlow fetches ETF holdings and flow data for a symbol
func (e *EtfFlowApi) GetEtfFlow(symbol string) (*EtfFlowData, error) {
	e.mu.RLock()
	if cached, ok := e.cache[symbol]; ok && time.Since(cached.CreatedAt) < 24*time.Hour {
		e.mu.RUnlock()
		return cached.Data, nil
	}
	e.mu.RUnlock()

	data, err := e.fetchFromYahoo(symbol)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.cache[symbol] = &etfFlowCacheEntry{Data: data, CreatedAt: time.Now()}
	e.mu.Unlock()

	return data, nil
}

// fetchFromYahoo fetches ETF data from Yahoo Finance quote summary API
func (e *EtfFlowApi) fetchFromYahoo(symbol string) (*EtfFlowData, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=assetProfile,defaultKeyStatistics,summaryDetail", symbol)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("etf flow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yahoo quote summary returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		QuoteSummary struct {
			Result []struct {
				DefaultKeyStatistics struct {
					TotalAssets struct {
						Raw float64 `json:"raw"`
					} `json:"totalAssets"`
					NavPrice struct {
						Raw float64 `json:"raw"`
					} `json:"navPrice"`
					HoldingsTurnover struct {
						Raw float64 `json:"raw"`
					} `json:"holdingsTurnover"`
				} `json:"defaultKeyStatistics"`
				SummaryDetail struct {
					TotalAssets struct {
						Raw float64 `json:"raw"`
					} `json:"totalAssets"`
					PreviousClose struct {
						Raw float64 `json:"raw"`
					} `json:"previousClose"`
					Volume struct {
						Raw int64 `json:"raw"`
					} `json:"volume"`
					AverageVolume10Day struct {
						Raw int64 `json:"raw"`
					} `json:"averageVolume10days"`
					AverageVolume3Month struct {
						Raw int64 `json:"raw"`
					} `json:"averageVolume3Month"`
					FiftyTwoWeekHigh struct {
						Raw float64 `json:"raw"`
					} `json:"fiftyTwoWeekHigh"`
					FiftyTwoWeekLow struct {
						Raw float64 `json:"raw"`
					} `json:"fiftyTwoWeekLow"`
				} `json:"summaryDetail"`
				AssetProfile struct {
					LongBusinessSummary string `json:"longBusinessSummary"`
				} `json:"assetProfile"`
			} `json:"result"`
			Error interface{} `json:"error"`
		} `json:"quoteSummary"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode etf flow response: %w", err)
	}

	if len(result.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no etf data for %s", symbol)
	}

	r := result.QuoteSummary.Result[0]
	data := &EtfFlowData{
		Symbol:      symbol,
		Name:        r.AssetProfile.LongBusinessSummary,
		TotalAssets: r.DefaultKeyStatistics.TotalAssets.Raw,
		Nav:         r.DefaultKeyStatistics.NavPrice.Raw,
		UpdatedAt:   time.Now(),
	}

	// Estimate holdings from total assets / spot price
	// For gold ETFs: 1 troy ounce = ~$X, so holdings = totalAssets / price
	// We'll leave holdings empty and let the caller provide spot price
	return data, nil
}

// EstimateHoldings calculates estimated physical holdings from total assets
func (e *EtfFlowApi) EstimateHoldings(totalAssets, spotPrice float64, unit string) (float64, string) {
	if totalAssets <= 0 || spotPrice <= 0 {
		return 0, unit
	}
	holdings := totalAssets / spotPrice
	return holdings, unit
}

// CalculateFlow estimates 1-day flow from volume * price change
func (e *EtfFlowApi) CalculateFlow(volume int64, prevClose, currentPrice float64) float64 {
	if volume <= 0 || prevClose <= 0 {
		return 0
	}
	avgPrice := (prevClose + currentPrice) / 2
	return float64(volume) * avgPrice
}

// GetCommodityEtfMapping returns ETF symbols for commodities
func GetCommodityEtfMapping(code string) string {
	mapping := map[string]string{
		"XAUUSD": "GLD",    // SPDR Gold Shares
		"XAGUSD": "SLV",    // iShares Silver Trust
		"USCL":   "USO",    // United States Oil Fund
	}
	return mapping[code]
}

// FormatFlow converts flow to human-readable string (millions/billions)
func FormatFlow(amount float64) string {
	if amount >= 1e9 {
		return fmt.Sprintf("%.2fB", amount/1e9)
	}
	if amount >= 1e6 {
		return fmt.Sprintf("%.2fM", amount/1e6)
	}
	return fmt.Sprintf("%.0f", amount)
}

// ParseYahooNumber parses a string number from Yahoo (handles suffixes like K, M, B)
func ParseYahooNumber(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" || s == "N/A" {
		return 0, nil
	}
	multiplier := 1.0
	if strings.HasSuffix(s, "K") {
		multiplier = 1e3
		s = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1e6
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "B") {
		multiplier = 1e9
		s = strings.TrimSuffix(s, "B")
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return val * multiplier, nil
}
package data

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"log"
)

// defaultFredAPIKey FRED 官方 API Key（免费注册：https://fred.stlouisfed.org/docs/api/api_key.html）。
// 匿名访问 fredgraph.csv 也可用但限流较低；带 Key 走官方 API 更稳定。
const defaultFredAPIKey = "7b9f46e9f6779100610467af49bc0ac6"

// FredApi fetches economic data from Federal Reserve Economic Data (FRED)
type FredApi struct {
	client    *http.Client
	mu        sync.RWMutex
	cache     map[string]*fredCacheEntry
	baseURL   string
	apiKey    string
}

// FredObservation represents a single FRED data point
type FredObservation struct {
	Date  time.Time
	Value float64
}

type fredCacheEntry struct {
	Data      []FredObservation
	CreatedAt time.Time
}

// NewFredApi creates a new FRED API client
func NewFredApi() *FredApi {
	return &FredApi{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://fred.stlouisfed.org/graph",
		cache:   make(map[string]*fredCacheEntry),
		apiKey:  defaultFredAPIKey,
	}
}

// SetAPIKey sets the FRED API key (optional, for higher rate limits)
func (f *FredApi) SetAPIKey(apiKey string) {
	f.apiKey = apiKey
}

// GetObservations fetches time series data
func (f *FredApi) GetObservations(seriesID string, limit int) ([]FredObservation, error) {
	f.mu.RLock()
	if cached, ok := f.cache[seriesID]; ok && time.Since(cached.CreatedAt) < 24*time.Hour {
		f.mu.RUnlock()
		if limit > 0 && len(cached.Data) >= limit {
			return cached.Data[:limit], nil
		}
		return cached.Data, nil
	}
	f.mu.RUnlock()

	var observations []FredObservation
	var err error
	if f.apiKey != "" {
		// 官方 API：fredgraph.csv 不接受 api_key 参数（带 key 会返回 INTERNAL_ERROR），
		// 带 Key 时走官方 JSON 端点。
		observations, err = f.fetchViaOfficialAPI(seriesID)
	} else {
		observations, err = f.fetchViaGraphCSV(seriesID)
	}
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.cache[seriesID] = &fredCacheEntry{Data: observations, CreatedAt: time.Now()}
	f.mu.Unlock()

	if limit > 0 && len(observations) > limit {
		observations = observations[:limit]
	}

	return observations, nil
}

// fetchViaOfficialAPI 通过 FRED 官方 JSON API 获取序列（升序，旧→新，最新在末尾）。
func (f *FredApi) fetchViaOfficialAPI(seriesID string) ([]FredObservation, error) {
	url := fmt.Sprintf("https://api.stlouisfed.org/fred/series/observations?series_id=%s&api_key=%s&file_type=json&sort_order=asc&observation_start=2020-01-01",
		seriesID, f.apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("fred request create: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fred observations request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fred api returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fred observations read: %w", err)
	}
	var payload struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("fred json parse error: %w", err)
	}

	observations := make([]FredObservation, 0, len(payload.Observations))
	for _, o := range payload.Observations {
		if o.Value == "." || o.Value == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", o.Date)
		if err != nil {
			log.Printf("fred: skip invalid date %s: %v", o.Date, err)
			continue
		}
		value, err := strconv.ParseFloat(o.Value, 64)
		if err != nil {
			log.Printf("fred: skip invalid value %s: %v", o.Value, err)
			continue
		}
		observations = append(observations, FredObservation{Date: date, Value: value})
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("fred: no observations found for %s", seriesID)
	}
	return observations, nil
}

// fetchViaGraphCSV 通过 fredgraph.csv 匿名端点获取序列（无 API Key 时的降级路径）。
func (f *FredApi) fetchViaGraphCSV(seriesID string) ([]FredObservation, error) {
	url := fmt.Sprintf("%s/fredgraph.csv?id=%s&observation_start=2020-01-01", f.baseURL, seriesID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("fred request create: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/csv,application/json")
	req.Header.Set("Referer", "https://fred.stlouisfed.org/")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fred observations request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fred api returned status %d: %s", resp.StatusCode, string(body))
	}

	return f.parseCSVResponse(resp.Body)
}

// GetLatestValue fetches the most recent observation
func (f *FredApi) GetLatestValue(seriesID string) (float64, error) {
	observations, err := f.GetObservations(seriesID, 0)
	if err != nil {
		return 0, err
	}

	if len(observations) == 0 {
		return 0, fmt.Errorf("no observations found for %s", seriesID)
	}

	// FRED CSV returns oldest first, so last element is latest
	return observations[len(observations)-1].Value, nil
}

// parseCSVResponse parses FRED CSV format: DATE,VALUE
func (f *FredApi) parseCSVResponse(body io.Reader) ([]FredObservation, error) {
	reader := csv.NewReader(body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parse error: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("csv has no data rows")
	}

	// Skip header row
	records = records[1:]

	observations := make([]FredObservation, 0, len(records))
	for _, record := range records {
		if len(record) < 2 {
			continue
		}

		dateStr := strings.TrimSpace(record[0])
		valueStr := strings.TrimSpace(record[1])

		if valueStr == "." || valueStr == "" {
			continue
		}

		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("fred: skip invalid date %s: %v", dateStr, err)
			continue
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			log.Printf("fred: skip invalid value %s: %v", valueStr, err)
			continue
		}

		observations = append(observations, FredObservation{
			Date:  date,
			Value: value,
		})
	}

	return observations, nil
}

// GetTIPSRate fetches the latest 10-year TIPS rate
func (f *FredApi) GetTIPSRate() (float64, error) {
	return f.GetLatestValue("DFII10")
}

// GetTIPS5YRate fetches the latest 5-year TIPS rate
func (f *FredApi) GetTIPS5YRate() (float64, error) {
	return f.GetLatestValue("DFII5")
}

// GetTIPS20YRate fetches the latest 20-year TIPS rate
func (f *FredApi) GetTIPS20YRate() (float64, error) {
	return f.GetLatestValue("DFII20")
}

// GetTIPS30YRate fetches the latest 30-year TIPS rate
func (f *FredApi) GetTIPS30YRate() (float64, error) {
	return f.GetLatestValue("DFII30")
}

// GetBreakEvenInflation fetches 5-Year Breakeven Inflation Rate
func (f *FredApi) GetBreakEvenInflation() (float64, error) {
	return f.GetLatestValue("T5YIE")
}

// GetBreakEvenInflation10Y fetches 10-Year Breakeven Inflation Rate
func (f *FredApi) GetBreakEvenInflation10Y() (float64, error) {
	return f.GetLatestValue("T10YIE")
}

// GetBreakEvenInflation30Y fetches 30-Year Breakeven Inflation Rate (monthly)
func (f *FredApi) GetBreakEvenInflation30Y() (float64, error) {
	return f.GetLatestValue("T30YIEM")
}

// GetVIX fetches the latest CBOE Volatility Index (VIX)
func (f *FredApi) GetVIX() (float64, error) {
	return f.GetLatestValue("VIXCLS")
}

// GetDXY fetches the latest EUR/USD exchange rate (DEXUSEU).
// 注意：DEXUSEU 是欧元兑美元汇率（如 1.17），不是 ICE 美元指数（约 100 量级），
// 仅可作汇率参考，不可直接当 DXY 用。
func (f *FredApi) GetDXY() (float64, error) {
	return f.GetLatestValue("DEXUSEU")
}

// GetBroadDollarIndex fetches the latest trade-weighted broad dollar index (DTWEXBGS)。
// 量级与 DXY 相近，可作为 WallstreetCN 美元指数缺失时的回退源。
func (f *FredApi) GetBroadDollarIndex() (float64, error) {
	return f.GetLatestValue("DTWEXBGS")
}

// GetUS2YRate / GetUS5YRate / GetUS7YRate / GetUS10YRate / GetUS30YRate
// 分别获取 2/5/7/10/30 年期美债收益率（DGS2/DGS5/DGS7/DGS10/DGS30，单位 %）。
func (f *FredApi) GetUS2YRate() (float64, error)  { return f.GetLatestValue("DGS2") }
func (f *FredApi) GetUS5YRate() (float64, error)  { return f.GetLatestValue("DGS5") }
func (f *FredApi) GetUS7YRate() (float64, error)  { return f.GetLatestValue("DGS7") }
func (f *FredApi) GetUS10YRate() (float64, error) { return f.GetLatestValue("DGS10") }
func (f *FredApi) GetUS30YRate() (float64, error) { return f.GetLatestValue("DGS30") }

// CalculateRealRate calculates real interest rate: nominal - TIPS
func CalculateRealRate(nominalRate, tipsRate float64) float64 {
	return nominalRate - tipsRate
}
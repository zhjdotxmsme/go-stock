package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"log"
)

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

	url := fmt.Sprintf("%s/fredgraph.csv?id=%s&observation_start=2020-01-01", f.baseURL, seriesID)
	if f.apiKey != "" {
		url = fmt.Sprintf("%s&api_key=%s", url, f.apiKey)
	}
	if limit > 0 {
		url = fmt.Sprintf("%s&limit=%d", url, limit)
	}

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

	observations, err := f.parseCSVResponse(resp.Body)
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

// CalculateRealRate calculates real interest rate: nominal - TIPS
func CalculateRealRate(nominalRate, tipsRate float64) float64 {
	return nominalRate - tipsRate
}
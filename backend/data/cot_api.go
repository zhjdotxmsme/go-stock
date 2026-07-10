package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CotApi fetches Commitment of Traders (COT) data from CFTC.gov
type CotApi struct {
	client  *http.Client
	baseURL string
	cache   *map[string][]CotRecord
}

// CotRecord represents a single COT data point
type CotRecord struct {
	Date              time.Time
	OpenInterest     int64
	DealerLong        int64
	DealerShort       int64
	AssetManagerLong  int64
	AssetManagerShort int64
	LeveragedLong     int64
	LeveragedShort    int64
	OtherLong         int64
	OtherShort        int64
	NonReportableLong int64
	NonReportableShort int64
}

// CotPositionSummary represents position analysis
type CotPositionSummary struct {
	NetPosition        int64     // Net position (long - short)
	ZScore             float64   // Z-score for position extremes
	CrowdedLevel       string    // "none", "moderate", "crowded_top", "crowded_bottom"
	HistoricalPercentile float64  // Current position's percentile
}

// NewCotApi creates a new COT API client
func NewCotApi() *CotApi {
	cache := make(map[string][]CotRecord)
	return &CotApi{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://www.cftc.gov",
		cache:   &cache,
	}
}

// GetCotData fetches COT data for a commodity
// commodityCodes: e.g., "GC" for gold, "SI" for silver, "CL" for crude oil
func (c *CotApi) GetCotData(commodityCode string, limit int) ([]CotRecord, error) {
	// Check cache
	if data, ok := (*c.cache)[commodityCode]; ok {
		if len(data) >= limit {
			return data[:limit], nil
		}
	}

	// Construct CFTC download URL
	// Format: https://www.cftc.gov/dea/futures/deacmesf.htm
	// We'll use the CSV export endpoint if available
	url := fmt.Sprintf("%s/files/dea/history/deacmesf_%s.csv", c.baseURL, strings.ToLower(commodityCode))

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cot data request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cot api returned status %d: %s", resp.StatusCode, string(body))
	}

	records, err := c.parseCotCSV(resp.Body)
	if err != nil {
		return nil, err
	}

	// Cache the data
	(*c.cache)[commodityCode] = records

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

// parseCotCSV parses CFTC CSV format
func (c *CotApi) parseCotCSV(body io.Reader) ([]CotRecord, error) {
	reader := csv.NewReader(body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parse error: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("csv has no data rows")
	}

	// CFTC CSV format:
	// Market and Exchange Names, Report Date as YYYY-MM-DD,
	// Open Interest, Dealer Long, Dealer Short,
	// Asset Manager Long, Asset Manager Short,
	// Leveraged Long, Leveraged Short,
	// Other Long, Other Short,
	// Nonreportable Long, Nonreportable Short

	cotRecords := make([]CotRecord, 0, len(records)-1)

	// Skip header row (first row)
	for _, record := range records[1:] {
		if len(record) < 13 {
			continue
		}

		dateStr := strings.TrimSpace(record[1])
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		openInterest, _ := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64)
		dealerLong, _ := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		dealerShort, _ := strconv.ParseInt(strings.TrimSpace(record[4]), 10, 64)
		assetManagerLong, _ := strconv.ParseInt(strings.TrimSpace(record[5]), 10, 64)
		assetManagerShort, _ := strconv.ParseInt(strings.TrimSpace(record[6]), 10, 64)
		leveragedLong, _ := strconv.ParseInt(strings.TrimSpace(record[7]), 10, 64)
		leveragedShort, _ := strconv.ParseInt(strings.TrimSpace(record[8]), 10, 64)
		otherLong, _ := strconv.ParseInt(strings.TrimSpace(record[9]), 10, 64)
		otherShort, _ := strconv.ParseInt(strings.TrimSpace(record[10]), 10, 64)
		nonReportableLong, _ := strconv.ParseInt(strings.TrimSpace(record[11]), 10, 64)
		nonReportableShort, _ := strconv.ParseInt(strings.TrimSpace(record[12]), 10, 64)

		cotRecords = append(cotRecords, CotRecord{
			Date:                date,
			OpenInterest:        openInterest,
			DealerLong:          dealerLong,
			DealerShort:         dealerShort,
			AssetManagerLong:   assetManagerLong,
			AssetManagerShort:  assetManagerShort,
			LeveragedLong:      leveragedLong,
			LeveragedShort:     leveragedShort,
			OtherLong:           otherLong,
			OtherShort:          otherShort,
			NonReportableLong:  nonReportableLong,
			NonReportableShort: nonReportableShort,
		})
	}

	return cotRecords, nil
}

// GetPositionSummary calculates position metrics for managed money (asset managers)
func (c *CotApi) GetPositionSummary(commodityCode string) (*CotPositionSummary, error) {
	records, err := c.GetCotData(commodityCode, 104) // 2 years of data
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no COT data available for %s", commodityCode)
	}

	// Get latest record
	latest := records[0]

	// Calculate net position for asset managers (managed money)
	netPosition := latest.AssetManagerLong - latest.AssetManagerShort

	// Calculate z-score
	zScore, err := c.calculateZScore(commodityCode, "assetManager", netPosition)
	if err != nil {
		return nil, fmt.Errorf("calculate z-score: %w", err)
	}

	// Calculate percentile
	percentile := c.calculatePercentile(commodityCode, "assetManager", netPosition)

	// Determine crowded level
	crowdedLevel := "none"
	switch {
	case zScore > 2:
		crowdedLevel = "crowded_top" // Overcrowded longs - potential top
	case zScore < -2:
		crowdedLevel = "crowded_bottom" // Overcrowded shorts - potential bottom
	case zScore > 1:
		crowdedLevel = "moderate_top"
	case zScore < -1:
		crowdedLevel = "moderate_bottom"
	}

	return &CotPositionSummary{
		NetPosition:        netPosition,
		ZScore:             zScore,
		CrowdedLevel:       crowdedLevel,
		HistoricalPercentile: percentile,
	}, nil
}

// calculateZScore calculates z-score for a position type
func (c *CotApi) calculateZScore(commodityCode, positionType string, currentValue int64) (float64, error) {
	records, err := c.GetCotData(commodityCode, 260) // 5 years of data
	if err != nil {
		return 0, err
	}

	if len(records) < 2 {
		return 0, fmt.Errorf("insufficient data for z-score calculation")
	}

	// Calculate historical net positions
	var values []float64
	for _, record := range records {
		var netPosition int64
		switch positionType {
		case "assetManager":
			netPosition = record.AssetManagerLong - record.AssetManagerShort
		case "dealer":
			netPosition = record.DealerLong - record.DealerShort
		case "leveraged":
			netPosition = record.LeveragedLong - record.LeveragedShort
		}
		values = append(values, float64(netPosition))
	}

	// Calculate mean and standard deviation
	mean := c.calculateMean(values)
	stdDev := c.calculateStdDev(values, mean)

	if stdDev == 0 {
		return 0, nil
	}

	// Calculate z-score
	zScore := (float64(currentValue) - mean) / stdDev
	return zScore, nil
}

// calculatePercentile calculates the historical percentile of current position
func (c *CotApi) calculatePercentile(commodityCode, positionType string, currentValue int64) float64 {
	records, err := c.GetCotData(commodityCode, 260)
	if err != nil {
		return 0
	}

	if len(records) < 2 {
		return 0
	}

	var values []int64
	for _, record := range records {
		var netPosition int64
		switch positionType {
		case "assetManager":
			netPosition = record.AssetManagerLong - record.AssetManagerShort
		case "dealer":
			netPosition = record.DealerLong - record.DealerShort
		case "leveraged":
			netPosition = record.LeveragedLong - record.LeveragedShort
		}
		values = append(values, netPosition)
	}

	// Sort ascending
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })

	// Find position of current value (how many values are <= currentValue)
	position := 0
	for _, v := range values {
		if v <= currentValue {
			position++
		} else {
			break
		}
	}

	// Calculate percentile
	percentile := float64(position) / float64(len(values))
	return percentile * 100
}

// calculateMean calculates mean of values
func (c *CotApi) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev calculates standard deviation
func (c *CotApi) calculateStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(values)-1)
	return math.Sqrt(variance)
}

// GetLatestCotData fetches the latest COT report for a commodity
func (c *CotApi) GetLatestCotData(commodityCode string) (*CotRecord, error) {
	records, err := c.GetCotData(commodityCode, 1)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no COT data available for %s", commodityCode)
	}

	return &records[0], nil
}
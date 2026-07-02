package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testYahooChartJSON = `{
  "chart": {
    "result": [{
      "meta": {
        "currency": "USD",
        "symbol": "GC=F",
        "regularMarketPrice": 2345.6,
        "previousClose": 2300.0,
        "chartPreviousClose": 2300.0,
        "regularMarketTime": 1719878400
      },
      "timestamp": [1719792000, 1719878400],
      "indicators": {
        "quote": [{
          "open": [2301.0, 2295.0],
          "high": [2310.0, 2350.0],
          "low": [2290.0, 2285.0],
          "close": [2305.0, 2345.6],
          "volume": [1000, 2000]
        }]
      }
    }],
    "error": null
  }
}`

func TestParseYahooChart(t *testing.T) {
	chart, err := parseYahooChart([]byte(testYahooChartJSON))
	require.NoError(t, err)
	require.NotNil(t, chart)
	assert.Equal(t, "GC=F", chart.Chart.Result[0].Meta.Symbol)
	assert.Equal(t, 2345.6, chart.Chart.Result[0].Meta.RegularMarketPrice)
}

func TestYahooBarsFromChart(t *testing.T) {
	chart, err := parseYahooChart([]byte(testYahooChartJSON))
	require.NoError(t, err)

	bars, err := yahooBarsFromChart(chart, "XAUUSD", 0)
	require.NoError(t, err)
	require.Len(t, bars, 2)

	assert.Equal(t, 2301.0, bars[0].Open)
	assert.Equal(t, 2310.0, bars[0].High)
	assert.Equal(t, 2290.0, bars[0].Low)
	assert.Equal(t, 2305.0, bars[0].Close)
	assert.Equal(t, int64(1000), bars[0].Volume)

	assert.Equal(t, 2295.0, bars[1].Open)
	assert.Equal(t, 2350.0, bars[1].High)
	assert.Equal(t, 2345.6, bars[1].Close)
	assert.Equal(t, int64(2000), bars[1].Volume)
}

func TestYahooBarsFromChart_Limit(t *testing.T) {
	chart, err := parseYahooChart([]byte(testYahooChartJSON))
	require.NoError(t, err)

	bars, err := yahooBarsFromChart(chart, "XAUUSD", 1)
	require.NoError(t, err)
	require.Len(t, bars, 1)
	assert.Equal(t, 2345.6, bars[0].Close)
}

func TestYahooRangeForCount(t *testing.T) {
	assert.Equal(t, "5d", yahooRangeForCount(3, "day"))
	assert.Equal(t, "1mo", yahooRangeForCount(20, "day"))
	assert.Equal(t, "1y", yahooRangeForCount(200, "day"))
	assert.Equal(t, "1y", yahooRangeForCount(50, "week"))
	assert.Equal(t, "1y", yahooRangeForCount(10, "month"))
}

func TestParseYahooChart_Error(t *testing.T) {
	errorJSON := `{"chart":{"result":[],"error":{"code":"Not Found","description":"No data found"}}}`
	chart, err := parseYahooChart([]byte(errorJSON))
	assert.Error(t, err)
	assert.Nil(t, chart)
}

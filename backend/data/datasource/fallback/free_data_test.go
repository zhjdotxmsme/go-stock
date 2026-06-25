package fallback

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToTencentCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"600519", "sh600519"},
		{"000001", "sz000001"},
		{"688981", "sh688981"},
		{"300750", "sz300750"},
		{"SH600519", "sh600519"},
		{"sz000001", "sz000001"},
		{"sh600519", "sh600519"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, toTencentCode(tt.input))
		})
	}
}

func TestParseTencentQuoteResponse(t *testing.T) {
	resp := "v_sh600519=\"1~贵州茅台~600519~1788.00~1785.00~1790.00~1770.00~1000~2000000~0.15~0.05~\";"
	quote, err := parseTencentQuoteResponse(resp, "600519")
	assert.NoError(t, err)
	assert.Equal(t, "600519", quote.Code)
	assert.InDelta(t, 1788.00, quote.Price, 0.01)
	assert.WithinDuration(t, time.Now(), quote.Time, time.Minute)
}

func TestParseTencentQuoteResponseInvalid(t *testing.T) {
	_, err := parseTencentQuoteResponse("no tilde", "600519")
	assert.Error(t, err)

	_, err = parseTencentQuoteResponse("a~b~c", "600519")
	assert.Error(t, err)
}

func TestParseTencentKLineResponse(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"data": {
			"sh600519": {
				"qfqday": [
					["2024-01-01", "1700.00", "1710.00", "1720.00", "1690.00", "10000"],
					["2024-01-02", "1710.00", "1720.00", "1730.00", "1700.00", "12000"]
				]
			}
		}
	}`)
	kline, err := parseTencentKLineResponse(body, "600519", "101", "sh600519")
	assert.NoError(t, err)
	assert.Equal(t, "600519", kline.Code)
	assert.Equal(t, "101", kline.Period)
	assert.Len(t, kline.Bars, 2)

	bar := kline.Bars[0]
	assert.Equal(t, "2024-01-01", bar.Time.Format("2006-01-02"))
	assert.InDelta(t, 1700.00, bar.Open, 0.01)
	assert.InDelta(t, 1710.00, bar.Close, 0.01)
	assert.InDelta(t, 1720.00, bar.High, 0.01)
	assert.InDelta(t, 1690.00, bar.Low, 0.01)
	assert.Equal(t, int64(10000), bar.Volume)
}

func TestParseTencentKLineResponseFallbackDay(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"data": {
			"sz000001": {
				"day": [
					["2024-01-01", "10.00", "10.50", "10.80", "9.90", "50000"]
				]
			}
		}
	}`)
	kline, err := parseTencentKLineResponse(body, "000001", "101", "sz000001")
	assert.NoError(t, err)
	assert.Len(t, kline.Bars, 1)
}

func TestParseTencentKLineResponseErrorCode(t *testing.T) {
	body := []byte(`{"code": -1, "msg": "error"}`)
	_, err := parseTencentKLineResponse(body, "600519", "101", "sh600519")
	assert.Error(t, err)
}

func TestParseTencentKLineResponseEmptyBars(t *testing.T) {
	body := []byte(`{"code": 0, "data": {"sh600519": {"qfqday": []}}}`)
	_, err := parseTencentKLineResponse(body, "600519", "101", "sh600519")
	assert.Error(t, err)
}

func TestParseKLineTime(t *testing.T) {
	assert.Equal(t, "2024-01-15", parseKLineTime("2024-01-15").Format("2006-01-02"))
	assert.Equal(t, "2024-01-15", parseKLineTime("2024-01-15 10:30:00").Format("2006-01-02"))
	assert.True(t, parseKLineTime("invalid").IsZero())
}

func TestToFloat64(t *testing.T) {
	assert.InDelta(t, 1.5, toFloat64(1.5), 0.0001)
	assert.InDelta(t, 1.0, toFloat64(1), 0.0001)
	assert.InDelta(t, 1.5, toFloat64("1.5"), 0.0001)
	assert.InDelta(t, 0.0, toFloat64(nil), 0.0001)
	assert.InDelta(t, 0.0, toFloat64(""), 0.0001)
}

package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data/datasource"
	"strconv"
	"strings"
	"time"
)

// YahooFinanceApi 提供 Yahoo Finance 行情与 K 线数据，作为商品现货 fallback。
// Yahoo 商品代码：黄金 GC=F、白银 SI=F、原油 CL=F。
type YahooFinanceApi struct{}

var yahooCommoditySymbols = map[string]string{
	"XAUUSD": "GC=F",
	"XAGUSD": "SI=F",
	"USCL":   "CL=F",
	// 国内期货主力合约映射到国际期货，作为 Sina 失效时的 fallback。
	"AU": "GC=F",
	"AG": "SI=F",
	"SC": "CL=F",
}

func (y *YahooFinanceApi) resolveSymbol(code string) (string, error) {
	if sym, ok := yahooCommoditySymbols[code]; ok {
		return sym, nil
	}
	return "", fmt.Errorf("Yahoo Finance 不支持品种: %s", code)
}

// GetQuote 获取实时行情，使用 Yahoo Finance chart API (range=1d)。
func (y *YahooFinanceApi) GetQuote(code string) (*datasource.QuoteData, error) {
	symbol, err := y.resolveSymbol(code)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)
	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("yahoo quote request: %w", err)
	}

	chart, err := parseYahooChart(resp.Body())
	if err != nil {
		return nil, fmt.Errorf("yahoo quote parse: %w", err)
	}

	result := chart.Chart.Result[0]
	meta := result.Meta
	price := meta.RegularMarketPrice
	if price == 0 && len(result.Indicators.Quote) > 0 {
		q := result.Indicators.Quote[0]
		for i := len(q.Close) - 1; i >= 0; i-- {
			if q.Close[i] != nil {
				price = *q.Close[i]
				break
			}
		}
	}
	if price == 0 {
		return nil, fmt.Errorf("yahoo quote: no price for %s", symbol)
	}

	prevClose := meta.PreviousClose
	if prevClose == 0 && meta.ChartPreviousClose != 0 {
		prevClose = meta.ChartPreviousClose
	}
	change := price - prevClose
	changePct := 0.0
	if prevClose != 0 {
		changePct = change / prevClose * 100
	}

	name := code
	switch code {
	case "XAUUSD", "AU":
		name = "黄金"
	case "XAGUSD", "AG":
		name = "白银"
	case "USCL", "SC":
		name = "原油"
	}

	return &datasource.QuoteData{
		Code:      code,
		Name:      name,
		Price:     price,
		Change:    change,
		ChangePct: changePct,
		Time:      time.Now(),
	}, nil
}

// GetKLine 获取历史 K 线。
// period 支持 day/week/month；count 用于估算 range。
func (y *YahooFinanceApi) GetKLine(code, period string, count int) ([]datasource.KLineBar, error) {
	symbol, err := y.resolveSymbol(code)
	if err != nil {
		return nil, err
	}

	interval := "1d"
	switch period {
	case "week":
		interval = "1wk"
	case "month":
		interval = "1mo"
	}

	rangeParam := yahooRangeForCount(count, period)
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=%s&range=%s", symbol, interval, rangeParam)

	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("yahoo kline request: %w", err)
	}

	chart, err := parseYahooChart(resp.Body())
	if err != nil {
		return nil, fmt.Errorf("yahoo kline parse: %w", err)
	}

	return yahooBarsFromChart(chart, code, count)
}

// yahooRangeForCount 根据期望条数和周期选择 Yahoo range 参数。
func yahooRangeForCount(count int, period string) string {
	if count <= 0 {
		count = 120
	}
	switch period {
	case "week":
		if count <= 52 {
			return "1y"
		}
		if count <= 104 {
			return "2y"
		}
		return "5y"
	case "month":
		if count <= 12 {
			return "1y"
		}
		if count <= 36 {
			return "3y"
		}
		return "10y"
	default:
		if count <= 5 {
			return "5d"
		}
		if count <= 30 {
			return "1mo"
		}
		if count <= 90 {
			return "3mo"
		}
		if count <= 180 {
			return "6mo"
		}
		if count <= 365 {
			return "1y"
		}
		if count <= 730 {
			return "2y"
		}
		return "5y"
	}
}

// --- Yahoo chart response structs ---

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func parseYahooChart(body []byte) (*yahooChartResponse, error) {
	var resp yahooChartResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if resp.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo api error: %s - %s", resp.Chart.Error.Code, resp.Chart.Error.Description)
	}
	if len(resp.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty chart result")
	}
	return &resp, nil
}

func yahooBarsFromChart(chart *yahooChartResponse, code string, count int) ([]datasource.KLineBar, error) {
	result := chart.Chart.Result[0]
	if len(result.Timestamp) == 0 {
		return nil, fmt.Errorf("yahoo kline: no timestamps")
	}
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo kline: no quote data")
	}

	q := result.Indicators.Quote[0]
	n := len(result.Timestamp)
	if len(q.Open) < n || len(q.High) < n || len(q.Low) < n || len(q.Close) < n {
		return nil, fmt.Errorf("yahoo kline: mismatched quote arrays")
	}

	bars := make([]datasource.KLineBar, 0, n)
	for i := 0; i < n; i++ {
		if q.Open[i] == nil || q.High[i] == nil || q.Low[i] == nil || q.Close[i] == nil {
			continue
		}
		var volume int64
		if i < len(q.Volume) && q.Volume[i] != nil {
			volume = *q.Volume[i]
		}
		bars = append(bars, datasource.KLineBar{
			Time:   time.Unix(result.Timestamp[i], 0),
			Open:   *q.Open[i],
			High:   *q.High[i],
			Low:    *q.Low[i],
			Close:  *q.Close[i],
			Volume: volume,
		})
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("yahoo kline: no valid bars")
	}

	// Yahoo returns oldest first, keep order; take most recent count if needed.
	if count > 0 && len(bars) > count {
		bars = bars[len(bars)-count:]
	}
	return bars, nil
}

// ParseYahooFloat 安全解析 interface{} 到 float64，用于测试中复用。
func ParseYahooFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	}
	return 0
}

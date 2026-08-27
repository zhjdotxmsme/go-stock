package data

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/data/datasource"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// === Yahoo Symbol Resolver ===
//
// Converts go-stock internal codes to Yahoo Finance format.
//
// Internal format → Yahoo format:
//   usAAPL        → AAPL
//   hk00700       → 0700.HK
//   sh600519      → 600519.SS
//   sz000001      → 000001.SZ
//   bj430047      → 430047.BJ
//   us_SPX        → ^GSPC
//   jp_N225       → ^N225
//   XAUUSD        → GC=F
//   TLT           → TLT

// YahooSymbolResolver converts go-stock codes to Yahoo Finance symbols.
type YahooSymbolResolver struct{}

// NewYahooSymbolResolver creates a new symbol resolver.
func NewYahooSymbolResolver() *YahooSymbolResolver {
	return &YahooSymbolResolver{}
}

// Resolve converts an internal stock code to Yahoo Finance symbol format.
func (r *YahooSymbolResolver) Resolve(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("empty code")
	}

	lc := strings.ToLower(code)

	// 1. 美股 (usXXX / gb_XXX)
	if strings.HasPrefix(lc, "us") || strings.HasPrefix(lc, "gb_") {
		symbol := strings.TrimPrefix(strings.TrimPrefix(lc, "us"), "gb_")
		symbol = strings.TrimSpace(symbol)
		symbol = strings.ToUpper(symbol)
		symbol = strings.TrimSuffix(symbol, ".US")
		if symbol == "" {
			return "", fmt.Errorf("invalid US code: %s", code)
		}
		return symbol, nil
	}

	// 2. 港股 (hkXXXX)
	if strings.HasPrefix(lc, "hk") {
		symbol := strings.TrimPrefix(lc, "hk")
		symbol = strings.TrimSpace(symbol)
		symbol = fmt.Sprintf("%05s", symbol)
		return symbol + ".HK", nil
	}

	// 3. A股上交所 (shXXXXXX)
	if strings.HasPrefix(lc, "sh") {
		symbol := strings.TrimPrefix(lc, "sh")
		return strings.ToUpper(symbol) + ".SS", nil
	}

	// 4. A股深交所 (szXXXXXX)
	if strings.HasPrefix(lc, "sz") {
		symbol := strings.TrimPrefix(lc, "sz")
		return strings.ToUpper(symbol) + ".SZ", nil
	}

	// 5. A股北交所 (bjXXXXXX)
	if strings.HasPrefix(lc, "bj") {
		symbol := strings.TrimPrefix(lc, "bj")
		return strings.ToUpper(symbol) + ".BJ", nil
	}

	// 6. 全球指数
	if idxSym, ok := globalIndexMap[lc]; ok {
		return idxSym, nil
	}

	// 7. 商品期货 / ETF
	if sym, ok := yahooCommoditySymbols[lc]; ok {
		return sym, nil
	}

	// 8. 如果已经是大写的标准格式（AAPL, GC=F, ^GSPC）
	if isValidYahooSymbol(code) {
		return strings.ToUpper(code), nil
	}

	return "", fmt.Errorf("unsupported code format: %s", code)
}

// ResolveBatch batch resolves multiple codes.
func (r *YahooSymbolResolver) ResolveBatch(codes []string) (map[string]string, []string) {
	result := make(map[string]string, len(codes))
	var failed []string
	for _, code := range codes {
		if sym, err := r.Resolve(code); err == nil {
			result[code] = sym
		} else {
			failed = append(failed, code)
		}
	}
	return result, failed
}

// Reverse maps a Yahoo symbol back to go-stock internal format (best effort).
func (r *YahooSymbolResolver) Reverse(yahooSymbol string) string {
	yahooSymbol = strings.ToUpper(strings.TrimSpace(yahooSymbol))

	for internal, yahoo := range globalIndexMap {
		if yahoo == yahooSymbol {
			return internal
		}
	}

	if strings.HasSuffix(yahooSymbol, ".HK") {
		return "hk" + strings.TrimSuffix(yahooSymbol, ".HK")
	}
	if strings.HasSuffix(yahooSymbol, ".SS") {
		return "sh" + strings.TrimSuffix(yahooSymbol, ".SS")
	}
	if strings.HasSuffix(yahooSymbol, ".SZ") {
		return "sz" + strings.TrimSuffix(yahooSymbol, ".SZ")
	}
	if strings.HasSuffix(yahooSymbol, ".BJ") {
		return "bj" + strings.TrimSuffix(yahooSymbol, ".BJ")
	}

	if matched, _ := regexp.MatchString(`^[A-Z]+$`, yahooSymbol); matched {
		return "us" + yahooSymbol
	}

	for internal, yahoo := range yahooCommoditySymbols {
		if yahoo == yahooSymbol {
			return internal
		}
	}

	return ""
}

// === Global data maps ===

// globalIndexMap: go-stock internal code → Yahoo Finance symbol
var globalIndexMap = map[string]string{
	// Americas
	"us_spx":     "^GSPC",    // S&P 500
	"us_ndx":     "^IXIC",    // NASDAQ
	"us_dji":     "^DJI",     // Dow Jones
	"us_rut":     "^RUT",     // Russell 2000
	"us_vix":     "^VIX",     // VIX
	"us_dxy":     "DX-Y.NYB", // US Dollar Index
	// Asia
	"hk_hsi":     "^HSI",     // Hang Seng
	"hk_hstech":  "^HSTECH",  // Hang Seng Tech
	"jp_n225":    "^N225",    // Nikkei 225
	"kr_kospi":   "^KS11",    // KOSPI
	"in_sensex":  "^BSESN",   // BSE Sensex
	"sg_sti":     "^STI",     // Straits Times
	"tw_twi":     "^TWII",    // Taiwan Weighted
	// Europe
	"uk_ftse":    "^FTSE",    // FTSE 100
	"de_dax":     "^GDAXI",   // DAX
	"fr_cac":     "^FCHI",    // CAC 40
	"eu_stoxx50": "^STOXX50E", // Euro Stoxx 50
	"it_mib":     "^FTSEMIB.MI", // FTSE MIB
	"es_ibex":    "^IBEX",    // IBEX 35
	// Others
	"au_asx":     "^AXJO",    // ASX 200
	"br_bovespa": "^BVSP",    // Bovespa
}

// yahooCommoditySymbols: go-stock code → Yahoo symbol
var yahooCommoditySymbols = map[string]string{
	// Precious metals
	"xauusd": "GC=F",
	"xagusd": "SI=F",
	"xau":    "GC=F",
	"xag":    "SI=F",
	"gc":     "GC=F",
	"si":     "SI=F",
	// Energy
	"uscl":   "CL=F",
	"usco":   "BZ=F",
	"cl":     "CL=F",
	"bz":     "BZ=F",
	// Industrial metals
	"cu":     "HG=F",
	"hg":     "HG=F",
	// Agriculture
	"zc":     "ZC=F",
	"zw":     "ZW=F",
	"zs":     "ZS=F",
	// Domestic futures mapping
	"au":     "GC=F",
	"ag":     "SI=F",
	"sc":     "CL=F",
	// ETFs
	"tlt":    "TLT",
	"tip":    "TIP",
	"gld":    "GLD",
	"slv":    "SLV",
	"spy":    "SPY",
	"qqq":    "QQQ",
	// Fund
	"161226": "161226.SZ",
}

func isValidYahooSymbol(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	if matched, _ := regexp.MatchString(`^[A-Z][A-Z0-9]*$`, s); matched {
		return true
	}
	if matched, _ := regexp.MatchString(`^[A-Z0-9]+\.[A-Z]+$`, s); matched {
		return true
	}
	if strings.HasPrefix(s, "^") {
		return true
	}
	if strings.HasSuffix(s, "=F") {
		return true
	}
	return false
}

// === Fundamental data via quoteSummary ===

// YahooFundamentalApi provides fundamental data via Yahoo Finance.
type YahooFundamentalApi struct {
	resolver *YahooSymbolResolver
}

// NewYahooFundamentalApi creates a new Yahoo Finance fundamental API client.
func NewYahooFundamentalApi() *YahooFundamentalApi {
	return &YahooFundamentalApi{
		resolver: NewYahooSymbolResolver(),
	}
}

func (y *YahooFundamentalApi) Name() string    { return "yahoo_fundamental" }
func (y *YahooFundamentalApi) Priority() int   { return 25 }
func (y *YahooFundamentalApi) Available(ctx context.Context) bool { return true }

func (y *YahooFundamentalApi) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	symbol, err := y.resolver.Resolve(code)
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=summaryDetail,defaultKeyStatistics,financialData",
		url.QueryEscape(symbol))

	body, err := yahooFetch(urlStr)
	if err != nil {
		return nil, fmt.Errorf("yahoo fetch: %w", err)
	}

	var result struct {
		QuoteSummary struct {
			Result []struct {
				SummaryDetail        *yahooSummaryDetail        `json:"summaryDetail"`
				DefaultKeyStatistics *yahooDefaultKeyStatistics `json:"defaultKeyStatistics"`
				FinancialData        *yahooFinancialData        `json:"financialData"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"quoteSummary"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("yahoo parse: %w", err)
	}
	if result.QuoteSummary.Error != nil {
		return nil, fmt.Errorf("yahoo api error: %s", result.QuoteSummary.Error.Description)
	}
	if len(result.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("yahoo: no data for %s", code)
	}

	r := result.QuoteSummary.Result[0]
	fd := &datasource.FundamentalData{}

	if r.SummaryDetail != nil {
		fd.PE = parseYahooNumber(r.SummaryDetail.TrailingPE)
		fd.PB = parseYahooNumber(r.SummaryDetail.PriceToBook)
	}
	if r.DefaultKeyStatistics != nil {
		fd.ROE = parseYahooNumber(r.DefaultKeyStatistics.ReturnOnEquity)
	}
	if r.FinancialData != nil {
		fd.Revenue = parseYahooNumber(r.FinancialData.TotalRevenue)
		fd.NetProfit = parseYahooNumber(r.FinancialData.NetIncomeToCommon)
		fd.DebtRatio = parseYahooNumber(r.FinancialData.DebtToEquity)
	}

	return fd, nil
}

// --- Yahoo response structs for fundamental data ---

type yahooNumber struct {
	Raw float64 `json:"raw"`
	Fmt string  `json:"fmt"`
}

func parseYahooNumber(n *yahooNumber) float64 {
	if n == nil {
		return 0
	}
	return n.Raw
}

type yahooSummaryDetail struct {
	TrailingPE    *yahooNumber `json:"trailingPE"`
	PriceToBook   *yahooNumber `json:"priceToBook"`
	MarketCap     *yahooNumber `json:"marketCap"`
	DividendYield *yahooNumber `json:"dividendYield"`
}

type yahooDefaultKeyStatistics struct {
	ReturnOnEquity    *yahooNumber `json:"returnOnEquity"`
	EnterpriseValue   *yahooNumber `json:"enterpriseValue"`
	FloatShares       *yahooNumber `json:"floatShares"`
	SharesOutstanding *yahooNumber `json:"sharesOutstanding"`
}

type yahooFinancialData struct {
	TotalRevenue      *yahooNumber `json:"totalRevenue"`
	NetIncomeToCommon *yahooNumber `json:"netIncomeToCommon"`
	DebtToEquity      *yahooNumber `json:"debtToEquity"`
	RevenueGrowth     *yahooNumber `json:"revenueGrowth"`
	EarningsGrowth    *yahooNumber `json:"earningsGrowth"`
	CurrentRatio      *yahooNumber `json:"currentRatio"`
	QuickRatio        *yahooNumber `json:"quickRatio"`
}

// === Dividend History ===

// YahooDividend represents a single dividend payment.
type YahooDividend struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
}

// GetDividendHistory fetches dividend history for a stock.
func (y *YahooFinanceApi) GetDividendHistory(code string, years int) ([]YahooDividend, error) {
	symbol, err := NewYahooSymbolResolver().Resolve(code)
	if err != nil {
		return nil, err
	}

	rangeParam := fmt.Sprintf("%dy", years)
	urlStr := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%s&events=div",
		url.QueryEscape(symbol), rangeParam)

	body, err := yahooFetch(urlStr)
	if err != nil {
		return nil, fmt.Errorf("yahoo fetch: %w", err)
	}

	var result struct {
		Chart struct {
			Result []struct {
				Events *struct {
					Dividends map[string]struct {
						Amount float64 `json:"amount"`
						Date   int64   `json:"date"`
					} `json:"dividends"`
				} `json:"events"`
			} `json:"result"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Chart.Result) == 0 || result.Chart.Result[0].Events == nil {
		return []YahooDividend{}, nil
	}

	var dividends []YahooDividend
	for _, div := range result.Chart.Result[0].Events.Dividends {
		dividends = append(dividends, YahooDividend{
			Date:   time.Unix(div.Date, 0),
			Amount: div.Amount,
		})
	}

	for i, j := 0, len(dividends)-1; i < j; i, j = i+1, j-1 {
		dividends[i], dividends[j] = dividends[j], dividends[i]
	}

	return dividends, nil
}

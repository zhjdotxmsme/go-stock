package backtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// ----- ParseStockCode / NormalizeStockCode -----

func TestParseStockCode(t *testing.T) {
	cases := []struct {
		input        string
		wantDigits   string
		wantExchange string
		wantOK       bool
	}{
		{"600519.SH", "600519", "sh", true},
		{"000001.SZ", "000001", "sz", true},
		{"430047.BJ", "430047", "bj", true},
		{"sh600519", "600519", "sh", true},
		{"SH600519", "600519", "sh", true},
		{"sz000001", "000001", "sz", true},
		{"bj430047", "430047", "bj", true},
		{"sh.600519", "600519", "sh", true},
		{"600519", "600519", "sh", true},   // 6 开头推断上交所
		{"000001", "000001", "sz", true},   // 0 开头推断深交所
		{"300750", "300750", "sz", true},   // 创业板推断深交所
		{"688981", "688981", "sh", true},   // 科创板推断上交所
		{"830799", "830799", "bj", true},   // 8 开头推断北交所
		{"920001", "920001", "bj", true},   // 920 号段北交所
		{"510300", "510300", "sh", true},   // ETF 推断上交所
		{"sh510300", "510300", "sh", true}, // 指数/ETF 前缀格式
		{"sh000300", "000300", "sh", true}, // 指数显式前缀优先于数字推断
		{" 600519.SH ", "600519", "sh", true},
		// 无法识别的输入
		{"mock_win", "", "", false},
		{"test_tplus1", "", "", false},
		{"", "", "", false},
		{"60051", "", "", false},
		{"xx600519", "", "", false},
	}
	for _, c := range cases {
		digits, exchange, ok := ParseStockCode(c.input)
		assert.Equal(t, c.wantOK, ok, "input=%q", c.input)
		if c.wantOK {
			assert.Equal(t, c.wantDigits, digits, "input=%q", c.input)
			assert.Equal(t, c.wantExchange, exchange, "input=%q", c.input)
		}
	}
}

func TestNormalizeStockCode(t *testing.T) {
	cases := map[string]string{
		"600519.SH": "sh600519",
		"SH600519":  "sh600519",
		"sh600519":  "sh600519",
		"sh.600519": "sh600519",
		"600519":    "sh600519",
		"000001.SZ": "sz000001",
		"SZ000001":  "sz000001",
		"bj430047":  "bj430047",
		"430047":    "bj430047",
		"920001":    "bj920001",
		"sh510300":  "sh510300", // 指数/ETF 原样规范
		"sh000001":  "sh000001", // 显式前缀的指数不被数字推断误伤
		"mock_win":  "mock_win", // 无法识别原样返回
	}
	for input, want := range cases {
		assert.Equal(t, want, NormalizeStockCode(input), "input=%q", input)
	}
}

func TestStockCodeCandidates(t *testing.T) {
	assert.Equal(t,
		[]string{"600519.SH", "sh600519", "600519"},
		StockCodeCandidates("600519.SH"))
	assert.Equal(t,
		[]string{"600519", "sh600519", "600519.SH"},
		StockCodeCandidates("600519"))
	assert.Equal(t,
		[]string{"sh600519", "600519.SH", "600519"},
		StockCodeCandidates("sh600519"))
	// 无法识别的输入只有原始格式一个候选
	assert.Equal(t, []string{"mock_win"}, StockCodeCandidates("mock_win"))
}

// ----- limitUpDown 板块判断 -----

func TestLimitUpDown_Boards(t *testing.T) {
	engine := NewEngine()
	cases := []struct {
		name                string
		in                  Input
		wantUp, wantDown    float64
	}{
		// 科创板 20%，三种输入格式
		{"sci-ts", Input{StockCode: "688981.SH"}, 1.199, 0.801},
		{"sci-prefix", Input{StockCode: "sh688981"}, 1.199, 0.801},
		{"sci-bare", Input{StockCode: "688981"}, 1.199, 0.801},
		// 创业板 20%
		{"gem-300-ts", Input{StockCode: "300750.SZ"}, 1.199, 0.801},
		{"gem-301-prefix", Input{StockCode: "sz301269"}, 1.199, 0.801},
		{"gem-bare", Input{StockCode: "301269"}, 1.199, 0.801},
		// 北交所 30%
		{"bj-43", Input{StockCode: "bj430047"}, 1.299, 0.701},
		{"bj-43-bare", Input{StockCode: "430047"}, 1.299, 0.701},
		{"bj-83", Input{StockCode: "830799.BJ"}, 1.299, 0.701},
		{"bj-87-prefix", Input{StockCode: "bj872808"}, 1.299, 0.701},
		{"bj-920", Input{StockCode: "920001"}, 1.299, 0.701},
		// 主板 10%
		{"main-sh-ts", Input{StockCode: "600519.SH"}, 1.099, 0.901},
		{"main-sz-prefix", Input{StockCode: "sz000001"}, 1.099, 0.901},
		{"main-bare", Input{StockCode: "601398"}, 1.099, 0.901},
		// 指数/ETF 不落入个股板块，按主板处理
		{"index-510300", Input{StockCode: "sh510300"}, 1.099, 0.901},
		{"index-000300", Input{StockCode: "sh000300"}, 1.099, 0.901},
		// 无法识别的代码按主板处理
		{"mock", Input{StockCode: "mock_win"}, 1.099, 0.901},
		// ST 5%，任意格式都优先走 ST 分支
		{"st-main", Input{StockCode: "600519.SH", IsST: true}, 1.049, 0.951},
		{"st-sci", Input{StockCode: "sh688981", IsST: true}, 1.049, 0.951},
	}
	for _, c := range cases {
		up, down := engine.limitUpDown(c.in)
		assert.InDelta(t, c.wantUp, up, 1e-9, c.name)
		assert.InDelta(t, c.wantDown, down, 1e-9, c.name)
	}
}

// ----- 缓存多格式命中 -----

// seedCacheBars 向 kline_bars 写入指定存储格式的日线数据。
func seedCacheBars(t *testing.T, storedCode string, bars []models.KLineBar) {
	t.Helper()
	for i := range bars {
		bars[i].StockCode = storedCode
		bars[i].Period = "day"
	}
	require.NoError(t, datasource.NewKLineStore().UpsertKLines(context.Background(), bars))
}

func TestEngineRun_CacheHitAcrossFormats(t *testing.T) {
	restore := setupAEngineTestDB(t)
	defer restore()

	// 种子脚本写入的裸码格式存量数据
	seedCacheBars(t, "600519", []models.KLineBar{
		{TradeDate: "2024-01-02", Open: 100, High: 102, Low: 99, Close: 101, PrevClose: 100},
		{TradeDate: "2024-01-03", Open: 101, High: 105, Low: 100, Close: 104},
		{TradeDate: "2024-01-04", Open: 104, High: 108, Low: 103, Close: 107},
	})

	engine := NewEngine()
	// 不注册 mock：缓存 miss 会走 fallback 并报错，因此 Run 成功即证明缓存命中
	for _, inputCode := range []string{"600519.SH", "sh600519", "600519"} {
		result, err := engine.Run(context.Background(), Input{
			StockCode:   inputCode,
			SignalDate:  "2024-01-02",
			EntryPrice:  101,
			HoldingDays: 3,
			Adjusted:    false,
		})
		require.NoError(t, err, "input=%q should hit cache stored as bare code", inputCode)
		assert.Equal(t, "2024-01-04", result.ExitDate, "input=%q", inputCode)
		assert.Equal(t, 107.0, result.ExitPrice, "input=%q", inputCode)
	}

	// 前缀格式存量数据，裸码/ts_code 输入也应命中
	seedCacheBars(t, "sz000001", []models.KLineBar{
		{TradeDate: "2024-01-02", Open: 10, High: 10.2, Low: 9.9, Close: 10.1, PrevClose: 10},
		{TradeDate: "2024-01-03", Open: 10.1, High: 10.5, Low: 10, Close: 10.4},
	})
	for _, inputCode := range []string{"000001", "000001.SZ"} {
		result, err := engine.Run(context.Background(), Input{
			StockCode:   inputCode,
			SignalDate:  "2024-01-02",
			EntryPrice:  10.1,
			HoldingDays: 3,
			Adjusted:    false,
		})
		require.NoError(t, err, "input=%q should hit cache stored as prefix code", inputCode)
		assert.Equal(t, 10.4, result.ExitPrice, "input=%q", inputCode)
	}
}

func TestEngineRun_PrevCloseBackfillAcrossFormats(t *testing.T) {
	restore := setupAEngineTestDB(t)
	defer restore()

	// 存量为 sh 前缀格式；信号日 PrevClose=0（旧数据），需要从缓存回填前一日收盘价。
	// 前收 100 → 收盘 110 触发主板 10% 涨停，若回填失败则不会拒绝买入。
	seedCacheBars(t, "sh600002", []models.KLineBar{
		{TradeDate: "2024-01-01", Open: 99, High: 101, Low: 98, Close: 100},
		{TradeDate: "2024-01-02", Open: 100, High: 110, Low: 100, Close: 110, PrevClose: 0},
		{TradeDate: "2024-01-03", Open: 110, High: 112, Low: 108, Close: 111},
	})

	engine := NewEngine()
	_, err := engine.Run(context.Background(), Input{
		StockCode:   "600002", // 裸码输入命中前缀格式缓存
		SignalDate:  "2024-01-02",
		EntryPrice:  110,
		HoldingDays: 3,
		Adjusted:    false,
	})
	assert.ErrorContains(t, err, "price limit")
}

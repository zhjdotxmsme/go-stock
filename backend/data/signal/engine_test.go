package signal

import (
	"testing"
	"time"

	"go-stock/backend/data/datasource"
)

// mkBar 构造一根K线
func mkBar(day int, o, h, l, c float64, v int64) datasource.KLineBar {
	return datasource.KLineBar{
		Time:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, day),
		Open:   o, High: h, Low: l, Close: c, Volume: v,
	}
}

// flatBars 构造 n 根平盘K线（收平 100，量 1000）
func flatBars(startDay, n int) []datasource.KLineBar {
	bars := make([]datasource.KLineBar, 0, n)
	for i := 0; i < n; i++ {
		bars = append(bars, mkBar(startDay+i, 100, 100.5, 99.5, 100, 1000))
	}
	return bars
}

func keysOf(matches []SignalMatch) map[string]bool {
	m := make(map[string]bool, len(matches))
	for _, mm := range matches {
		m[mm.Key] = true
	}
	return m
}

func evalKeys(e *Engine, bars []datasource.KLineBar) map[string]bool {
	return keysOf(e.EvaluateLatest("sz000001", bars))
}

// 趋势序列：从 start 起每天涨跌 step，量 1000
func trendBars(startDay int, start, step float64, n int) []datasource.KLineBar {
	bars := make([]datasource.KLineBar, 0, n)
	c := start
	for i := 0; i < n; i++ {
		prev := c
		c = start + step*float64(i+1)
		o := prev
		hi, lo := c, o
		if hi < lo {
			hi, lo = lo, hi
		}
		bars = append(bars, mkBar(startDay+i, o, hi+0.2, lo-0.2, c, 1000))
	}
	return bars
}

func TestMinBarsGuard(t *testing.T) {
	e := &Engine{}
	if got := e.EvaluateLatest("sz000001", flatBars(0, 10)); got != nil {
		t.Errorf("bars < MinBars 应返回空, got %v", got)
	}
}

func TestTrendAndCrossSignals(t *testing.T) {
	e := &Engine{}

	// 单边上涨 40 天：应命中均线多头/连阳/放量外的趋势信号
	up := trendBars(0, 100, 1.0, 40)
	got := evalKeys(e, up)
	if !got["LONG_AVG_ARRAY"] {
		t.Error("单边上涨应命中均线多头排列")
	}
	if !got["UPPER_8DAYS"] || !got["UPPER_9DAYS"] {
		t.Error("40 连涨应命中八连阳/九连阳")
	}
	if got["SHORT_AVG_ARRAY"] || got["DOWN_7DAYS"] {
		t.Error("单边上涨不应命中空头信号")
	}

	// 单边下跌 40 天
	down := trendBars(0, 140, -1.0, 40)
	got = evalKeys(e, down)
	if !got["SHORT_AVG_ARRAY"] {
		t.Error("单边下跌应命中均线空头排列")
	}
	if !got["DOWN_7DAYS"] {
		t.Error("40 连跌应命中七连阴")
	}
	if got["LONG_AVG_ARRAY"] {
		t.Error("单边下跌不应命中多头排列")
	}

	// V 形（先跌后涨）：应出现 MACD 与 KDJ 金叉（扫描时点）
	v := append(trendBars(0, 120, -0.8, 35), trendBars(35, 92, 1.5, 10)...)
	foundMACD, foundKDJ := false, false
	for i := MinBars; i < len(v); i++ {
		ks := keysOf(e.Evaluate("sz000001", v, i))
		foundMACD = foundMACD || ks["MACD_GOLDEN_FORK"]
		foundKDJ = foundKDJ || ks["KDJ_GOLDEN_FORK"]
	}
	if !foundMACD {
		t.Error("V形反转中应出现 MACD金叉")
	}
	if !foundKDJ {
		t.Error("V形反转中应出现 KDJ金叉")
	}

	// 平盘：不应出现金叉/排列类信号
	flat := flatBars(0, 40)
	got = evalKeys(e, flat)
	for _, k := range []string{"MACD_GOLDEN_FORK", "KDJ_GOLDEN_FORK", "LONG_AVG_ARRAY", "SHORT_AVG_ARRAY"} {
		if got[k] {
			t.Errorf("平盘不应命中 %s", k)
		}
	}
}

func TestCandlePatterns(t *testing.T) {
	e := &Engine{}
	mk := func(tail ...datasource.KLineBar) []datasource.KLineBar {
		return append(flatBars(0, 40), tail...)
	}

	tests := []struct {
		name string
		bars []datasource.KLineBar
		hit  string
		miss []string
	}{
		{
			name: "大阳线",
			bars: mk(mkBar(40, 100, 105.5, 99.5, 105, 1000)), // +5%
			hit:  "ONE_DAYANG_LINE",
		},
		{
			name: "乌云盖顶",
			bars: mk(
				mkBar(40, 100, 105.5, 99.5, 105, 1000), // 大阳
				mkBar(41, 106, 106.5, 101.5, 102, 1000), // 高开低走收阴，深入实体中点(102.5)下方
			),
			hit: "BLACK_CLOUD_TOPS",
		},
		{
			name: "穿头破脚",
			bars: mk(
				mkBar(40, 100, 103.5, 99.5, 103, 1000), // 阳线
				mkBar(41, 103.5, 104, 98.5, 99, 1000),  // 大阴吞没
			),
			hit: "BEARISH_ENGULFING",
		},
		{
			name: "早晨之星",
			bars: mk(
				mkBar(40, 104, 104.5, 99.5, 100, 1000),   // 大阴
				mkBar(41, 99.8, 100.2, 99.4, 99.9, 800), // 星线
				mkBar(42, 100.5, 105, 100.2, 104.8, 1200), // 大阳(4.4%)收复中点(102)上方
			),
			hit: "MORNING_STAR",
		},
		{
			name: "黄昏之星",
			bars: mk(
				mkBar(40, 100, 104.5, 99.5, 104, 1000),
				mkBar(41, 104.2, 104.8, 103.9, 104.3, 800),
				mkBar(42, 104, 104.2, 98.8, 99, 1200), // 大阴(-4.8%)跌破中点(102)
			),
			hit: "EVENING_STAR",
		},
		{
			name: "曙光初现",
			bars: mk(
				mkBar(40, 104, 104.5, 99.5, 100, 1000),   // 大阴
				mkBar(41, 99, 103.2, 98.5, 103, 1100),    // 低开高走深入实体中点(102)上方
			),
			hit: "FIRST_DAWN",
		},
		{
			name: "强势多方炮",
			bars: mk(
				mkBar(40, 100, 105.5, 99.5, 105, 1200), // 大阳
				mkBar(41, 104.5, 105, 103.5, 104, 800), // 小阴
				mkBar(42, 104.2, 108, 104, 107.5, 1300), // 大阳(3.2%)创新高
			),
			hit: "POWER_FULGUN",
		},
		{
			name: "身怀六甲",
			bars: mk(
				mkBar(40, 100, 106, 99.5, 105.5, 1000), // 大实体
				mkBar(41, 103, 103.8, 102.2, 102.8, 700), // 小实体被包含
			),
			hit: "PREGNANT",
		},
		{
			name: "射击之星",
			bars: append(trendBars(0, 100, 1.0, 40),
				mkBar(40, 141, 146, 141.0, 141.3, 1000)), // 上涨末端长上影、无下影
			hit: "SHOOTING_STAR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalKeys(e, tt.bars)
			if !got[tt.hit] {
				t.Errorf("应命中 %s，实际命中: %v", tt.hit, got)
			}
			for _, m := range tt.miss {
				if got[m] {
					t.Errorf("不应命中 %s", m)
				}
			}
		})
	}
}

func TestVolumeSignals(t *testing.T) {
	e := &Engine{}

	// 放量突破：20 日横盘后放量创新高
	bars := flatBars(0, 40)
	bars = append(bars, mkBar(40, 100, 101, 99.5, 101, 2000)) // 创20日新高 + 量2倍
	if got := evalKeys(e, bars); !got["BREAK_THROUGH"] {
		t.Error("放量突破未命中")
	}

	// 天量：量 3 倍以上
	bars = flatBars(0, 40)
	bars = append(bars, mkBar(40, 100, 101, 99, 100.5, 3500))
	if got := evalKeys(e, bars); !got["HEAVEN_RULE"] {
		t.Error("天量法则未命中")
	}

	// 下跌无量：连跌且缩量
	bars = flatBars(0, 40)
	bars = append(bars,
		mkBar(40, 100, 100.5, 98.5, 99, 600),
		mkBar(41, 99, 99.2, 97.5, 98, 500),
		mkBar(42, 98, 98.2, 96.5, 97, 400),
	)
	if got := evalKeys(e, bars); !got["DOWN_NARROW_VOLUME"] {
		t.Error("下跌无量未命中")
	}
}

func TestFundsSignals(t *testing.T) {
	// 低位 + 净流入 → 命中
	low := trendBars(0, 140, -1.0, 40) // 单边下跌，价格处于低位
	e := &Engine{Funds: func(code string) (float64, bool) { return 5e7, true }}
	if got := evalKeys(e, low); !got["LOW_FUNDS_INFLOW"] {
		t.Error("低位+净流入应命中 LOW_FUNDS_INFLOW")
	}

	// 高位 + 净流出 → 命中
	high := trendBars(0, 100, 1.0, 40)
	e2 := &Engine{Funds: func(code string) (float64, bool) { return -5e7, true }}
	if got := evalKeys(e2, high); !got["HIGH_FUNDS_OUTFLOW"] {
		t.Error("高位+净流出应命中 HIGH_FUNDS_OUTFLOW")
	}

	// 未注入 Provider → 资金信号不参与判定，不报错
	e3 := &Engine{}
	if got := evalKeys(e3, low); got["LOW_FUNDS_INFLOW"] {
		t.Error("无 Provider 时不应命中资金流信号")
	}

	// Provider 失败 → 降级跳过
	e4 := &Engine{Funds: func(code string) (float64, bool) { return 0, false }}
	if got := evalKeys(e4, low); got["LOW_FUNDS_INFLOW"] {
		t.Error("Provider 失败时不应命中资金流信号")
	}
}

func TestAsOfNoLookahead(t *testing.T) {
	e := &Engine{}
	// 前 40 根上涨 + 后 10 根暴跌；在上涨末端判定不应看到暴跌
	up := trendBars(0, 100, 1.0, 45)
	crash := trendBars(45, 145, -3.0, 10)
	bars := append(up, crash...)

	gotAt44 := keysOf(e.Evaluate("sz000001", bars, 44))
	if !gotAt44["LONG_AVG_ARRAY"] {
		t.Error("上涨末端应命中多头排列")
	}
	if gotAt44["SHORT_AVG_ARRAY"] || gotAt44["DOWN_7DAYS"] {
		t.Error("asOf 截断后不应看到未来的下跌（未来函数泄漏）")
	}

	gotLatest := keysOf(e.Evaluate("sz000001", bars, -1))
	if !gotLatest["DOWN_7DAYS"] {
		t.Error("最新时点应命中七连阴")
	}
}

func TestRegistryMetadataComplete(t *testing.T) {
	// 注册表元数据完整性：30 个信号、key 唯一、名称/说明非空
	reg := Registry()
	if len(reg) != 30 {
		t.Errorf("信号数量 = %d, want 30", len(reg))
	}
	seen := map[string]bool{}
	for _, d := range reg {
		if d.Key == "" || d.Name == "" || d.Tip == "" || d.Category == "" {
			t.Errorf("信号 %+v 元数据不完整", d)
		}
		if seen[d.Key] {
			t.Errorf("信号 key 重复: %s", d.Key)
		}
		seen[d.Key] = true
		if d.detect == nil {
			t.Errorf("信号 %s 缺少判定函数", d.Key)
		}
	}
}

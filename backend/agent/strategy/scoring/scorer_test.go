package scoring

import (
	"math"
	"testing"
)

// makeKLine 构造 n 根线性漂移 K 线（收盘价从 start 起每根变化 drift）。
func makeKLine(n int, start, drift float64) []KLineBar {
	bars := make([]KLineBar, n)
	for i := 0; i < n; i++ {
		c := start + float64(i)*drift
		bars[i] = KLineBar{Open: c, High: c * 1.001, Low: c * 0.999, Close: c, Volume: 10000, Amount: 1e8}
	}
	return bars
}

// makeAlternatingKLine 构造高开低走的锯齿 K 线（高波动/大回撤场景）。
func makeAlternatingKLine(n int, start, amplitude float64) []KLineBar {
	bars := make([]KLineBar, n)
	c := start
	for i := 0; i < n; i++ {
		if i > 0 {
			if i%2 == 0 {
				c = c * (1 + amplitude/100)
			} else {
				c = c * (1 - amplitude/100)
			}
		}
		bars[i] = KLineBar{Open: c, High: c * 1.001, Low: c * 0.999, Close: c, Volume: 10000, Amount: 1e8}
	}
	return bars
}

func almostEqual(t *testing.T, name string, got, want, delta float64) {
	t.Helper()
	if math.Abs(got-want) > delta {
		t.Errorf("%s: got %.4f, want %.4f (±%.4f)", name, got, want, delta)
	}
}

// ---------- Value ----------

func TestValueFactor(t *testing.T) {
	f := NewValueFactor()

	// 低估值（百分位低）应得高分
	r := f.Score(&FactorInput{PE: 10, PB: 1, PEPercentile: 0.1, PBPercentile: 0.2})
	almostEqual(t, "低估值得分", r.Score, 0.35*90+0.65*80, 0.01)

	// 高估值（百分位高）应得低分
	r = f.Score(&FactorInput{PE: 80, PB: 10, PEPercentile: 0.95, PBPercentile: 0.9})
	almostEqual(t, "高估值得分", r.Score, 0.35*5+0.65*10, 0.01)

	// 亏损股（PE<=0）PE 分量按最差处理，仅 PB 贡献
	r = f.Score(&FactorInput{PE: -5, PB: 1, PEPercentile: 1, PBPercentile: 0.0})
	almostEqual(t, "亏损股得分", r.Score, 0.65*100, 0.01)
}

// ---------- Liquidity ----------

func TestLiquidityFactor(t *testing.T) {
	f := NewLiquidityFactor()

	r := f.Score(&FactorInput{Amount: 1e9, AmountPercentile: 1.0})
	almostEqual(t, "最高流动性", r.Score, 100, 0.01)

	r = f.Score(&FactorInput{Amount: 1e8, AmountPercentile: 0.5})
	almostEqual(t, "中等流动性", r.Score, 50, 0.01)

	r = f.Score(&FactorInput{Amount: 1e6, AmountPercentile: 0.0})
	almostEqual(t, "最低流动性", r.Score, 0, 0.01)
}

// ---------- Size ----------

func TestSizeFactor(t *testing.T) {
	f := NewSizeFactor()

	// 小市值（百分位低）应得高分
	r := f.Score(&FactorInput{TotalCap: 3e9, CapPercentile: 0.05})
	almostEqual(t, "小市值", r.Score, 95, 0.01)

	r = f.Score(&FactorInput{TotalCap: 5e10, CapPercentile: 0.5})
	almostEqual(t, "中市值", r.Score, 50, 0.01)

	// 大市值（百分位高）应得低分
	r = f.Score(&FactorInput{TotalCap: 2e12, CapPercentile: 0.98})
	almostEqual(t, "大市值", r.Score, 2, 0.01)
}

// ---------- Momentum ----------

func TestMomentumFactor(t *testing.T) {
	f := NewMomentumFactor()

	// 平稳行情：日内 0%、60 日趋势 0%，无惩罚
	r := f.Score(&FactorInput{ChangePercent: 0, KLine: makeKLine(80, 10, 0.001)})
	if r.Score < 45 || r.Score > 70 {
		t.Errorf("平稳行情得分应在 45-70 区间, got %.2f", r.Score)
	}
	almostEqual(t, "平稳无惩罚", r.Detail["penalty"], 0, 0.01)

	// 强势行情：日内 +3%（未触发追涨）、60 日趋势上行
	strong := f.Score(&FactorInput{ChangePercent: 3, KLine: makeKLine(80, 10, 0.08)})
	if strong.Score <= r.Score {
		t.Errorf("强势行情得分(%.2f)应高于平稳行情(%.2f)", strong.Score, r.Score)
	}

	// 追涨惩罚：当日 +6% 触发 ChasePenalty
	chase := f.Score(&FactorInput{ChangePercent: 6, KLine: makeKLine(80, 10, 0.08)})
	almostEqual(t, "追涨惩罚", chase.Detail["penalty"], f.ChasePenalty, 0.01)

	// 过热惩罚：60 日涨幅 >45%（10→15，+50%）
	heat := f.Score(&FactorInput{ChangePercent: 0, KLine: []KLineBar{
		{Close: 10, Open: 10, High: 10, Low: 10},
		{Close: 10, Open: 10, High: 10, Low: 10},
		{Close: 15, Open: 15, High: 15, Low: 15},
	}})
	almostEqual(t, "过热惩罚", heat.Detail["penalty"], f.HeatPenalty, 0.01)

	// 破位惩罚：60 日跌幅 >20%（10→7.5，-25%）
	brk := f.Score(&FactorInput{ChangePercent: 0, KLine: []KLineBar{
		{Close: 10, Open: 10, High: 10, Low: 10},
		{Close: 10, Open: 10, High: 10, Low: 10},
		{Close: 7.5, Open: 7.5, High: 7.5, Low: 7.5},
	}})
	almostEqual(t, "破位惩罚", brk.Detail["penalty"], f.BreakPenalty, 0.01)
}

// ---------- Reversal ----------

func TestReversalFactor(t *testing.T) {
	f := NewReversalFactor()
	neutralK := makeAlternatingKLine(30, 10, 0.5) // 锯齿波动，RSI 保持中性（单调序列 RSI 会顶到 100）

	// 理想跌幅 -3%：得分接近满分
	r := f.Score(&FactorInput{ChangePercent: -3, KLine: neutralK})
	if r.Score < 90 {
		t.Errorf("理想跌幅得分应接近 100, got %.2f", r.Score)
	}

	// 偏离理想值（+2%）：按偏离扣 5×8=40 分
	r2 := f.Score(&FactorInput{ChangePercent: 2, KLine: neutralK})
	if r2.Score >= r.Score {
		t.Errorf("偏离理想跌幅得分(%.2f)应低于理想跌幅(%.2f)", r2.Score, r.Score)
	}
	almostEqual(t, "偏离度", r2.Detail["deviation"], 5, 0.01)

	// 崩盘 -9%：追加 20 分惩罚
	crash := f.Score(&FactorInput{ChangePercent: -9, KLine: neutralK})
	almostEqual(t, "崩盘惩罚", crash.Detail["crash_penalty"], f.CrashPenalty, 0.01)

	// RSI 超卖（连续下跌 RSI→0）：+10 分
	oversold := f.Score(&FactorInput{ChangePercent: -3, KLine: makeKLine(30, 15, -0.3)})
	almostEqual(t, "超卖加分", oversold.Detail["rsi_adj"], f.RSIOversoldBonus, 0.01)

	// RSI 超买（连续上涨 RSI→100）：扣分幅度为 -|RSIOverboughtCut|
	overbought := f.Score(&FactorInput{ChangePercent: -3, KLine: makeKLine(30, 10, 0.3)})
	almostEqual(t, "超买减分", overbought.Detail["rsi_adj"], -math.Abs(f.RSIOverboughtCut), 0.01)
}

// ---------- Activity ----------

func TestActivityFactor(t *testing.T) {
	f := NewActivityFactor()

	// 理想值：量比 2.0 + 换手 4.0 → 满分
	r := f.Score(&FactorInput{VolumeRatio: 2.0, TurnoverRate: 4.0})
	almostEqual(t, "理想活跃度", r.Score, 100, 0.01)

	// 极度偏离：量比 6(偏离 4×25=100 扣)、换手 12(偏离 8×12.5=100 扣) → 0 分
	r = f.Score(&FactorInput{VolumeRatio: 6, TurnoverRate: 12})
	almostEqual(t, "极度偏离", r.Score, 0, 0.01)

	// 部分偏离：量比 3.0(偏离 1×25=25 扣)、换手 6.0(偏离 2×12.5=25 扣)
	r = f.Score(&FactorInput{VolumeRatio: 3.0, TurnoverRate: 6.0})
	almostEqual(t, "部分偏离", r.Score, 75, 0.01)
}

// ---------- Stability ----------

func TestStabilityFactor(t *testing.T) {
	f := NewStabilityFactor()

	// 平稳 K 线：无扣减，得起步分 78
	calm := f.Score(&FactorInput{Price: 10, KLine: makeKLine(60, 10, 0.002)})
	almostEqual(t, "平稳起步分", calm.Score, f.BaseScore, 0.01)

	// 高波动（±5% 锯齿 → 年化波动率远超 45%）：触发波动率扣减
	volatileK := f.Score(&FactorInput{Price: 10, KLine: makeAlternatingKLine(60, 10, 5)})
	if volatileK.Detail["vol_deduct"] != f.VolPenalty {
		t.Errorf("高波动应扣 %.0f 分, deduct=%.2f", f.VolPenalty, volatileK.Detail["vol_deduct"])
	}
	if volatileK.Score >= calm.Score {
		t.Errorf("高波动得分(%.2f)应低于平稳(%.2f)", volatileK.Score, calm.Score)
	}

	// 大回撤：从 10 涨到 12 再跌到 10.2（回撤 -15%）触发回撤扣减
	ddK := append(makeKLine(30, 10, 0.067), makeKLine(30, 12, -0.06)...)
	dd := f.Score(&FactorInput{Price: 10.2, KLine: ddK})
	if dd.Detail["drawdown_deduct"] != f.DrawdownPenalty {
		t.Errorf("大回撤应扣 %.0f 分, deduct=%.2f (mdd=%.2f)", f.DrawdownPenalty, dd.Detail["drawdown_deduct"], dd.Detail["max_drawdown"])
	}

	// 数据不足：中性分 50
	r := f.Score(&FactorInput{Price: 10, KLine: makeKLine(5, 10, 0.1)})
	almostEqual(t, "数据不足", r.Score, f.InsufficientScore, 0.01)
}

// ---------- ThemeHeat ----------

func TestThemeHeatFactor(t *testing.T) {
	f := NewThemeHeatFactor()

	// 无数据：中性分
	r := f.Score(&FactorInput{})
	almostEqual(t, "无热度数据", r.Score, f.NeutralScore, 0.01)

	// 升温中：基础 60×0.5=30 + 趋势 2×5=10 + 持续 5×3=15 + 观察 10×0.5=5 = 60
	hot := f.Score(&FactorInput{ThemeHeat: &ThemeHeatInput{
		Latest: 60, Trend: 2, PersistenceDays: 5, Cooling: 0, WatchCount: 10,
	}})
	almostEqual(t, "升温综合", hot.Score, 30+10+15+5, 0.01)

	// 降温 + 过热：最新 90(基础 45)、降温 10×1.5=15 扣、过热 (90-85)×2=10 扣
	cooling := f.Score(&FactorInput{ThemeHeat: &ThemeHeatInput{
		Latest: 90, Trend: -4, PersistenceDays: 0, Cooling: 10, WatchCount: 0,
	}})
	almostEqual(t, "降温扣分", cooling.Detail["cooling_deduct"], 15, 0.01)
	almostEqual(t, "过热扣分", cooling.Detail["overheat_deduct"], 10, 0.01)
	if cooling.Score >= hot.Score {
		t.Errorf("降温过热得分(%.2f)应低于升温(%.2f)", cooling.Score, hot.Score)
	}
}

// ---------- TopicAlignment ----------

func TestTopicAlignmentFactor(t *testing.T) {
	f := NewTopicAlignmentFactor()

	// 完全命中：3 个热点 token 全部命中 → 满分
	r := f.Score(&FactorInput{
		Industry:       "半导体",
		Concepts:       []string{"芯片", "人工智能"},
		HotTopicTokens: []string{"半导体", "芯片", "人工智能"},
	})
	almostEqual(t, "完全命中", r.Score, 100, 0.01)
	almostEqual(t, "命中数", r.Detail["hits"], 3, 0.01)

	// 无命中 → 0 分
	r = f.Score(&FactorInput{
		Industry:       "白酒",
		Concepts:       []string{"消费"},
		HotTopicTokens: []string{"机器人", "AI算力"},
	})
	almostEqual(t, "无命中", r.Score, 0, 0.01)

	// 空热点 token → 0 分
	r = f.Score(&FactorInput{Industry: "半导体"})
	almostEqual(t, "空热点", r.Score, 0, 0.01)

	// 子串包含匹配："人形机器人" 命中 "机器人"
	r = f.Score(&FactorInput{
		Concepts:       []string{"人形机器人"},
		HotTopicTokens: []string{"机器人"},
	})
	almostEqual(t, "包含匹配", r.Score, 100, 0.01)
}

// ---------- PercentileRanks ----------

func TestPercentileRanks(t *testing.T) {
	// 常规排序：最小值百分位 0，最大值百分位 1
	ranks := PercentileRanks([]float64{30, 10, 20})
	almostEqual(t, "最小值", ranks[1], 0, 0.01)
	almostEqual(t, "中位值", ranks[2], 0.5, 0.01)
	almostEqual(t, "最大值", ranks[0], 1, 0.01)

	// 空输入与单值
	if len(PercentileRanks(nil)) != 0 {
		t.Error("空输入应返回空切片")
	}
	almostEqual(t, "单值", PercentileRanks([]float64{42})[0], 0.5, 0.01)

	// 并列值取中点秩
	ranks = PercentileRanks([]float64{10, 10, 30})
	almostEqual(t, "并列秩", ranks[0], ranks[1], 0.001)
}

// ---------- Scorer ----------

func TestDefaultWeights(t *testing.T) {
	w := DefaultWeights(0.35)
	almostEqual(t, "value 权重", w["value"], 0.325, 0.0001)
	almostEqual(t, "momentum 权重", w["momentum"], 0.1925, 0.0001)
	almostEqual(t, "activity 权重", w["activity"], 0.1575, 0.0001)
	almostEqual(t, "liquidity 权重", w["liquidity"], 0.1625, 0.0001)
	almostEqual(t, "stability 权重", w["stability"], 0.1625, 0.0001)
	almostEqual(t, "reversal 默认关闭", w["reversal"], 0, 0.0001)

	sum := 0.0
	for _, v := range w {
		sum += v
	}
	almostEqual(t, "权重和为 1", sum, 1.0, 0.0001)
}

func TestScorerScore(t *testing.T) {
	s := NewScorer(DefaultScorerConfig())

	if len(s.FactorNames()) != 9 {
		t.Fatalf("应注册 9 个因子, got %d", len(s.FactorNames()))
	}

	// 确定性输入：空 K 线 + 固定百分位
	input := &FactorInput{
		Code:             "600000",
		ChangePercent:    2,
		PE:               15,
		PB:               2,
		PEPercentile:     0.2,
		PBPercentile:     0.4,
		AmountPercentile: 0.6,
		VolumeRatio:      2.0,
		TurnoverRate:     4.0,
	}
	r := s.Score(input)

	// 手算期望：value=67, momentum=59.2, activity=100, liquidity=60, stability=50(数据不足)
	want := 0.325*67 + 0.1925*59.2 + 0.1575*100 + 0.1625*60 + 0.1625*50
	almostEqual(t, "加权综合分", r.Total, want, 0.01)

	// 各因子明细齐全
	for _, name := range []string{"value", "liquidity", "momentum", "reversal", "activity", "stability", "size", "theme_heat", "topic_alignment"} {
		if _, ok := r.Factors[name]; !ok {
			t.Errorf("缺少因子明细: %s", name)
		}
	}

	// 综合分与各因子分加权和自洽
	sum := 0.0
	for name, fr := range r.Factors {
		sum += fr.Score * r.Weights[name]
	}
	almostEqual(t, "加权和自洽", r.Total, clamp100(sum), 0.01)
}

func TestScorerCustomWeights(t *testing.T) {
	// 显式权重覆盖 + 自动归一化（0.5/0.5 → 各 50%）
	s := NewScorer(ScorerConfig{Weights: map[string]float64{"value": 0.5, "activity": 0.5}})
	r := s.Score(&FactorInput{
		PE: 10, PB: 1, PEPercentile: 0, PBPercentile: 0,
		VolumeRatio: 2.0, TurnoverRate: 4.0,
	})
	// value=100, activity=100 → 100
	almostEqual(t, "自定义权重满分", r.Total, 100, 0.01)
	almostEqual(t, "归一化权重", r.Weights["value"], 0.5, 0.01)

	// 显式全零权重：归一化失败，退化为 9 因子等权
	s2 := NewScorer(ScorerConfig{Weights: map[string]float64{"value": 0, "activity": 0}})
	r2 := s2.Score(&FactorInput{})
	sum := 0.0
	for _, fr := range r2.Factors {
		sum += fr.Score
	}
	almostEqual(t, "全零权重等权", r2.Total, sum/9, 0.01)
}

func TestLoadScorerConfigJSON(t *testing.T) {
	// 显式 techWeight
	cfg, err := LoadScorerConfigJSON([]byte(`{"techWeight": 0.5}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "techWeight", cfg.TechWeight, 0.5, 0.0001)

	// 缺省回退 0.35
	cfg, err = LoadScorerConfigJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "默认 techWeight", cfg.TechWeight, 0.35, 0.0001)

	// 显式 weights
	cfg, err = LoadScorerConfigJSON([]byte(`{"weights": {"value": 1.0}}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "显式 weights", cfg.Weights["value"], 1.0, 0.0001)

	// 非法 JSON
	if _, err = LoadScorerConfigJSON([]byte(`{invalid`)); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}

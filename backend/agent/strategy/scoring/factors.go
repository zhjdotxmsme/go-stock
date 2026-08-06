// Package scoring 实现 9 因子量化评分系统（方案 §8.1 D1）。
// 因子为纯函数，只依赖 FactorInput 输入结构，不直连任何数据源；
// 数据装配（含百分位排名所需的候选池上下文）由调用方完成。
package scoring

// KLineBar 单根日 K 线数据（时间升序排列，最后一根为最新）。
type KLineBar struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Amount float64
}

// ThemeHeatInput 板块热度多信号输入（对应 DSA 板块热度 6 维评分的核心信号）。
type ThemeHeatInput struct {
	Latest          float64 // 最新热度 0-100
	Trend           float64 // 热度趋势（近期变化斜率，正=升温，负=降温）
	PersistenceDays int     // 热度持续天数
	Cooling         float64 // 降温幅度（正数表示正在降温的幅度）
	WatchCount      int     // 观察数
}

// FactorInput 因子计算输入。百分位相关字段（xxxPercentile，取值 0~1）
// 需要候选池上下文才能计算，由调用方预先用 PercentileRanks 装配。
type FactorInput struct {
	Code          string
	Name          string
	Price         float64    // 最新价
	ChangePercent float64    // 当日涨跌幅 %
	Amount        float64    // 当日成交额（元）
	TurnoverRate  float64    // 换手率 %
	VolumeRatio   float64    // 量比
	PE            float64    // 市盈率（<=0 视为亏损股）
	PB            float64    // 市净率
	TotalCap      float64    // 总市值（元）
	Industry      string     // 所属行业
	Concepts      []string   // 所属概念列表
	KLine         []KLineBar // 日线序列，时间升序

	// 候选池百分位排名（0~1，越大表示在候选池中排位越靠后/数值越大），由调用方装配
	PEPercentile     float64
	PBPercentile     float64
	AmountPercentile float64
	CapPercentile    float64

	ThemeHeat      *ThemeHeatInput // 板块热度数据（可空，空则中性分）
	HotTopicTokens []string        // 热点主题关键词 token 集合
}

// FactorResult 单个因子的计算结果。
type FactorResult struct {
	Name   string             // 因子名
	Score  float64            // 因子得分 0-100
	Detail map[string]float64 // 中间量，便于解释与调试
}

// Factor 因子接口：纯函数，无副作用。
type Factor interface {
	Name() string
	Score(input *FactorInput) FactorResult
}

// clamp 将 v 限制在 [lo, hi]。
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clamp100 将得分限制在 0-100。
func clamp100(v float64) float64 {
	return clamp(v, 0, 100)
}

// PercentileRanks 计算一组候选中某个数值字段的百分位排名（0~1）。
// 返回值与输入下标一一对应：值越小百分位越接近 0，最大值百分位为 1。
// 空输入或全部相等时返回全 0.5。
func PercentileRanks(values []float64) []float64 {
	n := len(values)
	ranks := make([]float64, n)
	if n == 0 {
		return ranks
	}
	if n == 1 {
		ranks[0] = 0.5
		return ranks
	}
	for i := 0; i < n; i++ {
		less, equal := 0, 0
		for j := 0; j < n; j++ {
			if values[j] < values[i] {
				less++
			} else if values[j] == values[i] {
				equal++
			}
		}
		// 使用中点秩处理并列
		ranks[i] = (float64(less) + float64(equal-1)/2) / float64(n-1)
	}
	return ranks
}

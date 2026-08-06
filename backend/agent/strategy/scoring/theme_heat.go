package scoring

// ThemeHeatFactor 板块热度因子（15 参数）：
// 多信号综合，各有上限和斜率：
//   - 基础分：最新热度 × 基础斜率
//   - 趋势分：热度趋势为正时加分（斜率 × 趋势值，封顶）
//   - 持续分：持续天数 × 斜率，封顶
//   - 降温分：降温幅度 × 斜率扣分，封顶
//   - 过热分：最新热度超过过热阈值时按超出幅度扣分，封顶
//
// 无板块热度数据（ThemeHeat 为空）时返回中性分。
type ThemeHeatFactor struct {
	BaseWeight        float64 // 最新热度基础斜率，默认 0.5（热度 100 → 50 基础分）
	TrendSlope        float64 // 趋势斜率，默认 5
	TrendCap          float64 // 趋势分上限，默认 20
	PersistSlope      float64 // 持续天数斜率，默认 3
	PersistCap        float64 // 持续分上限，默认 15
	CoolingSlope      float64 // 降温斜率，默认 1.5
	CoolingCap        float64 // 降温扣分上限，默认 25
	OverheatThreshold float64 // 过热阈值（最新热度），默认 85
	OverheatSlope     float64 // 过热扣分斜率，默认 2
	OverheatCap       float64 // 过热扣分上限，默认 20
	WatchBonusSlope   float64 // 观察数加分斜率，默认 0.5
	WatchBonusCap     float64 // 观察数加分上限，默认 10
	NeutralScore      float64 // 无数据时中性分，默认 50
	TrendDamping      float64 // 趋势为负时的衰减系数，默认 0.5（降温趋势部分计入）
	MaxScore          float64 // 得分上限，默认 100
}

func NewThemeHeatFactor() *ThemeHeatFactor {
	return &ThemeHeatFactor{
		BaseWeight:        0.5,
		TrendSlope:        5,
		TrendCap:          20,
		PersistSlope:      3,
		PersistCap:        15,
		CoolingSlope:      1.5,
		CoolingCap:        25,
		OverheatThreshold: 85,
		OverheatSlope:     2,
		OverheatCap:       20,
		WatchBonusSlope:   0.5,
		WatchBonusCap:     10,
		NeutralScore:      50,
		TrendDamping:      0.5,
		MaxScore:          100,
	}
}

func (f *ThemeHeatFactor) Name() string { return "theme_heat" }

func (f *ThemeHeatFactor) Score(input *FactorInput) FactorResult {
	if input.ThemeHeat == nil {
		return FactorResult{Name: f.Name(), Score: f.NeutralScore, Detail: map[string]float64{}}
	}
	th := input.ThemeHeat

	// 基础分
	base := clamp(th.Latest, 0, 100) * f.BaseWeight

	// 趋势分：正趋势加分，负趋势按衰减系数扣分，各封顶
	trendScore := 0.0
	if th.Trend >= 0 {
		trendScore = clamp(th.Trend*f.TrendSlope, 0, f.TrendCap)
	} else {
		trendScore = clamp(th.Trend*f.TrendSlope*f.TrendDamping, -f.TrendCap, 0)
	}

	// 持续分
	persistScore := clamp(float64(th.PersistenceDays)*f.PersistSlope, 0, f.PersistCap)

	// 观察数加分
	watchScore := clamp(float64(th.WatchCount)*f.WatchBonusSlope, 0, f.WatchBonusCap)

	// 降温分（扣分）
	coolingDeduct := clamp(th.Cooling*f.CoolingSlope, 0, f.CoolingCap)

	// 过热分（扣分）
	overheatDeduct := 0.0
	if th.Latest > f.OverheatThreshold {
		overheatDeduct = clamp((th.Latest-f.OverheatThreshold)*f.OverheatSlope, 0, f.OverheatCap)
	}

	score := base + trendScore + persistScore + watchScore - coolingDeduct - overheatDeduct

	return FactorResult{
		Name:  f.Name(),
		Score: clamp(score, 0, f.MaxScore),
		Detail: map[string]float64{
			"base":            base,
			"trend_score":     trendScore,
			"persist_score":   persistScore,
			"watch_score":     watchScore,
			"cooling_deduct":  coolingDeduct,
			"overheat_deduct": overheatDeduct,
		},
	}
}

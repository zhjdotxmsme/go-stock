package data

// strategy_high_tight_flag.go —— 高而窄旗形策略（移植自 InStock high_tight_flag）。
//
// 形态定义（需至少 60 根 K 线；观察窗 = 倒数第 24~11 根，共 14 根）：
//  1. 观察窗末端最高价 / 观察窗最低价 >= 1.9（短期近乎翻倍的急涨旗杆）；
//  2. 观察窗内存在连续两日涨幅 >= 9.5%（急涨由涨停驱动）。
//
// 评分：旗杆高度 50 + 连续涨停 50，满分 100。
// 说明：InStock 原版以龙虎榜机构（istop）为前置门槛且默认关闭，本移植不依赖
// 龙虎榜数据，仅保留纯 K 线形态判定。
type HighTightFlagStrategy struct{}

func (s *HighTightFlagStrategy) Name() string { return "高窄旗形" }
func (s *HighTightFlagStrategy) Code() string { return "high_tight_flag" }
func (s *HighTightFlagStrategy) Description() string {
	return "10-24 日前连续涨停急涨近翻倍（旗杆），随后高位窄旗整理"
}

func (s *HighTightFlagStrategy) Score(ctx *StrategyContext) *StrategyResult {
	fail := &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: ""}
	n := len(ctx.CloseP)
	if n < 60 || len(ctx.LowP) < n || len(ctx.HighP) < n {
		fail.Signal = "K线数据不足（需≥60根）"
		return fail
	}

	// 观察窗 [n-24, n-11]（对齐 InStock tail(24).head(14)）
	ws, we := n-24, n-11
	low := ctx.LowP[ws]
	for i := ws; i <= we; i++ {
		if ctx.LowP[i] > 0 && ctx.LowP[i] < low {
			low = ctx.LowP[i]
		}
	}

	factors := map[string]float64{}

	// 条件1：旗杆高度（窗末最高价 / 窗口最低价 >= 1.9）
	if low > 0 && ctx.HighP[we]/low >= 1.9 {
		factors["flagpole_height"] = 50
	}

	// 条件2：窗口内连续两日涨幅 >= 9.5%
	consec := false
	prevHot := false
	for i := ws; i <= we; i++ {
		if ctx.CloseP[i-1] <= 0 {
			prevHot = false
			continue
		}
		hot := (ctx.CloseP[i]/ctx.CloseP[i-1]-1)*100 >= 9.5
		if hot && prevHot {
			consec = true
			break
		}
		prevHot = hot
	}
	if consec {
		factors["consecutive_limit_up"] = 50
	}

	score := 0.0
	for _, v := range factors {
		score += v
	}
	res := &StrategyResult{Score: score, Factors: factors}
	switch {
	case score >= 100:
		res.Signal = "高窄旗形成立：连续涨停急涨旗杆 + 高位整理"
	case score >= 50:
		res.Signal = "旗杆高度或连续涨停仅满足其一"
	default:
		res.Signal = "无高窄旗形"
	}
	return res
}

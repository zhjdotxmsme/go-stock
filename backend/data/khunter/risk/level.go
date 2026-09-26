package risk

// Level 风险档位
type Level struct {
	Name          string
	PositionLimit float64 // 仓位上限（0~1）
	ScoreExtra    float64 // 狩猎场入选分数加成
}

// LevelOf 按单日 VaR 分档（阈值 −3%/−5%/−8%）；var1d==0（数据不足）按"注意"保守处理
func LevelOf(var1d float64) Level {
	switch {
	case var1d == 0:
		return Level{"注意", 0.7, 5}
	case var1d > -0.03:
		return Level{"正常", 1.0, 0}
	case var1d > -0.05:
		return Level{"注意", 0.7, 5}
	case var1d > -0.08:
		return Level{"危险", 0.4, 15}
	default:
		return Level{"崩溃", 0, 999}
	}
}

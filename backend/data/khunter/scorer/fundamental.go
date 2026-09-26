package scorer

import "math"

// FundamentalInput 基本面输入（取 end_date <= 评分日的最新一期，防未来函数）
type FundamentalInput struct {
	Available    bool
	NetProfitYoy float64 // 净利润同比 %
	ROE          float64 // 加权 ROE %
	OcfToIncome  float64 // 经营现金流/营收
	HasYoy       bool
	HasROE       bool
	HasOcf       bool
}

// ScoreFundamental = 50 + 增速分 + ROE分 + 现金流分，截断 [-100,100]
func ScoreFundamental(f FundamentalInput) DimScore {
	if !f.Available {
		return DimScore{Score: 50, Degraded: true, Reason: "基本面数据缺失"}
	}
	// 一票否决
	if f.HasYoy && f.NetProfitYoy < -50 {
		return DimScore{Score: -100, Veto: true, Reason: "净利润同比下滑超50%"}
	}
	if f.HasROE && f.ROE < -5 {
		return DimScore{Score: -100, Veto: true, Reason: "ROE低于-5%"}
	}
	score := 50.0
	if f.HasYoy {
		switch {
		case f.NetProfitYoy > 30:
			score += 20
		case f.NetProfitYoy >= 0:
			score += 10
		default:
			score -= 20
		}
	}
	if f.HasROE {
		switch {
		case f.ROE > 15:
			score += 20
		case f.ROE >= 5:
			score += 10
		case f.ROE < 0:
			score -= 20
		}
	}
	if f.HasOcf {
		switch {
		case f.OcfToIncome > 1:
			score += 20
		case f.OcfToIncome > 0:
			score += 10
		case f.OcfToIncome < 0:
			score -= 20
		}
	}
	return DimScore{Score: math.Max(-100, math.Min(100, score))}
}

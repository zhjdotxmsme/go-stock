package scorer

import (
	"fmt"
	"math"
	"strings"
)

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
		return DimScore{Score: 50, Degraded: true, Missing: true, Reason: "基本面数据缺失（F10 财务指标获取失败）"}
	}
	// 一票否决
	parts := fundamentalParts(f)
	if f.HasYoy && f.NetProfitYoy < -50 {
		return DimScore{Score: -100, Veto: true, Reason: "净利润同比下滑超过 50%，一票否决。" + strings.Join(parts, "")}
	}
	if f.HasROE && f.ROE < -5 {
		return DimScore{Score: -100, Veto: true, Reason: "ROE 低于 -5%，一票否决。" + strings.Join(parts, "")}
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
	text := strings.Join(parts, "")
	if text == "" {
		text = "财务指标已取到，但同比、ROE、现金流三项都没有有效值，维持基准 50 分。"
	}
	return DimScore{Score: math.Max(-100, math.Min(100, score)), Reason: text}
}

func fundamentalParts(f FundamentalInput) []string {
	var parts []string
	if f.HasYoy {
		tone := "同比下滑"
		switch {
		case f.NetProfitYoy > 30:
			tone = "高增长"
		case f.NetProfitYoy >= 0:
			tone = "正增长"
		}
		parts = append(parts, fmt.Sprintf("净利润同比 %+.1f%%（%s）。", f.NetProfitYoy, tone))
	} else {
		parts = append(parts, "净利润同比缺失，这项不加分。")
	}
	if f.HasROE {
		tone := "偏低"
		switch {
		case f.ROE > 15:
			tone = "较高"
		case f.ROE >= 5:
			tone = "中等"
		case f.ROE < 0:
			tone = "为负"
		}
		parts = append(parts, fmt.Sprintf("加权 ROE %+.1f%%（%s）。", f.ROE, tone))
	} else {
		parts = append(parts, "ROE 缺失，这项不加分。")
	}
	if f.HasOcf {
		tone := "经营现金流为负"
		switch {
		case f.OcfToIncome > 1:
			tone = "现金流超过营收"
		case f.OcfToIncome > 0:
			tone = "现金流为正"
		}
		parts = append(parts, fmt.Sprintf("经营现金流/营收 %.2f（%s）。", f.OcfToIncome, tone))
	} else {
		parts = append(parts, "现金流数据缺失，这项不加分。")
	}
	return parts
}

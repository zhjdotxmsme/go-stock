package khunter

import (
	"math"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// NormalizeVolume 按数据源把原始成交量归一为股，并返回内部单位标记。
// 口径唯一来源是 datasource.NormalizeKLineVolume：eastmoney_kline 为手需 ×100，
// mootdx_kline / freestockdb / seed 已是股，其余来源保持 unknown 且不改数值。
func NormalizeVolume(source string, raw int64) (int64, string) {
	return datasource.NormalizeKLineVolume(source, raw)
}

// DataDecision 数据质量闸门结论，分别对应展示/观察、确认推荐、回测三类放行。
type DataDecision struct {
	AllowObservation bool
	AllowConfirm     bool
	AllowBacktest    bool
	Status           string   // valid / invalid / intraday
	Reasons          []string // 全部不放行原因，供前端"讲清楚"
}

// ValidateDailyBar 校验单根日 K 的质量与单位，返回全部不通过原因。
// 全内部口径：量为正且单位为股、额为正且币种为人民币元、OHLC 为正且高低关系成立。
// 旧记录单位为空同样判为不通过（允许展示，禁止确认）。
func ValidateDailyBar(b models.KLineBar) []string {
	var reasons []string
	if b.Open <= 0 || b.High <= 0 || b.Low <= 0 || b.Close <= 0 {
		reasons = append(reasons, "OHLC 非正数")
	}
	if b.High < math.Max(b.Open, b.Close) ||
		b.Low > math.Min(b.Open, b.Close) || b.High < b.Low {
		reasons = append(reasons, "OHLC 关系异常")
	}
	if b.Volume <= 0 || b.VolumeUnit != "share" {
		reasons = append(reasons, "成交量缺失或单位未知")
	}
	if b.Amount <= 0 || b.AmountUnit != "CNY" {
		reasons = append(reasons, "成交额缺失或单位未知")
	}
	return reasons
}

// EvaluateDataGate 评估目标日 K 线是否允许展示/确认/回测。
// 只检查目标日最后一根 K 线的质量及序列日期（不得含晚于目标日的未来数据）：
//   - syncCompleted=false：盘中或同步未完成，允许观察、禁止确认，Status=intraday；
//   - 质量校验失败（含旧记录单位为空、零量额、OHLC 异常）：允许展示、禁止确认，
//     Status=invalid 并给出全部原因；
//   - 全部通过：Status=valid，三类放行。
func EvaluateDataGate(bars []models.KLineBar, asOfDate string, syncCompleted bool) DataDecision {
	for _, b := range bars {
		if b.TradeDate > asOfDate {
			return DataDecision{
				Status:  "invalid",
				Reasons: []string{"序列包含晚于 " + asOfDate + " 的K线，时点数据被污染"},
			}
		}
	}
	var last models.KLineBar
	found := false
	for _, b := range bars {
		if b.TradeDate == asOfDate {
			last = b
			found = true
		}
	}
	if !found {
		return DataDecision{
			Status:  "invalid",
			Reasons: []string{"目标日 " + asOfDate + " 无K线"},
		}
	}

	reasons := ValidateDailyBar(last)
	switch {
	case !syncCompleted:
		return DataDecision{
			AllowObservation: true,
			Status:           "intraday",
			Reasons:          append(reasons, "盘中未收盘或同步未完成，只可观察不得确认"),
		}
	case len(reasons) > 0:
		return DataDecision{AllowObservation: true, Status: "invalid", Reasons: reasons}
	default:
		return DataDecision{
			AllowObservation: true,
			AllowConfirm:     true,
			AllowBacktest:    true,
			Status:           "valid",
		}
	}
}

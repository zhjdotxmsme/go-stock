package filter

import (
	"fmt"
	"regexp"
)

// 快照级过滤（方案 §8.1 D7：9 类规则，15 参数）。

// SnapshotRules 构建快照级规则集。
func SnapshotRules(cfg *HardFilterConfig) []Rule {
	var rules []Rule

	// 1. 排除 ST/退市（正则默认 ST|退）
	if cfg.ExcludeST {
		pattern := cfg.STPattern
		if pattern == "" {
			pattern = `ST|退`
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			re = regexp.MustCompile(`ST|退`)
		}
		rules = append(rules, Rule{
			Name: "exclude_st", Desc: "排除ST/退市股",
			Reject: func(in *FilterInput) string {
				if re.MatchString(in.Name) {
					return fmt.Sprintf("名称 %q 命中 ST/退市模式 %q", in.Name, re.String())
				}
				return ""
			},
		})
	}

	// 2-9. 数值范围规则
	rules = append(rules,
		Rule{Name: "amount_range", Desc: "成交额范围", Reject: func(in *FilterInput) string {
			return rangeReject("成交额", in.Amount, cfg.MinAmount, cfg.MaxAmount)
		}},
		Rule{Name: "price_range", Desc: "价格范围", Reject: func(in *FilterInput) string {
			return rangeReject("价格", in.Price, cfg.MinPrice, cfg.MaxPrice)
		}},
		Rule{Name: "mv_range", Desc: "总市值范围", Reject: func(in *FilterInput) string {
			return rangeReject("总市值", in.TotalMV, cfg.MinTotalMV, cfg.MaxTotalMV)
		}},
		Rule{Name: "pe_range", Desc: "PE范围", Reject: func(in *FilterInput) string {
			return rangeReject("PE", in.PE, cfg.MinPE, cfg.MaxPE)
		}},
		Rule{Name: "pb_range", Desc: "PB范围", Reject: func(in *FilterInput) string {
			return rangeReject("PB", in.PB, cfg.MinPB, cfg.MaxPB)
		}},
		Rule{Name: "volume_ratio_min", Desc: "量比下限", Reject: func(in *FilterInput) string {
			if cfg.MinVolumeRatio > 0 && in.VolumeRatio < cfg.MinVolumeRatio {
				return fmt.Sprintf("量比 %.2f 低于下限 %.2f", in.VolumeRatio, cfg.MinVolumeRatio)
			}
			return ""
		}},
		Rule{Name: "turnover_min", Desc: "换手率下限", Reject: func(in *FilterInput) string {
			if cfg.MinTurnoverRate > 0 && in.TurnoverRate < cfg.MinTurnoverRate {
				return fmt.Sprintf("换手率 %.2f%% 低于下限 %.2f%%", in.TurnoverRate, cfg.MinTurnoverRate)
			}
			return ""
		}},
		Rule{Name: "change_pct_range", Desc: "涨跌幅范围", Reject: func(in *FilterInput) string {
			if cfg.MinChangePct != 0 && in.ChangePercent < cfg.MinChangePct {
				return fmt.Sprintf("涨跌幅 %.2f%% 低于下限 %.2f%%", in.ChangePercent, cfg.MinChangePct)
			}
			if cfg.MaxChangePct != 0 && in.ChangePercent > cfg.MaxChangePct {
				return fmt.Sprintf("涨跌幅 %.2f%% 高于上限 %.2f%%", in.ChangePercent, cfg.MaxChangePct)
			}
			return ""
		}},
	)
	return rules
}

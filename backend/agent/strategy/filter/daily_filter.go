package filter

import "fmt"

// 日线级过滤（方案 §8.1 D7：15 类规则，23 参数——文档列出的是类别，
// 此处按类别展开为具体配置项，见 config.go 日线级区块）。

// DailyRules 构建日线级规则集。无日线数据（HasDailyData=false）时：
// RejectMissingDaily=true 直接淘汰，否则跳过全部日线规则。
func DailyRules(cfg *HardFilterConfig) []Rule {
	// guard 包装：无日线数据时按配置处理
	guard := func(reject func(in *FilterInput) string) func(in *FilterInput) string {
		return func(in *FilterInput) string {
			if !in.HasDailyData {
				if cfg.RejectMissingDaily {
					return "无日线数据"
				}
				return ""
			}
			return reject(in)
		}
	}

	return []Rule{
		// 1. 60 日涨跌幅范围（2 参数）
		{Name: "change_pct60_range", Desc: "60日涨跌幅范围", Reject: guard(func(in *FilterInput) string {
			if cfg.MinChangePct60 != 0 && in.ChangePct60 < cfg.MinChangePct60 {
				return fmt.Sprintf("60日涨跌幅 %.1f%% 低于下限 %.1f%%", in.ChangePct60, cfg.MinChangePct60)
			}
			if cfg.MaxChangePct60 != 0 && in.ChangePct60 > cfg.MaxChangePct60 {
				return fmt.Sprintf("60日涨跌幅 %.1f%% 高于上限 %.1f%%", in.ChangePct60, cfg.MaxChangePct60)
			}
			return ""
		})},
		// 2. MA 多头排列（1 参数）
		{Name: "ma_bull_align", Desc: "MA多头排列", Reject: guard(func(in *FilterInput) string {
			if cfg.RequireMABullAlign && !in.MABullAlign {
				return "均线非多头排列"
			}
			return ""
		})},
		// 3. 价格站上 MA20（1 参数）
		{Name: "above_ma20", Desc: "价格站上MA20", Reject: guard(func(in *FilterInput) string {
			if cfg.RequireAboveMA20 && !in.AboveMA20 {
				return "价格未站上MA20"
			}
			return ""
		})},
		// 4. 信号分下限（1 参数）
		{Name: "signal_score_min", Desc: "信号分下限", Reject: guard(func(in *FilterInput) string {
			if cfg.MinSignalScore > 0 && in.SignalScore < cfg.MinSignalScore {
				return fmt.Sprintf("信号分 %.0f 低于下限 %.0f", in.SignalScore, cfg.MinSignalScore)
			}
			return ""
		})},
		// 5. MACD 状态白名单（1 参数）
		{Name: "macd_whitelist", Desc: "MACD状态白名单", Reject: guard(func(in *FilterInput) string {
			if !inWhitelist(cfg.MACDWhitelist, in.MACDState) {
				return fmt.Sprintf("MACD状态 %q 不在白名单 %v", in.MACDState, cfg.MACDWhitelist)
			}
			return ""
		})},
		// 6. RSI 状态白名单（1 参数）
		{Name: "rsi_whitelist", Desc: "RSI状态白名单", Reject: guard(func(in *FilterInput) string {
			if !inWhitelist(cfg.RSIWhitelist, in.RSIState) {
				return fmt.Sprintf("RSI状态 %q 不在白名单 %v", in.RSIState, cfg.RSIWhitelist)
			}
			return ""
		})},
		// 7. 20 日突破幅度范围（2 参数）
		{Name: "breakout_range", Desc: "20日突破幅度范围", Reject: guard(func(in *FilterInput) string {
			if cfg.MinBreakoutPct != 0 && in.Breakout20dPct < cfg.MinBreakoutPct {
				return fmt.Sprintf("突破幅度 %.1f%% 低于下限 %.1f%%", in.Breakout20dPct, cfg.MinBreakoutPct)
			}
			if cfg.MaxBreakoutPct > 0 && in.Breakout20dPct > cfg.MaxBreakoutPct {
				return fmt.Sprintf("突破幅度 %.1f%% 高于上限 %.1f%%", in.Breakout20dPct, cfg.MaxBreakoutPct)
			}
			return ""
		})},
		// 8. 振幅范围（2 参数）
		{Name: "amplitude_range", Desc: "振幅范围", Reject: guard(func(in *FilterInput) string {
			if cfg.MinAmplitudePct > 0 && in.AmplitudePct < cfg.MinAmplitudePct {
				return fmt.Sprintf("振幅 %.1f%% 低于下限 %.1f%%", in.AmplitudePct, cfg.MinAmplitudePct)
			}
			if cfg.MaxAmplitudePct > 0 && in.AmplitudePct > cfg.MaxAmplitudePct {
				return fmt.Sprintf("振幅 %.1f%% 高于上限 %.1f%%", in.AmplitudePct, cfg.MaxAmplitudePct)
			}
			return ""
		})},
		// 9. 20 日量比范围（2 参数）
		{Name: "volume_ratio20d_range", Desc: "20日量比范围", Reject: guard(func(in *FilterInput) string {
			if cfg.MinVolumeRatio20d > 0 && in.VolumeRatio20d < cfg.MinVolumeRatio20d {
				return fmt.Sprintf("20日量比 %.2f 低于下限 %.2f", in.VolumeRatio20d, cfg.MinVolumeRatio20d)
			}
			if cfg.MaxVolumeRatio20d > 0 && in.VolumeRatio20d > cfg.MaxVolumeRatio20d {
				return fmt.Sprintf("20日量比 %.2f 高于上限 %.2f", in.VolumeRatio20d, cfg.MaxVolumeRatio20d)
			}
			return ""
		})},
		// 10. K 线实体比例下限（1 参数）
		{Name: "body_pct_min", Desc: "K线实体比例下限", Reject: guard(func(in *FilterInput) string {
			if cfg.MinBodyPct > 0 && in.BodyPct < cfg.MinBodyPct {
				return fmt.Sprintf("K线实体比例 %.2f 低于下限 %.2f", in.BodyPct, cfg.MinBodyPct)
			}
			return ""
		})},
		// 11. 回踩 MA20 幅度范围（2 参数）
		{Name: "ma20_deviation_range", Desc: "回踩MA20幅度范围", Reject: guard(func(in *FilterInput) string {
			if cfg.MinMA20DevPct != 0 && in.MA20DeviationPct < cfg.MinMA20DevPct {
				return fmt.Sprintf("MA20偏离 %.1f%% 低于下限 %.1f%%", in.MA20DeviationPct, cfg.MinMA20DevPct)
			}
			if cfg.MaxMA20DevPct != 0 && in.MA20DeviationPct > cfg.MaxMA20DevPct {
				return fmt.Sprintf("MA20偏离 %.1f%% 高于上限 %.1f%%", in.MA20DeviationPct, cfg.MaxMA20DevPct)
			}
			return ""
		})},
		// 12. 盘整天数范围（2 参数）
		{Name: "consolidation_days_range", Desc: "盘整天数范围", Reject: guard(func(in *FilterInput) string {
			if cfg.MinConsolidationDays > 0 && in.ConsolidationDays < cfg.MinConsolidationDays {
				return fmt.Sprintf("盘整 %d 天少于下限 %d 天", in.ConsolidationDays, cfg.MinConsolidationDays)
			}
			if cfg.MaxConsolidationDays > 0 && in.ConsolidationDays > cfg.MaxConsolidationDays {
				return fmt.Sprintf("盘整 %d 天多于上限 %d 天", in.ConsolidationDays, cfg.MaxConsolidationDays)
			}
			return ""
		})},
		// 13. 波动率上限（1 参数）
		{Name: "volatility_max", Desc: "波动率上限", Reject: guard(func(in *FilterInput) string {
			if cfg.MaxVolatilityPct > 0 && in.VolatilityPct > cfg.MaxVolatilityPct {
				return fmt.Sprintf("波动率 %.1f%% 高于上限 %.1f%%", in.VolatilityPct, cfg.MaxVolatilityPct)
			}
			return ""
		})},
		// 14. 最大回撤下限（1 参数）
		{Name: "drawdown_min", Desc: "最大回撤下限", Reject: guard(func(in *FilterInput) string {
			if cfg.MinDrawdownPct != 0 && in.MaxDrawdownPct < cfg.MinDrawdownPct {
				return fmt.Sprintf("最大回撤 %.1f%% 深于下限 %.1f%%", in.MaxDrawdownPct, cfg.MinDrawdownPct)
			}
			return ""
		})},
		// 15. ATR 上限（1 参数）
		{Name: "atr_max", Desc: "ATR上限", Reject: guard(func(in *FilterInput) string {
			if cfg.MaxATRPct > 0 && in.ATRPct > cfg.MaxATRPct {
				return fmt.Sprintf("ATR/价格 %.1f%% 高于上限 %.1f%%", in.ATRPct, cfg.MaxATRPct)
			}
			return ""
		})},
	}
}

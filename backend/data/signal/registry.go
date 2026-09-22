package signal

// registry.go 30 个信号的注册表：定义 + 判定函数。
// 阈值为经验值常量（见下方 const），形态定义参考经典K线理论。
// key 与形态选股页（models.TechnicalIndicators）的条件 key 保持一致。

// 判定阈值常量
const (
	bigBodyPct       = 4.0  // 大阳线/大阴线实体阈值（%）
	midBodyPct       = 3.0  // 中阳/中阴实体阈值（连续大阳线等使用）
	starBodyRatio    = 0.3  // 星线实体占振幅上限
	longShadowRatio  = 2.0  // 锤头/射击之星：长影线 ≥ 实体倍数
	narrowRangePct   = 5.0  // 窄幅整理：近7日振幅上限（%）
	volBreakRatio    = 1.8  // 放量突破：量 ≥ 20日均量倍数
	volAttackRatio   = 2.0  // 放量上攻：量 ≥ 5日均量倍数
	volShrinkRatio   = 0.7  // 下跌无量：量 ≤ 20日均量倍数
	volHeavenRatio   = 3.0  // 天量：量 ≥ 20日均量倍数
	lowPosThreshold  = 0.3  // 低位：价格处于60日区间下30%
	highPosThreshold = 0.7  // 高位：价格处于60日区间上30%
)

var registry = []SignalDef{
	// ---- 指标信号 ----
	{
		Key: "MACD_GOLDEN_FORK", Name: "MACD金叉", Category: CatIndicator, Direction: Bullish,
		Tip: "快线DIF上穿慢线DEA，短期动能转强，常见买入信号；零轴上方的金叉强度更高",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 1 && c.dif[i-1] <= c.dea[i-1] && c.dif[i] > c.dea[i]
		},
	},
	{
		Key: "KDJ_GOLDEN_FORK", Name: "KDJ金叉", Category: CatIndicator, Direction: Bullish,
		Tip: "K线上穿D线，短线反弹信号；在20以下超卖区发生的金叉可靠性更高",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 1 && c.k[i-1] <= c.d[i-1] && c.k[i] > c.d[i]
		},
	},
	{
		Key: "BREAKUP_MA_5DAYS", Name: "向上突破5日均线", Category: CatIndicator, Direction: Bullish,
		Tip: "收盘价站上5日均线，短线转强的初步信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 1 && c.ma5[i] > 0 && c.ma5[i-1] > 0 &&
				c.close[i-1] <= c.ma5[i-1] && c.close[i] > c.ma5[i]
		},
	},
	{
		Key: "LONG_AVG_ARRAY", Name: "均线多头排列", Category: CatIndicator, Direction: Bullish,
		Tip: "短/中/长期均线自上而下依次排列且向上发散，上升趋势的典型结构",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 1 || c.ma20[i] == 0 || c.ma20[i-1] == 0 {
				return false
			}
			return c.ma5[i] > c.ma10[i] && c.ma10[i] > c.ma20[i] && c.ma5[i] > c.ma5[i-1]
		},
	},
	{
		Key: "SHORT_AVG_ARRAY", Name: "均线空头排列", Category: CatIndicator, Direction: Bearish,
		Tip: "短/中/长期均线自下而上依次排列且向下发散，下降趋势，回避为主",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 1 || c.ma20[i] == 0 || c.ma20[i-1] == 0 {
				return false
			}
			return c.ma5[i] < c.ma10[i] && c.ma10[i] < c.ma20[i] && c.ma5[i] < c.ma5[i-1]
		},
	},

	// ---- K线形态 ----
	{
		Key: "ONE_DAYANG_LINE", Name: "一根大阳线", Category: CatPattern, Direction: Bullish,
		Tip:      "单日收出实体明显的大阳线，买方力量强劲",
		detect:   func(c *evalCtx) bool { return c.isBigYang(c.idx) },
	},
	{
		Key: "TWO_DAYANG_LINES", Name: "两根大阳线", Category: CatPattern, Direction: Bullish,
		Tip: "连续两根大阳线，强势上攻形态",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 1 && c.bullish(i) && c.bullish(i-1) &&
				c.bodyPct(i) >= midBodyPct && c.bodyPct(i-1) >= midBodyPct
		},
	},
	{
		Key: "UPPER_4DAYS", Name: "四串阳", Category: CatPattern, Direction: Bullish,
		Tip:      "连续4根阳线稳步推升，多头持续占优",
		detect:   func(c *evalCtx) bool { return c.consecutiveUp(c.idx) >= 4 },
	},
	{
		Key: "UPPER_8DAYS", Name: "八仙过海(八连阳)", Category: CatPattern, Direction: Bullish,
		Tip:      "连续8日收阳，极端强势；高位出现时也需防物极必反",
		detect:   func(c *evalCtx) bool { return c.consecutiveUp(c.idx) >= 8 },
	},
	{
		Key: "UPPER_9DAYS", Name: "九阳神功(九连阳)", Category: CatPattern, Direction: Bullish,
		Tip:      "连续9日收阳的极端强势形态，注意高位滞涨风险",
		detect:   func(c *evalCtx) bool { return c.consecutiveUp(c.idx) >= 9 },
	},
	{
		Key: "DOWN_7DAYS", Name: "七仙女下凡(七连阴)", Category: CatPattern, Direction: Bearish,
		Tip:      "连续7日收阴，严重超卖，可关注反弹机会",
		detect:   func(c *evalCtx) bool { return c.consecutiveDown(c.idx) >= 7 },
	},
	{
		Key: "RISE_SUN", Name: "旭日东升", Category: CatPattern, Direction: Bullish,
		Tip: "下跌中阴线之后高开高走收大阳线，收盘超过前阴开盘价，见底反转信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 1 && c.isBigYin(i-1) && c.isBigYang(i) &&
				c.open[i] > c.close[i-1] && c.close[i] > c.open[i-1]
		},
	},
	{
		Key: "POWER_FULGUN", Name: "强势多方炮", Category: CatPattern, Direction: Bullish,
		Tip: "两阳夹一阴：阳线实体大、阴线实体小，上涨中继的强势信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 2 {
				return false
			}
			// 两阳夹一阴：阴线实体小于两侧阳线，且收盘收于首阳收盘上方
			return c.isBigYang(i-2) && c.bearish(i-1) && c.bullish(i) &&
				c.absBody(i-1) < c.absBody(i-2)*0.6 &&
				c.bodyPct(i) >= midBodyPct && c.close[i] > c.close[i-2]
		},
	},
	{
		Key: "RESTORE_JUSTICE", Name: "拨云见日", Category: CatPattern, Direction: Bullish,
		Tip: "连续下跌后出现带量长阳收复失地，底部反转形态",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 4 {
				return false
			}
			// 近3日处于下跌/低位，今日带量长阳收复近3日最高收盘
			return c.consecutiveDown(i-1) >= 2 && c.isBigYang(i) &&
				c.vol[i] > avgVol(c.vol, i-1, 5)*1.5 &&
				c.close[i] > maxClose(c.close, i-3, i-1)
		},
	},
	{
		Key: "MORNING_STAR", Name: "早晨之星", Category: CatPattern, Direction: Bullish,
		Tip: "大阴线→星线→大阳线的三根组合，经典底部反转信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 2 {
				return false
			}
			mid := (c.open[i-2] + c.close[i-2]) / 2 // 首阴实体中点
			return c.isBigYin(i-2) && c.isStar(i-1) && c.isBigYang(i) && c.close[i] > mid
		},
	},
	{
		Key: "FIRST_DAWN", Name: "曙光初现", Category: CatPattern, Direction: Bullish,
		Tip: "大阴线后的大阳线深入前阴线实体一半以上，见底信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 1 {
				return false
			}
			mid := (c.open[i-1] + c.close[i-1]) / 2 // 前阴实体中点
			return c.isBigYin(i-1) && c.bullish(i) && c.bodyPct(i) >= midBodyPct &&
				c.open[i] < c.close[i-1] && c.close[i] > mid && c.close[i] < c.open[i-1]
		},
	},
	{
		Key: "REVERSING_HAMMER", Name: "倒转锤头", Category: CatPattern, Direction: Bullish,
		Tip: "下跌末端出现上影线长、实体小的K线，试探性反攻信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 3 {
				return false
			}
			return c.consecutiveDown(i-1) >= 2 &&
				c.upperShadow(i) >= c.absBody(i)*longShadowRatio &&
				c.lowerShadow(i) <= c.absBody(i)*0.5 &&
				c.candleRange(i) > 0
		},
	},
	{
		Key: "PREGNANT", Name: "身怀六甲", Category: CatPattern, Direction: Warning,
		Tip: "大K线后紧跟的小K线实体被完全包含，趋势减速/反转预警",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 1 {
				return false
			}
			prevTop := maxOf(c.open[i-1], c.close[i-1])
			prevBot := minOf(c.open[i-1], c.close[i-1])
			curTop := maxOf(c.open[i], c.close[i])
			curBot := minOf(c.open[i], c.close[i])
			return c.absBody(i-1) > c.candleRange(i-1)*0.6 && // 前一根为大实体
				curTop < prevTop && curBot > prevBot // 今日实体被完全包含
		},
	},
	{
		Key: "NARROW_FINISH", Name: "窄幅整理", Category: CatPattern, Direction: Neutral,
		Tip: "股价在小范围横盘收敛、波动率压缩，常预示即将选择方向（变盘）",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 6 {
				return false
			}
			hi := maxHigh(c.high, i-6, i)
			lo := minLow(c.low, i-6, i)
			if lo <= 0 {
				return false
			}
			return (hi-lo)/lo*100 <= narrowRangePct
		},
	},
	{
		Key: "BLACK_CLOUD_TOPS", Name: "乌云盖顶", Category: CatPattern, Direction: Bearish,
		Tip: "大阳线后高开低走的大阴线深入阳线实体，经典顶部反转信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 1 {
				return false
			}
			mid := (c.open[i-1] + c.close[i-1]) / 2 // 前阳实体中点
			return c.isBigYang(i-1) && c.bearish(i) &&
				c.open[i] > c.close[i-1] && // 高开
				c.close[i] < mid && c.close[i] > c.open[i-1] // 深入实体但未到开盘
		},
	},
	{
		Key: "EVENING_STAR", Name: "黄昏之星", Category: CatPattern, Direction: Bearish,
		Tip: "大阳线→星线→大阴线的三根组合，顶部反转信号",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 2 {
				return false
			}
			mid := (c.open[i-2] + c.close[i-2]) / 2 // 首阳实体中点
			return c.isBigYang(i-2) && c.isStar(i-1) && c.isBigYin(i) && c.close[i] < mid
		},
	},
	{
		Key: "SHOOTING_STAR", Name: "射击之星", Category: CatPattern, Direction: Bearish,
		Tip: "上涨末端出现上影线长、实体小的K线，冲高回落的见顶预警",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 3 {
				return false
			}
			return c.consecutiveUp(i-1) >= 2 &&
				c.upperShadow(i) >= c.absBody(i)*longShadowRatio &&
				c.lowerShadow(i) <= c.absBody(i)*0.5 &&
				c.candleRange(i) > 0
		},
	},
	{
		Key: "BEARISH_ENGULFING", Name: "穿头破脚", Category: CatPattern, Direction: Bearish,
		Tip: "大阴线完全吞没前一根阳线实体，见顶反转信号（看跌）",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 1 && c.bullish(i-1) && c.bearish(i) &&
				c.open[i] >= c.close[i-1] && c.close[i] <= c.open[i-1] &&
				c.absBody(i) > c.absBody(i-1)
		},
	},

	// ---- 量价 ----
	{
		Key: "BREAK_THROUGH", Name: "放量突破", Category: CatVolume, Direction: Bullish,
		Tip: "成交量显著放大且价格突破关键阻力（均线/前高），资金涌入，突破有效性高",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 20 {
				return false
			}
			avg20 := avgVol(c.vol, i-1, 20)
			return avg20 > 0 && c.vol[i] >= avg20*volBreakRatio &&
				c.close[i] > maxClose(c.close, i-20, i-1)
		},
	},
	{
		Key: "UPPER_LARGE_VOLUME", Name: "连涨放量", Category: CatVolume, Direction: Bullish,
		Tip: "连续上涨且成交量持续放大，上涨有资金支撑，趋势健康",
		detect: func(c *evalCtx) bool {
			i := c.idx
			return i >= 2 && c.consecutiveUp(i) >= 3 &&
				c.vol[i] > c.vol[i-1] && c.vol[i-1] > c.vol[i-2]
		},
	},
	{
		Key: "UPSIDE_VOLUME", Name: "放量上攻", Category: CatVolume, Direction: Bullish,
		Tip: "价涨量增，真金白银推动的上涨，动能充足",
		detect: func(c *evalCtx) bool {
			i := c.idx
			avg5 := avgVol(c.vol, i-1, 5)
			return i >= 1 && avg5 > 0 &&
				c.changePct(i) >= 2.0 && c.vol[i] >= avg5*volAttackRatio
		},
	},
	{
		Key: "DOWN_NARROW_VOLUME", Name: "下跌无量", Category: CatVolume, Direction: Neutral,
		Tip: "下跌过程中成交量萎缩，抛压衰竭，接近阶段底部的特征",
		detect: func(c *evalCtx) bool {
			i := c.idx
			avg20 := avgVol(c.vol, i-1, 20)
			return i >= 3 && avg20 > 0 && c.consecutiveDown(i) >= 3 &&
				c.vol[i] <= avg20*volShrinkRatio && c.vol[i-1] <= avg20*volShrinkRatio
		},
	},
	{
		Key: "HEAVEN_RULE", Name: "天量法则", Category: CatVolume, Direction: Warning,
		Tip: "出现阶段天量成交、剧烈换手，常预示短期拐点（可能是突破也可能是顶部）",
		detect: func(c *evalCtx) bool {
			i := c.idx
			if i < 20 {
				return false
			}
			avg20 := avgVol(c.vol, i-1, 20)
			return avg20 > 0 && c.vol[i] >= avg20*volHeavenRatio
		},
	},

	// ---- 资金流（NeedFunds，Provider 未注入/失败时跳过） ----
	{
		Key: "LOW_FUNDS_INFLOW", Name: "低位资金净流入", Category: CatFunds, Direction: Bullish,
		NeedFunds: true,
		Tip:       "股价处于阶段低位但主力资金持续流入，疑似主力吸筹",
		detect: func(c *evalCtx) bool {
			return c.fundsOK && c.fundsInflow > 0 && c.pricePosition(c.idx, 60) <= lowPosThreshold
		},
	},
	{
		Key: "HIGH_FUNDS_OUTFLOW", Name: "高位资金净流出", Category: CatFunds, Direction: Bearish,
		NeedFunds: true,
		Tip:       "股价处于阶段高位但主力资金持续流出，警惕主力出货",
		detect: func(c *evalCtx) bool {
			return c.fundsOK && c.fundsInflow < 0 && c.pricePosition(c.idx, 60) >= highPosThreshold
		},
	},
}

func maxOf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minOf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

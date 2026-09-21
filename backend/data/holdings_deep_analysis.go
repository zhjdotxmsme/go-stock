package data

// 持仓深度分析：本地聚合单只持仓的多维数据包（直白关键价位 + 技术指标 +
// 资金流 + 板块 + 新闻 + 历史AI推荐），供前端即时展示与 AI 综合分析引用。
// 借鉴开源 daily_stock_analysis 的多维日报思路：技术面点位直白化，
// 资金面/消息面/板块面分项采集，最后交由 LLM 汇总结论。

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// PriceLevel 一条直白价位（含依据标签）
type PriceLevel struct {
	Price float64 `json:"price"`
	Label string  `json:"label"`
}

// KeyPriceLevels 直白关键价位：压力/支撑/买点/卖点
type KeyPriceLevels struct {
	Current    float64      `json:"current"`
	Resistance []PriceLevel `json:"resistance"` // 压力位（由近到远）
	Support    []PriceLevel `json:"support"`    // 支撑位（由近到远）
	BuyPoints  []PriceLevel `json:"buyPoints"`  // 买入参考
	SellPoints []PriceLevel `json:"sellPoints"` // 卖出/止损参考
}

// HoldingsDeepStock 单只持仓的深度分析数据包
type HoldingsDeepStock struct {
	StockCode     string                     `json:"stockCode"`
	StockName     string                     `json:"stockName"`
	CurrentPrice  float64                    `json:"currentPrice"`
	ChangePercent float64                    `json:"changePercent"`
	PositionPct   float64                    `json:"positionPct"`   // 占组合市值比例 %
	ProfitPercent float64                    `json:"profitPercent"` // 持仓收益率 %
	Indicators    *IndicatorSummary          `json:"indicators"`
	Levels        *KeyPriceLevels            `json:"levels"`
	CapitalFlow   []models.StockMoneyDataHis `json:"capitalFlow"`   // 近10日资金流
	CapitalSummary string                    `json:"capitalSummary"` // 资金面一句话
	Industry      string          `json:"industry"`
	IndustryFlow  string          `json:"industryFlow"` // 板块资金面一句话
	News          []SectorNewsItem `json:"news"`        // 最近新闻（最多5条）
	AiAdvice      *TradingAiAdvice `json:"aiAdvice"`    // 最近一次AI推荐
	Kronos        *KronosPrediction `json:"kronos"`     // Kronos 未来K线预测（可选，未开启/失败为 nil）
}

// dedupeLevels 按价格去重：相差 0.8% 以内的价位合并（标签用 " / " 连接），排序后各取前 limit 条。
// below=false 保留 >= current（压力），below=true 保留 <= current（支撑）。
func dedupeLevels(levels []PriceLevel, current float64, below bool, limit int) []PriceLevel {
	const mergePct = 0.008
	sort.Slice(levels, func(i, j int) bool {
		if below {
			return levels[i].Price > levels[j].Price // 支撑：近的（更高）在前
		}
		return levels[i].Price < levels[j].Price // 压力：近的（更低）在前
	})
	var merged []PriceLevel
	for _, l := range levels {
		if l.Price <= 0 {
			continue
		}
		if below && l.Price > current {
			continue
		}
		if !below && l.Price < current {
			continue
		}
		hit := false
		for i := range merged {
			base := merged[i].Price
			if base > 0 && abs64(l.Price-base)/base <= mergePct {
				if !strings.Contains(merged[i].Label, l.Label) {
					merged[i].Label += " / " + l.Label
				}
				hit = true
				break
			}
		}
		if !hit {
			merged = append(merged, l)
		}
	}
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// roundPrice 按价格量级取 2 位小数
func roundPrice(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// nearestIntegerLevel 向上取最近的整数关口（步长随价格量级：≥100 取 10，≥20 取 5，否则取 1）
func nearestIntegerLevel(price float64) (float64, float64) {
	step := 1.0
	if price >= 100 {
		step = 10
	} else if price >= 20 {
		step = 5
	}
	below := float64(int64(price/step)) * step
	return below, below + step
}

// computeKeyPriceLevels 基于日K线与技术指标计算直白关键价位。
// bars 升序；ind 为同源指标结果（MA/BOLL/ATR/GAP）。
func computeKeyPriceLevels(bars []datasource.KLineBar, ind *IndicatorResult) *KeyPriceLevels {
	n := len(bars)
	levels := &KeyPriceLevels{}
	if n == 0 {
		return levels
	}
	current := bars[n-1].Close
	levels.Current = roundPrice(current)

	high := make([]float64, n)
	low := make([]float64, n)
	for i, b := range bars {
		high[i] = b.High
		low[i] = b.Low
	}
	high20 := highestIn(high, 20)
	high60 := highestIn(high, 60)
	low20 := lowestIn(low, 20)

	var resistance, support []PriceLevel
	// 压力候选
	resistance = append(resistance, PriceLevel{Price: high20, Label: "20日高点"})
	if high60 > high20 {
		resistance = append(resistance, PriceLevel{Price: high60, Label: "60日高点"})
	}
	if ind != nil {
		if up := ind.BOLL["Up"]; up > 0 {
			resistance = append(resistance, PriceLevel{Price: up, Label: "布林上轨"})
		}
		if ind.GAP != nil {
			for _, g := range ind.GAP.Recent {
				if !g.Filled && g.Direction == "down" {
					resistance = append(resistance, PriceLevel{Price: g.GapHigh, Label: fmt.Sprintf("未回补向下缺口上沿(%s)", g.Date)})
					break
				}
			}
		}
	}
	intAbove, _ := nearestIntegerLevel(current)
	resistance = append(resistance, PriceLevel{Price: intAbove, Label: "整数关口"})

	// 支撑候选
	if ind != nil {
		if ma20 := ind.MA["MA20"]; ma20 > 0 {
			support = append(support, PriceLevel{Price: ma20, Label: "20日均线"})
		}
		if ma60 := ind.MA["MA60"]; ma60 > 0 {
			support = append(support, PriceLevel{Price: ma60, Label: "60日均线"})
		}
		if mid := ind.BOLL["Mid"]; mid > 0 {
			support = append(support, PriceLevel{Price: mid, Label: "布林中轨"})
		}
		if ind.GAP != nil {
			for _, g := range ind.GAP.Recent {
				if !g.Filled && g.Direction == "up" {
					support = append(support, PriceLevel{Price: g.GapHigh, Label: fmt.Sprintf("未回补缺口上沿(%s)", g.Date)})
					break
				}
			}
		}
	}
	support = append(support, PriceLevel{Price: low20, Label: "20日低点"})

	levels.Resistance = dedupeLevels(resistance, current, false, 3)
	levels.Support = dedupeLevels(support, current, true, 3)

	// 买点参考
	var buys []PriceLevel
	if len(levels.Support) > 0 {
		s1 := levels.Support[0]
		if abs64(s1.Price-current)/current <= 0.06 {
			buys = append(buys, PriceLevel{Price: roundPrice(s1.Price * 1.005), Label: "回踩" + s1.Label + "企稳"})
		}
	}
	if ind != nil && ind.GAP != nil {
		for _, g := range ind.GAP.Recent {
			if !g.Filled && g.Direction == "up" && g.GapHigh < current && (current-g.GapHigh)/current <= 0.08 {
				buys = append(buys, PriceLevel{Price: g.GapHigh, Label: "缺口上沿回踩(" + g.Date + ")"})
				break
			}
		}
	}
	if high20 > current {
		buys = append(buys, PriceLevel{Price: roundPrice(high20 * 1.005), Label: "放量突破20日高点确认"})
	}
	levels.BuyPoints = buys

	// 卖出/止损参考
	var sells []PriceLevel
	if len(levels.Resistance) > 0 {
		r1 := levels.Resistance[0]
		sells = append(sells, PriceLevel{Price: roundPrice(r1.Price * 0.995), Label: "接近" + r1.Label + "逢高减仓"})
	}
	if ind != nil && ind.ATR > 0 {
		sells = append(sells, PriceLevel{Price: roundPrice(current - 2*ind.ATR), Label: "ATR止损位(2×ATR)"})
	}
	if len(levels.Support) > 0 {
		s1 := levels.Support[0]
		sells = append(sells, PriceLevel{Price: roundPrice(s1.Price * 0.98), Label: "跌破" + s1.Label + "止损"})
	}
	levels.SellPoints = sells
	return levels
}

func highestIn(values []float64, period int) float64 {
	start := 0
	if len(values) > period {
		start = len(values) - period
	}
	h := values[start]
	for _, v := range values[start+1:] {
		if v > h {
			h = v
		}
	}
	return h
}

func lowestIn(values []float64, period int) float64 {
	start := 0
	if len(values) > period {
		start = len(values) - period
	}
	l := values[start]
	for _, v := range values[start+1:] {
		if v < l {
			l = v
		}
	}
	return l
}

// sumMainNetFlow 近 days 日主力净额合计（F62 元）
func sumMainNetFlow(rows []models.StockMoneyDataHis, days int) float64 {
	sum := 0.0
	for i := 0; i < len(rows) && i < days; i++ {
		sum += parsePrice(strings.ReplaceAll(rows[i].F62, ",", ""))
	}
	return sum
}

// consecutiveFlowDays 主力资金连续流入/流出天数（从最近一天往前数同号天数）
func consecutiveFlowDays(rows []models.StockMoneyDataHis) (days int, inflow bool) {
	for i := 0; i < len(rows); i++ {
		v := parsePrice(strings.ReplaceAll(rows[i].F62, ",", ""))
		if i == 0 {
			if v == 0 {
				return 0, false
			}
			inflow = v > 0
			days = 1
			continue
		}
		if (v > 0) == inflow && v != 0 {
			days++
		} else {
			break
		}
	}
	return days, inflow
}

// capitalFlowSummary 资金面一句话
func capitalFlowSummary(rows []models.StockMoneyDataHis) string {
	if len(rows) == 0 {
		return "暂无资金流数据"
	}
	sum5 := sumMainNetFlow(rows, 5)
	days, inflow := consecutiveFlowDays(rows)
	dir := "流出"
	if inflow {
		dir = "流入"
	}
	text := fmt.Sprintf("近5日主力净额合计 %.2f亿元", sum5/1e8)
	if days >= 2 {
		text += fmt.Sprintf("，主力连续%d日净%s", days, dir)
	}
	return text
}

// industryFlowSummary 查询该行业最新一次板块主力净流入快照
func industryFlowSummary(industry string) string {
	industry = strings.TrimSpace(industry)
	if industry == "" {
		return ""
	}
	var latest models.BKFundFlow
	err := db.Dao.Model(&models.BKFundFlow{}).
		Where("name = ?", industry).
		Order("snap_time DESC").First(&latest).Error
	if err != nil {
		return ""
	}
	snapDate := latest.SnapTime
	if len(snapDate) >= 10 {
		snapDate = snapDate[:10]
	}
	direction := "净流入"
	if latest.NetInflow < 0 {
		direction = "净流出"
	}
	return fmt.Sprintf("%s板块最新主力%s %.2f亿元（%s快照）", industry, direction, abs64(float64(latest.NetInflow))/1e8, snapDate)
}

// GetHoldingsDeepData 聚合全部持仓的深度分析数据包（本地数据，不含 AI）。
// positions 由调用方（trading service FIFO 口径）传入；单只失败不影响整体。
func GetHoldingsDeepData(ctx context.Context, positions []HoldingsPosition) ([]*HoldingsDeepStock, error) {
	if len(positions) == 0 {
		return []*HoldingsDeepStock{}, nil
	}
	totalMV := 0.0
	for _, p := range positions {
		totalMV += p.MarketValue
	}

	result := make([]*HoldingsDeepStock, 0, len(positions))
	for _, p := range positions {
		item := &HoldingsDeepStock{
			StockCode:     p.StockCode,
			StockName:     p.StockName,
			CurrentPrice:  p.CurrentPrice,
			ProfitPercent: p.ProfitPercent,
		}
		if totalMV > 0 {
			item.PositionPct = p.MarketValue / totalMV * 100
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("持仓深度数据 %s panic: %v", p.StockCode, r)
				}
			}()

			apiCode := normalizeTradingRecordAPI(p.StockCode)
			kline, kerr := datasource.GetRouter().GetKLine(ctx, apiCode, "101", 120)
			if kerr == nil && kline != nil && len(kline.Bars) >= 5 {
				ind, ierr := GetTechnicalIndicators(ctx, apiCode, "101", 120)
				if ierr == nil {
					item.Indicators = GetIndicatorSummary(ind)
					item.Levels = computeKeyPriceLevels(kline.Bars, ind)
				}
				last := kline.Bars[len(kline.Bars)-1]
				prev := kline.Bars[len(kline.Bars)-2]
				if item.CurrentPrice <= 0 {
					item.CurrentPrice = last.Close
				}
				if prev.Close > 0 {
					item.ChangePercent = (last.Close - prev.Close) / prev.Close * 100
				}
			}
			if item.Levels == nil {
				item.Levels = &KeyPriceLevels{}
			}
			if item.Indicators == nil {
				item.Indicators = &IndicatorSummary{Trend: "数据不足"}
			}

			// 资金面（近10日）
			money := NewStockDataApi().GetStockHistoryMoneyData(apiCode)
			if len(money) > 10 {
				money = money[:10]
			}
			item.CapitalFlow = money
			item.CapitalSummary = capitalFlowSummary(money)

			// 板块
			var info models.AllStockInfo
			if dberr := db.Dao.Model(&models.AllStockInfo{}).
				Where("secucode LIKE ?", strings.Split(apiCode, ".")[0]+".%").
				Order("secucode ASC").First(&info).Error; dberr == nil {
				item.Industry = strings.TrimSpace(info.INDUSTRY)
			}
			item.IndustryFlow = industryFlowSummary(item.Industry)

			// 新闻（最多5条）
			if news, nerr := NewMarketNewsApi().GetStockRelatedNews(apiCode, 5); nerr == nil && news != nil {
				item.News = news
			}

			// 历史AI推荐
			item.AiAdvice = GetLatestAiAdviceForStock(p.StockCode, p.StockName)

			// Kronos 未来K线预测（可选增强：未开启或失败时静默降级）
			if GetKronosConfig().Enable {
				pctx, pcancel := context.WithTimeout(ctx, 45*time.Second)
				pred, perr := kronosPredict(pctx, apiCode, 0)
				pcancel()
				if perr == nil && pred != nil {
					item.Kronos = pred
				} else if perr != nil && !errors.Is(perr, ErrKronosDisabled) && !errors.Is(perr, ErrKronosOffline) {
					logger.SugaredLogger.Warnf("Kronos 预测 %s 失败: %v", p.StockCode, perr)
				}
			}
		}()
		result = append(result, item)
	}
	return result, nil
}

// BuildHoldingsDeepPrompt 将持仓深度数据包整理成 AI 综合分析的输入 markdown。
func BuildHoldingsDeepPrompt(stocks []*HoldingsDeepStock) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业、务实的证券投资分析师。以下是基于真实行情、资金流、板块与新闻数据整理的持仓多维数据，")
	sb.WriteString("请逐股深度分析并给出组合层面的综合结论（今日日期：")
	sb.WriteString(time.Now().Format("2006-01-02"))
	sb.WriteString("）。\n\n")

	for i, s := range stocks {
		fmt.Fprintf(&sb, "## 持仓%d：%s(%s)\n\n", i+1, s.StockName, s.StockCode)
		fmt.Fprintf(&sb, "- 现价：%.2f（今日%.2f%%）；持仓收益率：%.2f%%；占组合：%.1f%%\n",
			s.CurrentPrice, s.ChangePercent, s.ProfitPercent, s.PositionPct)
		if s.Indicators != nil {
			fmt.Fprintf(&sb, "- 技术面：趋势%s；MACD %s；RSI14 %.1f(%s)；KDJ %s；布林%s\n",
				s.Indicators.Trend, s.Indicators.MACDSignal, s.Indicators.RSIValue, s.Indicators.RSIStatus, s.Indicators.KDJSignal, s.Indicators.BollStatus)
			if s.Indicators.GapStatus != "" {
				sb.WriteString("- 缺口：" + s.Indicators.GapStatus + "\n")
			}
		}
		if s.Levels != nil {
			sb.WriteString("- 关键价位（本地计算，供你校验与引用）：\n")
			sb.WriteString("  - 压力位：" + formatLevels(s.Levels.Resistance) + "\n")
			sb.WriteString("  - 支撑位：" + formatLevels(s.Levels.Support) + "\n")
			sb.WriteString("  - 买点参考：" + formatLevels(s.Levels.BuyPoints) + "\n")
			sb.WriteString("  - 卖出/止损参考：" + formatLevels(s.Levels.SellPoints) + "\n")
		}
		sb.WriteString("- 资金面：" + s.CapitalSummary + "\n")
		if s.IndustryFlow != "" {
			sb.WriteString("- 板块：" + s.IndustryFlow + "；所属行业：" + orDashStr(s.Industry) + "\n")
		} else if s.Industry != "" {
			sb.WriteString("- 板块：所属行业 " + s.Industry + "\n")
		}
		if s.AiAdvice != nil {
			sb.WriteString(fmt.Sprintf("- 历史AI推荐(%s)：评级%s；止盈区间%s；止损%s；理由：%s\n",
				s.AiAdvice.DataTime, orDashStr(s.AiAdvice.Rating),
				priceRangeStr(s.AiAdvice.TakeProfitMin, s.AiAdvice.TakeProfitMax), priceStr(s.AiAdvice.StopLossPrice),
				truncateStr(orDashStr(s.AiAdvice.Reason), 120)))
		}
		if s.Kronos != nil && s.Kronos.Summary != nil {
			ks := s.Kronos.Summary
			dirTxt := "下跌"
			if ks.Direction == "up" {
				dirTxt = "上涨"
			}
			fmt.Fprintf(&sb, "- Kronos模型K线预测（未来%d个交易日，仅供参考非投资建议）：预测方向%s；预测期末收盘 %.2f（较现价 %+.2f%%）；预测区间 %.2f ~ %.2f；采样一致度 %.0f/100\n",
				ks.PredLen, dirTxt, ks.PredEnd, ks.ChangePct, ks.PredLow, ks.PredHigh, ks.Confidence)
			if len(s.Kronos.Bars) > 0 {
				var pb []string
				for _, b := range s.Kronos.Bars {
					pb = append(pb, fmt.Sprintf("%s:%.2f", b.Date, b.Close))
				}
				sb.WriteString("  - 预测收盘序列：" + strings.Join(pb, " → ") + "\n")
			}
		}
		if len(s.News) > 0 {
			sb.WriteString("- 最近新闻：\n")
			for _, n := range s.News {
				sb.WriteString("  - " + truncateStr(n.Title, 60) + "（" + n.Time + " " + n.Source + "）\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`
## 输出要求
请输出 markdown 格式的持仓深度分析报告，结构如下：

### 一、个股深度分析
每只持仓一小节（用持仓代码做小节标题），每节必须包含：
1. **关键点位（直白）**：用表格列出 压力位/支撑位/可以买入的参考点/应该卖出或止损的参考点，每个点位一行价格+一句依据；点位数值以数据包中的本地计算为基准，可微调但不要凭空编造；
2. **资金面解读**：主力资金方向与力度，对短期走势的含义；
3. **消息与板块**：结合新闻与所属板块资金，指出利好/利空/中性及持续性；
4. **操作结论（明确）**：一句话给出当前应「持有/加仓/减仓/买入/卖出/观望」，以及触发加仓或离场的具体价格条件。
   若提供了 Kronos 模型K线预测，请将其作为技术面参考之一与本地指标相互印证（一致则增强结论，矛盾则明确指出分歧及各自依据）；预测置信度低于 50 时应弱化其权重并说明。

### 二、组合综合分析
1. 组合仓位结构评价（集中度、行业分布、浮盈浮亏）；
2. 组合层面主要风险点（个股共振、板块暴露、消息面风险）；
3. 明日重点关注清单（具体到价格条件或事件）；
4. 总体结论：一句话给组合当前的综合评级（积极/中性/防御）与最优先要处理的一笔持仓。

要求：观点直白鲜明、每个结论都有数据依据；禁止空话套话；点位必须给出具体数字。`)
	return sb.String()
}

func formatLevels(levels []PriceLevel) string {
	if len(levels) == 0 {
		return "暂无有效点位"
	}
	parts := make([]string, 0, len(levels))
	for _, l := range levels {
		parts = append(parts, fmt.Sprintf("%.2f（%s）", l.Price, l.Label))
	}
	return strings.Join(parts, "、")
}

func orDashStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return strings.TrimSpace(s)
}

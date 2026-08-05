package main

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 本文件为价格监控与预警相关逻辑，从 app.go 纯搬运拆分（含新闻推送、自选/AI 推荐股/基金监控、报警去重等）。

func (a *App) NewsPush(news *[]models.Telegraph) {

	follows := data.NewStockDataApi().GetFollowList(0)
	stockNames := slice.Map(*follows, func(index int, item data.FollowedStock) string {
		return item.Name
	})

	for _, telegraph := range *news {
		if a.systemHandler.GetConfig().EnableOnlyPushRedNews {
			if telegraph.IsRed || strutil.ContainsAny(telegraph.Content, stockNames) {
				go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
			}
		} else {
			go runtime.EventsEmit(a.ctx, "newsPush", telegraph)
		}
		//go data.NewAlertWindowsApi("go-stock", telegraph.Source+" "+telegraph.Time, telegraph.Content, string(icon)).SendNotification()
		//}
	}
}

func (a *App) AddCronTask(follow data.FollowedStock) func() {
	return func() {
		go runtime.EventsEmit(a.ctx, "warnMsg", "开始自动分析"+follow.Name+"_"+follow.StockCode)
		ai := data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId)
		thinking := data.GetSettingConfig().GetAIConfigThinking(follow.AiConfigId)
		msgs := ai.NewChatStream(follow.Name, follow.StockCode, "", nil, a.AiTools, thinking)
		var res strings.Builder

		chatId := ""
		question := ""
		for msg := range msgs {
			if v, ok := msg["extraContent"].(string); ok && v != "" {
				res.WriteString(v + "\n")
			}
			if v, ok := msg["content"].(string); ok && v != "" {
				res.WriteString(v)
			}
			if v, ok := msg["chatId"].(string); ok {
				chatId = v
			}
			if v, ok := msg["question"].(string); ok {
				question = v
			}
		}

		data.NewDeepSeekOpenAi(a.ctx, follow.AiConfigId).SaveAIResponseResult(follow.StockCode, follow.Name, res.String(), chatId, question)
		go runtime.EventsEmit(a.ctx, "warnMsg", "AI分析完成："+follow.Name+"_"+follow.StockCode)

	}
}

func MonitorFundPrices(a *App) {
	// 检查 A 股是否开市（基金交易时间与 A 股一致）
	if !isTradingTime(time.Now()) {
		logger.SugaredLogger.Debugf("当前 A 股未开市，跳过基金价格监控")
		return
	}

	logger.SugaredLogger.Debugf("A 股市场已开市，开始基金价格监控")

	dest := &[]data.FollowedFund{}
	db.Dao.Model(&data.FollowedFund{}).Find(dest)
	for _, follow := range *dest {
		_, err := data.NewFundApi().CrawlFundBasic(follow.Code)
		if err != nil {
			logger.SugaredLogger.Errorf("获取基金基本信息失败，基金代码：%s，错误信息：%s", follow.Code, err.Error())
			continue
		}
		data.NewFundApi().CrawlFundNetEstimatedUnit(follow.Code)
		data.NewFundApi().CrawlFundNetUnitValue(follow.Code)
	}
}

// MonitorAiRecommendStockPrices 监控 AI 推荐股票的价格，当股价达到预警线时发送通知
func MonitorAiRecommendStockPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过 AI 推荐股票价格监控")
		return
	}

	var aiRecommendStocks []models.AiRecommendStocks
	db.Dao.Model(&models.AiRecommendStocks{}).Where("enable_alert = ?", true).Find(&aiRecommendStocks)

	if len(aiRecommendStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockCodeMap := make(map[string]*models.AiRecommendStocks)
	for i := range aiRecommendStocks {
		stock := &aiRecommendStocks[i]
		stopLossPrice, _ := convertor.ToFloat(stock.RecommendStopLossPrice)
		if stock.RecommendBuyPriceMin <= 0 && stock.RecommendStopProfitPriceMin <= 0 && stopLossPrice <= 0 {
			continue
		}
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockCodeMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	if len(stockCodes) == 0 {
		logger.SugaredLogger.Debugf("没有设置预警价格的 AI 推荐股票，跳过价格监控")
		return
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取 AI 推荐股票实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		aiStock, ok := stockCodeMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		baseAlertKey := fmt.Sprintf("%s:%s", aiStock.StockCode, aiStock.DataTime.Format("20060102"))

		buyAlertKey := baseAlertKey + ":BUY"
		if aiStock.RecommendBuyPriceMin > 0 && currentPrice <= aiStock.RecommendBuyPriceMin {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if priceSinceLastBuyAlert == 0 || priceSinceLastBuyAlert > aiStock.RecommendBuyPriceMin {
				title := fmt.Sprintf("【买入预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议买入价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议买入价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendBuyPriceMin, aiStock.RecommendBuyPriceMax)
				if a.canSendAlert(buyAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(buyAlertKey)
					a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		} else {
			priceSinceLastBuyAlert := a.getPriceAtAlertReset(buyAlertKey)
			if currentPrice > aiStock.RecommendBuyPriceMin && (priceSinceLastBuyAlert == 0 || currentPrice > priceSinceLastBuyAlert) {
				a.updatePriceAtAlertReset(buyAlertKey, currentPrice)
			}
		}

		profitAlertKey := baseAlertKey + ":PROFIT"
		if aiStock.RecommendStopProfitPriceMin > 0 && currentPrice >= aiStock.RecommendStopProfitPriceMin {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if priceSinceLastProfitAlert == 0 || priceSinceLastProfitAlert < aiStock.RecommendStopProfitPriceMin {
				title := fmt.Sprintf("【止盈预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止盈价**: %.2f - %.2f\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止盈价: %.2f-%.2f",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopProfitPriceMin, aiStock.RecommendStopProfitPriceMax)
				if a.canSendAlert(profitAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(profitAlertKey)
					a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		} else {
			priceSinceLastProfitAlert := a.getPriceAtAlertReset(profitAlertKey)
			if currentPrice < aiStock.RecommendStopProfitPriceMin && (priceSinceLastProfitAlert == 0 || currentPrice < priceSinceLastProfitAlert) {
				a.updatePriceAtAlertReset(profitAlertKey, currentPrice)
			}
		}

		stopLossAlertKey := baseAlertKey + ":LOSS"
		stopLossPrice, _ := convertor.ToFloat(aiStock.RecommendStopLossPrice)
		if stopLossPrice > 0 && currentPrice <= stopLossPrice {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if priceSinceLastLossAlert == 0 || priceSinceLastLossAlert > stopLossPrice {
				title := fmt.Sprintf("【止损预警】%s", aiStock.StockName)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **建议止损价**: %s\n- **推荐时间**: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice,
					aiStock.DataTime.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n建议止损价: %s",
					aiStock.StockName, aiStock.StockCode, currentPrice, aiStock.RecommendStopLossPrice)
				if a.canSendAlert(stopLossAlertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(stopLossAlertKey)
					a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		} else {
			priceSinceLastLossAlert := a.getPriceAtAlertReset(stopLossAlertKey)
			if currentPrice > stopLossPrice && (priceSinceLastLossAlert == 0 || currentPrice > priceSinceLastLossAlert) {
				a.updatePriceAtAlertReset(stopLossAlertKey, currentPrice)
			}
		}
	}
}

// MonitorFollowedStockCostPrices 监控自选股的持仓成本价，当股价低于成本价时发送预警
func MonitorFollowedStockCostPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		logger.SugaredLogger.Debugf("当前所有市场均未开市，跳过自选股成本价监控")
		return
	}

	var followedStocks []data.FollowedStock
	db.Dao.Model(&data.FollowedStock{}).Where("cost_price > 0").Find(&followedStocks)

	if len(followedStocks) == 0 {
		return
	}

	stockCodes := make([]string, 0)
	stockMap := make(map[string]*data.FollowedStock)
	for i := range followedStocks {
		stock := &followedStocks[i]
		stockCodes = append(stockCodes, tools.GetStockCode(stock.StockCode))
		stockMap[tools.GetStockCode(stock.StockCode)] = stock
	}

	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil || len(*stockData) == 0 {
		logger.SugaredLogger.Errorf("获取自选股实时数据失败: %v", err)
		return
	}

	for _, stockInfo := range *stockData {
		followedStock, ok := stockMap[tools.GetStockCode(stockInfo.Code)]
		if !ok {
			continue
		}

		currentPrice, _ := convertor.ToFloat(stockInfo.Price)
		if currentPrice <= 0 {
			continue
		}

		costPrice := followedStock.CostPrice
		if costPrice <= 0 {
			continue
		}

		alertKey := fmt.Sprintf("COST:%s:%s", followedStock.StockCode, followedStock.Time.Format("20060102"))

		if currentPrice < costPrice {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if priceSinceLastAlert == 0 || priceSinceLastAlert >= costPrice {
				dropPercent := ((costPrice - currentPrice) / costPrice) * 100
				title := fmt.Sprintf("【成本价预警】%s", followedStock.Name)
				content := fmt.Sprintf("## %s\n\n- **股票代码**: %s\n- **当前价格**: %.2f\n- **持仓成本价**: %.2f\n- **亏损比例**: %.2f%%\n- **关注时间**: %s",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent,
					followedStock.Time.Format("2006-01-02 15:04:05"))
				plainContent := fmt.Sprintf("%s(%s)\n当前价格: %.2f\n成本价: %.2f\n亏损: %.2f%%",
					followedStock.Name, followedStock.StockCode, currentPrice, costPrice, dropPercent)
				if a.canSendAlert(alertKey, 5*time.Minute) {
					go data.NewAlertWindowsApi("go-stock价格预警", title, content, "").SendNotification()
					go data.NewDingDingAPI().SendToDingDing(title, content)
					go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
						"time":    title,
						"isRed":   true,
						"source":  "go-stock",
						"content": plainContent,
					})
					a.updateAlertSentTime(alertKey)
					a.updatePriceAtAlertReset(alertKey, currentPrice)
				}
			} else {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		} else {
			priceSinceLastAlert := a.getPriceAtAlertReset(alertKey)
			if currentPrice >= costPrice && (priceSinceLastAlert == 0 || currentPrice < priceSinceLastAlert) {
				a.updatePriceAtAlertReset(alertKey, currentPrice)
			}
		}
	}
}

// canSendAlert 检查是否可以发送预警，避免重复发送
// alertKey: 预警的唯一标识
// interval: 发送间隔
// 返回 true 表示可以发送，false 表示需要在间隔后才能发送
func (a *App) canSendAlert(alertKey string, interval time.Duration) bool {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()

	lastSent, exists := a.stockAlertLastSent[alertKey]
	if !exists {
		return true
	}

	return time.Since(lastSent) >= interval
}

// updateAlertSentTime 更新预警发送时间
func (a *App) updateAlertSentTime(alertKey string) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.stockAlertLastSent[alertKey] = time.Now()
}

// getPriceAtAlertReset 获取预警重置后的价格（用于判断是否需要重新触发预警）
func (a *App) getPriceAtAlertReset(alertKey string) float64 {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	return a.priceAtAlertReset[alertKey]
}

// updatePriceAtAlertReset 更新预警重置后的价格
func (a *App) updatePriceAtAlertReset(alertKey string, price float64) {
	a.stockAlertMu.Lock()
	defer a.stockAlertMu.Unlock()
	a.priceAtAlertReset[alertKey] = price
}

func GetStockInfos(follows ...data.FollowedStock) *[]data.StockInfo {
	stockInfos := make([]data.StockInfo, 0)
	stockCodes := make([]string, 0)
	for _, follow := range follows {
		if strutil.HasPrefixAny(follow.StockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}
		stockCodes = append(stockCodes, follow.StockCode)
	}
	stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockData == nil {
		return &stockInfos
	}
	for _, info := range *stockData {
		v, ok := slice.FindBy(follows, func(idx int, follow data.FollowedStock) bool {
			if strutil.HasPrefixAny(follow.StockCode, []string{"US", "us"}) {
				return strings.ToLower(strings.Replace(follow.StockCode, "us", "gb_", 1)) == info.Code
			}

			return follow.StockCode == info.Code
		})
		if ok {
			addStockFollowData(v, &info)
			stockInfos = append(stockInfos, info)
		}
	}
	return &stockInfos
}
func addStockFollowData(follow data.FollowedStock, stockData *data.StockInfo) {
	stockData.PrePrice = follow.Price //上次当前价格
	stockData.Sort = follow.Sort
	stockData.CostPrice = follow.CostPrice //成本价
	stockData.CostVolume = follow.Volume   //成本量
	stockData.AlarmChangePercent = follow.AlarmChangePercent
	stockData.AlarmPrice = follow.AlarmPrice
	stockData.Groups = follow.Groups

	//当前价格
	price, _ := convertor.ToFloat(stockData.Price)
	//当前价格为0 时 使用卖一价格作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.A1P)
	}
	//当前价格依然为0 时 使用买一报价作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.B1P)
	}

	//昨日收盘价
	preClosePrice, _ := convertor.ToFloat(stockData.PreClose)

	//当前价格依然为0 时 使用昨日收盘价为当前价格
	if price == 0 {
		price = preClosePrice
	}

	//今日最高价
	highPrice, _ := convertor.ToFloat(stockData.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(stockData.Open)
	}

	//今日最低价
	lowPrice, _ := convertor.ToFloat(stockData.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(stockData.Open)
	}
	//开盘价
	//openPrice, _ := convertor.ToFloat(stockData.Open)

	if price > 0 && preClosePrice > 0 {
		stockData.ChangePrice = mathutil.RoundToFloat(price-preClosePrice, 2)
		stockData.ChangePercent = mathutil.RoundToFloat(mathutil.Div(price-preClosePrice, preClosePrice)*100, 3)
	}
	if highPrice > 0 && preClosePrice > 0 {
		stockData.HighRate = mathutil.RoundToFloat(mathutil.Div(highPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if lowPrice > 0 && preClosePrice > 0 {
		stockData.LowRate = mathutil.RoundToFloat(mathutil.Div(lowPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if follow.CostPrice > 0 && follow.Volume > 0 {
		if price > 0 {
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(price-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((price-follow.CostPrice)*float64(follow.Volume), 2)
			stockData.ProfitAmountToday = mathutil.RoundToFloat((price-preClosePrice)*float64(follow.Volume), 2)
		} else {
			//未开盘时当前价格为昨日收盘价
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((preClosePrice-follow.CostPrice)*float64(follow.Volume), 2)
			// 未开盘时，今日盈亏为 0
			stockData.ProfitAmountToday = 0
		}

	}

	//logger.SugaredLogger.Debugf("stockData:%+v", stockData)
	if follow.Price != price && price > 0 {
		go db.Dao.Model(follow).Where("stock_code = ?", follow.StockCode).Updates(map[string]interface{}{
			"price": price,
		})
	}
}

func GenNotificationMsg(stockInfo *data.StockInfo) string {
	Price, err := convertor.ToFloat(stockInfo.Price)
	if err != nil {
		Price = 0
	}
	PreClose, err := convertor.ToFloat(stockInfo.PreClose)
	if err != nil {
		PreClose = 0
	}
	var RF float64
	if PreClose > 0 {
		RF = mathutil.RoundToFloat(((Price-PreClose)/PreClose)*100, 2)
	}

	return "[" + stockInfo.Name + "] " + stockInfo.Price + " " + convertor.ToString(RF) + "% " + stockInfo.Date + " " + stockInfo.Time
}

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟) 4 止盈报警(5分钟) 5 止损报警(5分钟)
func getMsgTypeTTL(msgType int) int {
	switch msgType {
	case 1:
		return 60 * 5
	case 2:
		return 60 * 30
	case 3:
		return 60 * 30
	case 4:
		return 60 * 5
	case 5:
		return 60 * 5
	default:
		return 60 * 5
	}
}

func getMsgTypeName(msgType int) string {
	switch msgType {
	case 1:
		return "涨跌报警"
	case 2:
		return "股价报警"
	case 3:
		return "成本价报警"
	case 4:
		return "止盈报警"
	case 5:
		return "止损报警"
	default:
		return "未知类型"
	}
}

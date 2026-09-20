// Package data ai_recommend_stocks_api.go
package data

import (
	"context"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"math"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/datetime"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/tidwall/gjson"
)

type AiRecommendStocksService struct{}

func NewAiRecommendStocksService() *AiRecommendStocksService {
	return &AiRecommendStocksService{}
}

// CreateAiRecommendStocks 创建AI推荐股票记录
func (s *AiRecommendStocksService) CreateAiRecommendStocks(recommend *models.AiRecommendStocks) error {
	result := db.Dao.Create(recommend)
	return result.Error
}

func (s *AiRecommendStocksService) BatchCreateAiRecommendStocks(recommends []*models.AiRecommendStocks) error {
	result := db.Dao.Create(recommends)
	return result.Error
}

// GetAiRecommendStocksList 分页查询AI推荐股票记录
func (s *AiRecommendStocksService) GetAiRecommendStocksList(query *models.AiRecommendStocksQuery) (*models.AiRecommendStocksPageData, error) {
	var list []models.AiRecommendStocks
	var total int64

	q := db.Dao.Model(&models.AiRecommendStocks{})

	// 构建关键词搜索条件（股票代码、股票名称、板块名称使用 OR 关系）
	keyword := query.StockCode
	if keyword == "" {
		keyword = query.StockName
	}
	if keyword == "" {
		keyword = query.BkName
	}
	if keyword == "" {
		keyword = query.ModelName
	}

	if keyword != "" {
		q = q.Where("(stock_code LIKE ? OR stock_name LIKE ? OR bk_name LIKE ? OR model_name LIKE ?)",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 日期范围查询
	if query.StartDate != "" && query.EndDate != "" {
		query.StartDate = strutil.ReplaceWithMap(query.StartDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		query.EndDate = strutil.ReplaceWithMap(query.EndDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		startDate, err := time.Parse("2006-01-02 15:04:05", query.StartDate)
		if err != nil {
			startDate, _ = time.Parse("2006-01-02", query.StartDate)
		}

		endDate, err := time.Parse("2006-01-02 15:04:05", query.EndDate)
		if err != nil {
			endDate, _ = time.Parse("2006-01-02", query.EndDate)
		}

		q = q.Where("data_time BETWEEN ? AND ?", datetime.BeginOfDay(startDate), datetime.EndOfDay(endDate))
	} else if query.StartDate == "" && query.EndDate == "" && keyword == "" {
		// 只有在没有关键词时才默认查询今天的数据
		q = q.Where("data_time BETWEEN ? AND ?", datetime.BeginOfDay(time.Now()), datetime.EndOfDay(time.Now()))
	} else if query.StartDate != "" && query.EndDate == "" {
		query.StartDate = strutil.ReplaceWithMap(query.StartDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		startDate, _ := time.Parse("2006-01-02", query.StartDate)
		q = q.Where("data_time BETWEEN ? AND ?", datetime.BeginOfDay(startDate), datetime.EndOfDay(startDate))
	}

	// 预警状态筛选
	if query.EnableAlert != nil {
		q = q.Where("enable_alert = ?", *query.EnableAlert)
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	stockCodes := slice.Map(list, func(index int, item models.AiRecommendStocks) string {
		return ConvertTushareCodeToStockCode(item.StockCode)
	})
	stockData, _ := NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	for _, info := range *stockData {
		for idx, item := range list {
			if ConvertTushareCodeToStockCode(item.StockCode) == ConvertTushareCodeToStockCode(info.Code) {
				list[idx].StockCurrentPrice = info.Price
				list[idx].StockPrePrice = info.PreClose
				list[idx].StockCurrentPriceTime = info.Date + " " + info.Time
			}
		}
	}

	return &models.AiRecommendStocksPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAiRecommendStocksByID 根据ID获取AI推荐股票记录
func (s *AiRecommendStocksService) GetAiRecommendStocksByID(id uint) (*models.AiRecommendStocks, error) {
	var recommend models.AiRecommendStocks
	err := db.Dao.First(&recommend, id).Error
	if err != nil {
		return nil, err
	}
	return &recommend, nil
}

// UpdateAiRecommendStocks 更新AI推荐股票记录
func (s *AiRecommendStocksService) UpdateAiRecommendStocks(id uint, recommend *models.AiRecommendStocks) error {
	result := db.Dao.Model(&models.AiRecommendStocks{}).Where("id = ?", id).Updates(recommend)
	return result.Error
}

// DeleteAiRecommendStocks 根据ID删除AI推荐股票记录
func (s *AiRecommendStocksService) DeleteAiRecommendStocks(id uint) error {
	// 使用软删除
	result := db.Dao.Where("id = ?", id).Delete(&models.AiRecommendStocks{})
	return result.Error
}

// UpdateAiRecommendStocksAlert 更新AI推荐股票的预警状态
func (s *AiRecommendStocksService) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) error {
	result := db.Dao.Model(&models.AiRecommendStocks{}).Where("id = ?", id).Update("enable_alert", enableAlert)
	return result.Error
}

// BatchDeleteAiRecommendStocks 批量删除AI推荐股票记录
func (s *AiRecommendStocksService) BatchDeleteAiRecommendStocks(ids []uint) error {
	// 使用软删除
	result := db.Dao.Where("id IN ?", ids).Delete(&models.AiRecommendStocks{})
	return result.Error
}

// ── Stats ──

type ModelStat struct {
	ModelName string  `json:"modelName"`
	WinRate   float64 `json:"winRate"`
	AvgReturn float64 `json:"avgReturn"`
	Count     int     `json:"count"`
}

type SectorStat struct {
	BkName string `json:"bkName"`
	Count  int    `json:"count"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type AiRecommendStats struct {
	ByModel    []ModelStat  `json:"byModel"`
	BySector   []SectorStat `json:"bySector"`
	DailyCount []DailyCount `json:"dailyCount"`
}

func parsePrice(s string) float64 {
	if s == "" {
		return 0
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err == nil {
		return v
	}
	return 0
}

// TradingAiAdvice 单只股票最近一次 AI 推荐的建议摘要（供交易日志带入止损/止盈与点评参考）
type TradingAiAdvice struct {
	StockCode string `json:"stockCode"`
	StockName string `json:"stockName"`
	Rating    string `json:"rating"`
	ModelName string `json:"modelName"`
	DataTime  string `json:"dataTime"`
	// RecommendId 最近一条 AI 推荐记录 ID
	RecommendId uint `json:"recommendId"`
	// StopLossPrice 建议止损价（取 AI 止损价的首个数值，区间时为下限）
	StopLossPrice float64 `json:"stopLossPrice"`
	// TakeProfitPrice 建议止盈价（止盈区间下限，保守目标位）
	TakeProfitPrice float64 `json:"takeProfitPrice"`
	// TakeProfitMin/TakeProfitMax 建议止盈区间
	TakeProfitMin float64 `json:"takeProfitMin"`
	TakeProfitMax float64 `json:"takeProfitMax"`
	// BuyPriceMin/BuyPriceMax 建议买入区间
	BuyPriceMin float64 `json:"buyPriceMin"`
	BuyPriceMax float64 `json:"buyPriceMax"`
	// Reason 推荐理由/驱动因素/逻辑
	Reason string `json:"reason"`
	// RiskRemarks 风险提示
	RiskRemarks string `json:"riskRemarks"`
}

// bareStockCode 归一化股票代码：去市场前缀（sh/sz/bj/hk）与后缀（.SH/.SZ/.BJ），统一大写
func bareStockCode(code string) string {
	bare := strings.ToLower(strings.TrimSpace(code))
	for _, suffix := range []string{".sh", ".sz", ".bj"} {
		bare = strings.TrimSuffix(bare, suffix)
	}
	for _, prefix := range []string{"sh", "sz", "bj", "hk", "us"} {
		bare = strings.TrimPrefix(bare, prefix)
	}
	return strings.ToUpper(strings.TrimSpace(bare))
}

// parseRangePrice 解析 "10.5-12.3"/"10.5" 形式的价格区间，返回 (min, max)
func parseRangePrice(s string) (float64, float64) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0
	}
	parts := strings.SplitN(s, "-", 2)
	minV := parsePrice(parts[0])
	if len(parts) == 1 {
		return minV, 0
	}
	maxV := parsePrice(parts[1])
	if maxV < minV {
		maxV = minV
	}
	return minV, maxV
}

// GetLatestAiAdviceForStock 查询该股票最近一次 AI 推荐建议；
// 先按代码精确（归一化后）匹配，找不到再按名称精确匹配。无匹配返回 nil。
func GetLatestAiAdviceForStock(stockCode, stockName string) *TradingAiAdvice {	bare := bareStockCode(stockCode)
	if bare == "" && strings.TrimSpace(stockName) == "" {
		return nil
	}

	var candidates []models.AiRecommendStocks
	q := db.Dao.Model(&models.AiRecommendStocks{})
	if bare != "" {
		q = q.Where("stock_code LIKE ?", "%"+bare+"%")
	} else {
		q = q.Where("stock_name = ?", strings.TrimSpace(stockName))
	}
	if err := q.Order("data_time DESC").Limit(20).Find(&candidates).Error; err != nil {
		logger.SugaredLogger.Warnf("查询AI推荐建议失败: %s", err.Error())
		return nil
	}

	var hit *models.AiRecommendStocks
	for i := range candidates {
		if bare != "" && bareStockCode(candidates[i].StockCode) == bare {
			hit = &candidates[i]
			break
		}
	}
	// 代码没匹配上（LIKE 命中的是别的股票），退回按名称精确匹配
	if hit == nil && strings.TrimSpace(stockName) != "" {
		for i := range candidates {
			if candidates[i].StockName == strings.TrimSpace(stockName) {
				hit = &candidates[i]
				break
			}
		}
		if hit == nil {
			var byName []models.AiRecommendStocks
			if err := db.Dao.Model(&models.AiRecommendStocks{}).
				Where("stock_name = ?", strings.TrimSpace(stockName)).
				Order("data_time DESC").Limit(1).Find(&byName).Error; err == nil && len(byName) > 0 {
				hit = &byName[0]
			}
		}
	}
	if hit == nil {
		return nil
	}

	advice := &TradingAiAdvice{
		StockCode:    hit.StockCode,
		StockName:    hit.StockName,
		Rating:       hit.Rating,
		ModelName:    hit.ModelName,
		RecommendId:  hit.ID,
		Reason:       hit.RecommendReason,
		RiskRemarks:  hit.RiskRemarks,
		StopLossPrice: parsePrice(hit.RecommendStopLossPrice),
	}
	if hit.DataTime != nil {
		advice.DataTime = hit.DataTime.Format("2006-01-02 15:04:05")
	}
	advice.TakeProfitMin, advice.TakeProfitMax = hit.RecommendStopProfitPriceMin, hit.RecommendStopProfitPriceMax
	if advice.TakeProfitMin == 0 && advice.TakeProfitMax == 0 {
		advice.TakeProfitMin, advice.TakeProfitMax = parseRangePrice(hit.RecommendStopProfitPrice)
	}
	advice.TakeProfitPrice = advice.TakeProfitMin
	advice.BuyPriceMin, advice.BuyPriceMax = hit.RecommendBuyPriceMin, hit.RecommendBuyPriceMax
	if advice.BuyPriceMin == 0 && advice.BuyPriceMax == 0 {
		advice.BuyPriceMin, advice.BuyPriceMax = parseRangePrice(hit.RecommendBuyPrice)
	}
	return advice
}

// AiSuggestPriceLevels 调用 AI 基于最新技术指标直接给出该股的建议止损/止盈价位（同步调用，
// 内部走流式接口累积全文，耗时约 10~30 秒）。历史有 AI 推荐记录时一并作为参考上下文。
// 生成失败或内容无法解析时返回 nil。
func AiSuggestPriceLevels(ctx context.Context, stockCode, stockName string, price float64, aiConfigId int) *TradingAiAdvice {
	if aiConfigId <= 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"你是专业证券交易员。请为 %s(%s) 给出建议的交易价位。当前参考价格：%.2f 元（价格为 0 表示获取失败，请基于技术面自行判断）。\n",
		stockName, stockCode, price))

	if ind, indErr := GetHoldingsStockIndicators(ctx, stockCode); indErr == nil && ind != nil && ind.Summary != nil {
		s := ind.Summary
		sb.WriteString(fmt.Sprintf("\n## 最新技术指标（日线）\n- 趋势：%s；MACD：%s；RSI14：%.1f（%s）；KDJ：%s；布林位置：%s\n- 解读：%s\n",
			s.Trend, s.MACDSignal, s.RSIValue, s.RSIStatus, s.KDJSignal, s.BollStatus, s.Summary))
		if s.GapStatus != "" {
			sb.WriteString("- 跳空缺口：" + s.GapStatus + "（未回补缺口可作为支撑/压力参考）\n")
		}
	}
	if advice := GetLatestAiAdviceForStock(stockCode, stockName); advice != nil {
		sb.WriteString(fmt.Sprintf("\n## 历史AI推荐（%s）\n- 评级：%s；建议买入区间：%s；建议止盈区间：%s；建议止损价：%s\n",
			advice.DataTime, advice.Rating, priceRangeStr(advice.BuyPriceMin, advice.BuyPriceMax),
			priceRangeStr(advice.TakeProfitMin, advice.TakeProfitMax), priceStr(advice.StopLossPrice)))
	}

	sb.WriteString(`
## 任务
结合以上信息给出建议止损价与止盈区间。要求：止损位依据关键支撑/波动幅度设置；止盈区间依据压力位/目标涨幅设置。

## 输出格式
只输出如下 JSON，禁止输出任何其它文字、解释或 markdown 代码块：
{"stopLoss": 止损价数字, "takeProfitLow": 止盈下限数字, "takeProfitHigh": 止盈上限数字, "reason": "不超过50字的设置依据"}`)

	ai := NewDeepSeekOpenAi(ctx, aiConfigId)
	ch := ai.NewSummaryStockNewsStream(sb.String(), nil, false, nil)
	var full strings.Builder
	for msg := range ch {
		if content, ok := msg["content"].(string); ok {
			full.WriteString(content)
		}
	}
	if full.Len() == 0 {
		return nil
	}

	// 从回复中提取 JSON（容忍被 markdown 代码块包裹）
	text := full.String()
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		logger.SugaredLogger.Warnf("AI建议价位解析失败: %s", text)
		return nil
	}
	jsonText := text[start : end+1]

	stopLoss := gjson.Get(jsonText, "stopLoss").Float()
	tpLow := gjson.Get(jsonText, "takeProfitLow").Float()
	tpHigh := gjson.Get(jsonText, "takeProfitHigh").Float()
	reason := gjson.Get(jsonText, "reason").String()
	if stopLoss <= 0 && tpLow <= 0 && tpHigh <= 0 {
		logger.SugaredLogger.Warnf("AI建议价位数值无效: %s", jsonText)
		return nil
	}
	if tpHigh < tpLow {
		tpHigh = tpLow
	}
	return &TradingAiAdvice{
		StockCode:       stockCode,
		StockName:       stockName,
		ModelName:       ai.GetModel(),
		DataTime:        time.Now().Format("2006-01-02 15:04:05"),
		StopLossPrice:   stopLoss,
		TakeProfitPrice: tpLow,
		TakeProfitMin:   tpLow,
		TakeProfitMax:   tpHigh,
		Reason:          reason,
	}
}

// priceStr 价格为 0 显示 "-"
func priceStr(p float64) string {
	if p > 0 {
		return fmt.Sprintf("%.2f", p)
	}
	return "-"
}

// priceRangeStr 价格区间文案，双零显示 "-"
func priceRangeStr(minV, maxV float64) string {
	if minV <= 0 && maxV <= 0 {
		return "-"
	}
	if maxV > minV {
		return fmt.Sprintf("%.2f ~ %.2f", minV, maxV)
	}
	return priceStr(minV)
}

func (s *AiRecommendStocksService) GetAiRecommendStats() (*AiRecommendStats, error) {
	var all []models.AiRecommendStocks
	if err := db.Dao.Model(&models.AiRecommendStocks{}).Find(&all).Error; err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return &AiRecommendStats{}, nil
	}

	// ByModel
	modelMap := make(map[string]struct{ wins int; total int; retSum float64 })
	for _, r := range all {
		key := r.ModelName
		if key == "" {
			key = "unknown"
		}
		entry := modelMap[key]
		entry.total++
		current := parsePrice(r.StockCurrentPrice)
		orig := parsePrice(r.StockPrice)
		if orig > 0 && current > 0 {
			if current > orig {
				entry.wins++
			}
			entry.retSum += (current - orig) / orig
		}
		modelMap[key] = entry
	}
	var byModel []ModelStat
	for name, m := range modelMap {
		wr := 0.0
		if m.total > 0 {
			wr = float64(m.wins) / float64(m.total) * 100
		}
		avgRet := 0.0
		if m.total > 0 {
			avgRet = m.retSum / float64(m.total) * 100
		}
		byModel = append(byModel, ModelStat{
			ModelName: name,
			WinRate:   math.Round(wr*10) / 10,
			AvgReturn: math.Round(avgRet*10) / 10,
			Count:     m.total,
		})
	}

	// BySector
	sectorMap := make(map[string]int)
	for _, r := range all {
		key := r.BkName
		if key == "" {
			key = "未知"
		}
		sectorMap[key]++
	}
	var bySector []SectorStat
	for name, cnt := range sectorMap {
		bySector = append(bySector, SectorStat{BkName: name, Count: cnt})
	}

	// DailyCount
	dayMap := make(map[string]int)
	for _, r := range all {
		if r.DataTime != nil {
			day := r.DataTime.Format("2006-01-02")
			dayMap[day]++
		}
	}
	var dailyCount []DailyCount
	for d, cnt := range dayMap {
		dailyCount = append(dailyCount, DailyCount{Date: d, Count: cnt})
	}

	return &AiRecommendStats{
		ByModel:    byModel,
		BySector:   bySector,
		DailyCount: dailyCount,
	}, nil
}

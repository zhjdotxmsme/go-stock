package data

// Kronos 滚动回测（P3）：在一段历史上按固定步长取切点，批量预测后
// 1) 统计方向命中率（预测方向 vs 切点后 HoldingDays 真实涨跌）
// 2) 按入场阈值扫描模拟交易（预测涨幅 ≥ 阈值才入场），评估信号策略价值
// 预测只做一次批量推理，阈值在缓存预测上复用评估（推理是主要成本）。

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// KronosRollingInput 滚动回测参数
type KronosRollingInput struct {
	StockCode    string
	StepDays     int     // 切点步长（交易日），默认 5
	PredLen      int     // 预测根数，默认设置值
	HoldingDays  int     // 模拟持有天数（交易日），默认 5
	StopLoss     float64 // 止损（小数，如 0.08），0=不止损
	StopProfit   float64 // 止盈（小数），0=不止盈
	MaxCutpoints int     // 切点上限，默认 24（推理成本约束）
}

// KronosRollingCut 单个切点明细
type KronosRollingCut struct {
	Date         string  `json:"date"`
	PredChange   float64 `json:"predChange"`   // 预测涨幅 %
	Direction    string  `json:"direction"`    // up/down
	Confidence   float64 `json:"confidence"`   // 一致度
	ActualReturn float64 `json:"actualReturn"` // HoldingDays 后实际涨跌 %
	DirectionHit bool    `json:"directionHit"`
}

// KronosThresholdStat 某入场阈值下的信号策略统计
type KronosThresholdStat struct {
	ThresholdPct float64 `json:"thresholdPct"` // 入场阈值（预测涨幅%）
	Signals      int     `json:"signals"`
	Wins         int     `json:"wins"`
	WinRate      float64 `json:"winRate"`
	AvgReturn    float64 `json:"avgReturn"`
	TotalReturn  float64 `json:"totalReturn"` // 复利累计
}

// KronosRollingReport 滚动回测报告
type KronosRollingReport struct {
	StockCode       string                `json:"stockCode"`
	Cutpoints       int                   `json:"cutpoints"`
	FailedCutpoints int                   `json:"failedCutpoints"`
	DirectionHits   int                   `json:"directionHits"`
	DirectionAcc    float64               `json:"directionAcc"` // 全部切点方向命中率
	BuyHoldReturn   float64               `json:"buyHoldReturn"`
	Cuts            []KronosRollingCut    `json:"cuts"`
	Thresholds      []KronosThresholdStat `json:"thresholds"` // 按胜率排序
	BestThreshold   float64               `json:"bestThreshold"`
	ElapsedSec      float64               `json:"elapsedSec"`
}

var kronosThresholdScan = []float64{0, 1, 2, 3, 5}

// RunKronosRollingBacktest 滚动回测主流程。
func RunKronosRollingBacktest(ctx context.Context, in KronosRollingInput) (*KronosRollingReport, error) {
	started := time.Now()
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return nil, ErrKronosDisabled
	}
	if err := kronosProc.EnsureRunning(cfg); err != nil {
		return nil, err
	}
	if in.StepDays <= 0 {
		in.StepDays = 5
	}
	if in.PredLen <= 0 {
		in.PredLen = orDefaultI(cfg.PredLen, 5)
	}
	if in.HoldingDays <= 0 {
		in.HoldingDays = 5
	}
	if in.MaxCutpoints <= 0 {
		in.MaxCutpoints = 24
	}
	if in.MaxCutpoints > 50 {
		in.MaxCutpoints = 50
	}

	apiCode := NormalizeKronosCode(in.StockCode)
	klineData, err := datasource.GetRouter().GetKLine(ctx, apiCode, "101", 320)
	if err != nil || klineData == nil || len(klineData.Bars) < 80 {
		return nil, ErrKronosInsuffData
	}
	bars := klineData.Bars

	// 切点：至少 60 根历史 + 之后至少 HoldingDays 根真实数据
	minHist := 60
	lastCut := len(bars) - 1 - in.HoldingDays
	if lastCut < minHist {
		return nil, fmt.Errorf("K线数量不足（%d 根），无法滚动回测", len(bars))
	}
	var cutIdx []int
	for i := minHist; i <= lastCut; i += in.StepDays {
		cutIdx = append(cutIdx, i)
	}
	// 均匀收缩到 MaxCutpoints
	for len(cutIdx) > in.MaxCutpoints {
		next := cutIdx[:0]
		for j := 0; j < len(cutIdx); j += 2 {
			next = append(next, cutIdx[j])
		}
		cutIdx = next
	}

	// 批量预测（一次 /predict_batch）
	items := make([]kronosBatchItem, 0, len(cutIdx))
	for _, ci := range cutIdx {
		hist := bars[:ci+1]
		if len(hist) > 512 {
			hist = hist[len(hist)-512:]
		}
		items = append(items, kronosBatchItem{
			Code:    hist[len(hist)-1].Time.Format("2006-01-02"),
			Candles: barsToKronosCandles(hist),
		})
	}
	preds, perrs, err := kronosPostBatch(ctx, cfg, items, in.PredLen)
	if err != nil {
		return nil, err
	}

	report := &KronosRollingReport{StockCode: apiCode, Cutpoints: len(cutIdx)}
	predByDate := map[int]*KronosPrediction{}
	for _, ci := range cutIdx {
		date := bars[ci].Time.Format("2006-01-02")
		pred := preds[date]
		if pred == nil || pred.Summary == nil {
			report.FailedCutpoints++
			if msg, ok := perrs[date]; ok {
				logger.SugaredLogger.Debugf("kronos_rolling: cut %s failed: %s", date, msg)
			}
			continue
		}
		predByDate[ci] = pred
		entry := bars[ci].Close
		endIdx := ci + in.HoldingDays
		if endIdx >= len(bars) {
			endIdx = len(bars) - 1
		}
		actualRet := (bars[endIdx].Close/entry - 1) * 100
		hit := (actualRet >= 0) == (pred.Summary.Direction == "up")
		if hit {
			report.DirectionHits++
		}
		report.Cuts = append(report.Cuts, KronosRollingCut{
			Date:         date,
			PredChange:   pred.Summary.ChangePct,
			Direction:    pred.Summary.Direction,
			Confidence:   pred.Summary.Confidence,
			ActualReturn: math.Round(actualRet*100) / 100,
			DirectionHit: hit,
		})
	}
	evaluated := len(report.Cuts)
	if evaluated == 0 {
		return nil, fmt.Errorf("所有切点预测均失败")
	}
	report.DirectionAcc = math.Round(float64(report.DirectionHits)/float64(evaluated)*1000) / 1000

	// 基准：首个切点买入持有到最后
	firstCi := cutIdx[0]
	report.BuyHoldReturn = math.Round((bars[len(bars)-1].Close/bars[firstCi].Close-1)*10000) / 100

	// 阈值扫描（复用缓存预测，不再推理）
	for _, th := range kronosThresholdScan {
		var stat KronosThresholdStat
		stat.ThresholdPct = th
		compound := 1.0
		var retSum float64
		for _, ci := range cutIdx {
			pred := predByDate[ci]
			if pred == nil || pred.Summary == nil || pred.Summary.ChangePct < th {
				continue
			}
			ret := simulateTrade(bars, ci, in.HoldingDays, in.StopLoss, in.StopProfit)
			stat.Signals++
			retSum += ret
			compound *= 1 + ret/100
			if ret > 0 {
				stat.Wins++
			}
		}
		if stat.Signals > 0 {
			stat.WinRate = math.Round(float64(stat.Wins)/float64(stat.Signals)*1000) / 1000
			stat.AvgReturn = math.Round(retSum/float64(stat.Signals)*100) / 100
			stat.TotalReturn = math.Round((compound-1)*10000) / 100
		}
		report.Thresholds = append(report.Thresholds, stat)
	}
	// 最优阈值：胜率优先，其次平均收益（信号数≥2 才参与评比）
	best := -1
	for i, s := range report.Thresholds {
		if s.Signals < 2 {
			continue
		}
		if best < 0 || s.WinRate > report.Thresholds[best].WinRate ||
			(s.WinRate == report.Thresholds[best].WinRate && s.AvgReturn > report.Thresholds[best].AvgReturn) {
			best = i
		}
	}
	if best >= 0 {
		report.BestThreshold = report.Thresholds[best].ThresholdPct
	}
	report.ElapsedSec = math.Round(time.Since(started).Seconds()*10) / 10
	sort.Slice(report.Cuts, func(i, j int) bool { return report.Cuts[i].Date < report.Cuts[j].Date })
	return report, nil
}

// simulateTrade 从 cutIdx 收盘入场，向前模拟持有：先触发止损/止盈先走，否则持有 HoldingDays。
// 返回收益率 %。
func simulateTrade(bars []datasource.KLineBar, cutIdx, holdingDays int, stopLoss, stopProfit float64) float64 {
	entry := bars[cutIdx].Close
	if entry <= 0 {
		return 0
	}
	end := cutIdx + holdingDays
	if end >= len(bars) {
		end = len(bars) - 1
	}
	for i := cutIdx + 1; i <= end; i++ {
		if stopLoss > 0 && bars[i].Low <= entry*(1-stopLoss) {
			return math.Round(-stopLoss*100*100) / 100
		}
		if stopProfit > 0 && bars[i].High >= entry*(1+stopProfit) {
			return math.Round(stopProfit*100*100) / 100
		}
	}
	return math.Round((bars[end].Close/entry-1)*10000) / 100
}

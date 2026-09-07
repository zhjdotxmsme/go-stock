package data

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/models"
)

// 全品种策略信号排行（设计规格第 3、4.2 节）。
// 信号 = 纯函数引擎（commodity_signal_engine.go）；期货标的额外取 OI/carry 象限。

type SignalBoardRow struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Market   string  `json:"market"`
	Price    float64 `json:"price"`
	Trend    string  `json:"trend,omitempty"`
	Momentum string  `json:"momentum,omitempty"`
	Breakout string  `json:"breakout,omitempty"`
	Carry    string  `json:"carry,omitempty"`
	OI       string  `json:"oi,omitempty"`
	Score    float64 `json:"score"`
	Verdict  string  `json:"verdict"`
	Failed   bool    `json:"failed"` // K 线不可得 → 信号全缺
}

type SignalBoardStats struct {
	Bull    int `json:"bull"`
	Neutral int `json:"neutral"`
	Bear    int `json:"bear"`
}

type CommoditySignalBoard struct {
	Rows      []SignalBoardRow `json:"rows"`
	Stats     SignalBoardStats `json:"stats"`
	FetchedAt string           `json:"fetchedAt"`
}

var boardCache2 = struct {
	sync.Mutex
	v  *CommoditySignalBoard
	at time.Time
}{}

const boardTTL2 = 120 * time.Second

// marketLabel 市场标签（纽约/伦敦/上海期货/沪深ETF）。
func marketLabel(a *models.CommodityAsset) string {
	switch a.AssetType {
	case models.AssetFutures:
		switch a.Exchange {
		case "SHFE":
			return "沪期货"
		case "INE":
			return "沪能期货"
		default:
			return "国内期货"
		}
	case models.AssetSpot:
		if a.Code == "XAUUSD" || a.Code == "XAGUSD" {
			return "纽约现货"
		}
		return "伦敦现货"
	case models.AssetETF:
		if a.Exchange == "SZ" {
			return "深ETF"
		}
		return "沪ETF"
	}
	return a.Exchange
}

func buildSignalRow(api *CommodityApi, panel *CommodityFuturesPanel, asset *models.CommodityAsset) SignalBoardRow {
	row := SignalBoardRow{
		Code:   asset.Code,
		Name:   asset.Name,
		Market: marketLabel(asset),
	}

	bars, err := api.GetKLine(asset.Code, "day", 260)
	if err != nil || len(bars) == 0 {
		row.Failed = true
		row.Price = 0
		row.Verdict = "无数据"
		return row
	}
	row.Price = bars[len(bars)-1].Close

	closes := make([]float64, 0, len(bars))
	hlc := make([]HighLowClose, 0, len(bars))
	for _, b := range bars {
		closes = append(closes, b.Close)
		hlc = append(hlc, HighLowClose{High: b.High, Low: b.Low, Close: b.Close})
	}

	parts := SignalParts{}
	if level, ok := ComputeTrend(closes); ok {
		s := string(level)
		parts.Trend = &s
		row.Trend = s
	}
	if level, ok := ComputeMomentum(closes); ok {
		s := string(level)
		parts.Momentum = &s
		row.Momentum = s
	}
	if level, ok := ComputeBreakout(hlc); ok {
		s := string(level)
		parts.Breakout = &s
		row.Breakout = s
	}

	// 期货标的：OI 四象限 + carry
	if asset.AssetType == models.AssetFutures && panel != nil {
		for _, c := range panel.Contracts {
			if c.Code != asset.Code {
				continue
			}
			if c.CarryOK {
				level, ok := ComputeCarry(c.NearPrice, c.FarPrice)
				if ok {
					s := string(level)
					parts.Carry = &s
					row.Carry = s
				}
			}
			if c.OIChange != nil {
				q, ok := ComputeOIQuadrant(c.ChangePct, *c.OIChange)
				if ok {
					s := string(q)
					parts.OI = &s
					row.OI = s
				}
			}
		}
	}

	score, _, err := CompositeScore(parts)
	if err != nil {
		row.Score = 0
		row.Verdict = "中性"
		return row
	}
	row.Score = score
	row.Verdict = Verdict(score)
	return row
}

func getCommodityFuturesPanelSafe() *CommodityFuturesPanel {
	p, err := GetCommodityFuturesPanel()
	if err != nil {
		return nil
	}
	return p
}

// GetCommoditySignalBoard 全品种信号排行（120s 缓存），按 |score| 降序。
func GetCommoditySignalBoard() (*CommoditySignalBoard, error) {
	boardCache2.Lock()
	if boardCache2.v != nil && time.Since(boardCache2.at) < boardTTL2 {
		boardCache2.Unlock()
		return boardCache2.v, nil
	}
	boardCache2.Unlock()

	panel := getCommodityFuturesPanelSafe()
	api := NewCommodityApi()
	assets := TradableCommodities()

	rows := make([]SignalBoardRow, 0, len(assets))
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range assets {
		wg.Add(1)
		go func(asset *models.CommodityAsset) {
			defer wg.Done()
			row := buildSignalRow(api, panel, asset)
			mu.Lock()
			rows = append(rows, row)
			mu.Unlock()
		}(&assets[i])
	}
	wg.Wait()

	sort.SliceStable(rows, func(i, j int) bool {
		if abs(rows[i].Score) != abs(rows[j].Score) {
			return abs(rows[i].Score) > abs(rows[j].Score)
		}
		return strings.ToLower(rows[i].Code) < strings.ToLower(rows[j].Code)
	})

	stats := SignalBoardStats{Neutral: len(rows)}
	for _, r := range rows {
		switch r.Verdict {
		case "偏多":
			stats.Bull++
			stats.Neutral--
		case "偏空":
			stats.Bear++
			stats.Neutral--
		}
	}

	board := &CommoditySignalBoard{
		Rows:      rows,
		Stats:     stats,
		FetchedAt: time.Now().Format(time.RFC3339),
	}
	boardCache2.Lock()
	boardCache2.v = board
	boardCache2.at = time.Now()
	boardCache2.Unlock()
	return board, nil
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// signalBoardSummary 供 AI 工具使用的文本摘要。
func signalBoardSummary(b *CommoditySignalBoard) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("市场概览：偏多 %d / 中性 %d / 偏空 %d（共 %d 个标的）\n",
		b.Stats.Bull, b.Stats.Neutral, b.Stats.Bear, len(b.Rows)))
	for _, r := range b.Rows {
		if r.Failed {
			sb.WriteString(fmt.Sprintf("- %s(%s) 数据缺失\n", r.Name, r.Market))
			continue
		}
		signals := make([]string, 0, 5)
		if r.Trend != "" {
			signals = append(signals, "趋势="+cnSignal(r.Trend, "趋势"))
		}
		if r.Momentum != "" {
			signals = append(signals, "动量="+cnSignal(r.Momentum, "动量"))
		}
		if r.Breakout != "" {
			signals = append(signals, "突破="+cnSignal(r.Breakout, "突破"))
		}
		if r.Carry != "" {
			signals = append(signals, "期限="+cnSignal(r.Carry, "期限"))
		}
		if r.OI != "" {
			signals = append(signals, "持仓="+cnSignal(r.OI, "持仓"))
		}
		sb.WriteString(fmt.Sprintf("- %s(%s) 价=%.2f 综合分=%+.3f %s | %s\n",
			r.Name, r.Market, r.Price, r.Score, r.Verdict, strings.Join(signals, " ")))
	}
	return sb.String()
}

func cnSignal(level, kind string) string {
	// 等级 → 中文简述（AI 上下文友好）
	positive := map[string]bool{
		"bull": true, "strong_bull": true, "up": true,
		"backwardation": true, "long_attack": true, "short_cover": true,
	}
	negative := map[string]bool{
		"bear": true, "strong_bear": true, "down": true,
		"contango": true, "short_attack": true, "long_exit": true,
	}
	if positive[level] {
		return kind + "偏多"
	}
	if negative[level] {
		return kind + "偏空"
	}
	return kind + "中性"
}

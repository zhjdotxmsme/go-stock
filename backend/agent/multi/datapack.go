package multi

import (
	"context"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"sync"
)

// DataPack carries data fetched once by the engine (in parallel) and shared
// by all analysts. Previously every analyst re-fetched overlapping upstreams
// independently — e.g. the daily K-line was pulled by both the technical and
// the hot-money analyst — multiplying latency and request volume before the
// LLM even started.
//
// Analysts must treat every field as optional: PrefetchDataPack never fails
// the pipeline, it only enriches. When DataPack is nil (tests, custom
// pipelines) analysts fall back to their own fetches.
type DataPack struct {
	StockCode           string
	KLineDaily          *[]data.KLineData          // daily K, last 60 bars ("101")
	TechnicalIndicators *data.IndicatorResult      // MACD/RSI/KDJ/BOLL/MA from stock-sdk MCP
	FinancialReports    *[]string                  // crawled financial report pages
	HistoryMoneyData    []models.StockMoneyDataHis // historical money flow
}

// PrefetchDataPack fetches the shared data set concurrently. Individual
// fetch failures are logged and leave the corresponding field nil; the
// function itself never returns an error so a slow/broken upstream cannot
// stall the whole analysis.
func PrefetchDataPack(ctx context.Context, stockCode string) *DataPack {
	pack := &DataPack{StockCode: stockCode}

	var wg sync.WaitGroup
	run := func(name string, fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("datapack prefetch %s panic: %v", name, r)
				}
			}()
			fn()
		}()
	}

	run("kline_daily", func() {
		pack.KLineDaily = data.NewStockDataApi().GetKLineData(stockCode, "101", 60)
	})
	run("technical_indicators", func() {
		ind, err := data.GetTechnicalIndicators(ctx, stockCode, "101", 60)
		if err == nil {
			pack.TechnicalIndicators = ind
		}
	})
	run("financial_reports", func() {
		pack.FinancialReports = data.GetFinancialReports(stockCode, 30)
	})
	run("history_money", func() {
		pack.HistoryMoneyData = data.NewStockDataApi().GetStockHistoryMoneyData(stockCode)
	})

	wg.Wait()
	logger.SugaredLogger.Infof("datapack ready for %s: kline=%v indicators=%v reports=%v moneyHis=%d",
		stockCode,
		pack.KLineDaily != nil, pack.TechnicalIndicators != nil,
		pack.FinancialReports != nil && len(*pack.FinancialReports) > 0,
		len(pack.HistoryMoneyData))
	return pack
}

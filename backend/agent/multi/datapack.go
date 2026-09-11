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
	// EtfQuote 场内基金专属快照（溢价率/真净值/份额趋势）。仅 ETF 品种填充；
	// 此时 FinancialReports 与 HistoryMoneyData 不预取（对基金无意义）。
	EtfQuote *data.EtfQuoteSnapshot
}

// PrefetchDataPack fetches the shared data set concurrently. Individual
// fetch failures are logged and leave the corresponding field nil; the
// function itself never returns an error so a slow/broken upstream cannot
// stall the whole analysis.
func PrefetchDataPack(ctx context.Context, stockCode string) *DataPack {
	pack := &DataPack{StockCode: stockCode}
	isEtf := data.InstrumentKindOf(stockCode) == data.InstrumentKindETF

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
		// ETF：个股 K 线接口对基金可能被反爬拦截，退回基金 K 线多源链（新浪→东财→腾讯）
		if isEtf && (pack.KLineDaily == nil || len(*pack.KLineDaily) == 0) {
			if r := data.NewFundKLineApi().GetFundKLineWithFallback(data.PureFundCode(stockCode), "101", 60); r != nil {
				pack.KLineDaily = r.Data
			}
		}
	})
	run("technical_indicators", func() {
		ind, err := data.GetTechnicalIndicators(ctx, stockCode, "101", 60)
		if err == nil {
			pack.TechnicalIndicators = ind
		}
	})
	if isEtf {
		// ETF：财报 F10 与个股资金流历史均不适用，改取 ETF 专属快照（溢价/净值/份额）
		run("etf_quote", func() {
			pack.EtfQuote = data.NewFundApi().GetEtfQuoteSnapshot(stockCode)
		})
	} else {
		run("financial_reports", func() {
			pack.FinancialReports = data.GetFinancialReports(stockCode, 30)
		})
		run("history_money", func() {
			pack.HistoryMoneyData = data.NewStockDataApi().GetStockHistoryMoneyData(stockCode)
		})
	}

	wg.Wait()
	logger.SugaredLogger.Infof("datapack ready for %s: kline=%v indicators=%v reports=%v moneyHis=%d etfQuote=%v",
		stockCode,
		pack.KLineDaily != nil, pack.TechnicalIndicators != nil,
		pack.FinancialReports != nil && len(*pack.FinancialReports) > 0,
		len(pack.HistoryMoneyData),
		pack.EtfQuote != nil)
	return pack
}

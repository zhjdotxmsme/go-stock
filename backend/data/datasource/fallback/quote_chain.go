package fallback

import (
	"go-stock/backend/data/datasource"
)

// The live quote chain registers exactly one provider per upstream:
//
//	mootdx  (5)  — EastMoney real-time page scrape (data.GetRealTimeStockPriceInfo)
//	tencent (10) — qt.gtimg.cn free quote API
//	yahoo   (25) — global stocks/indices/commodities, circuit-breaker gated
//
// Earlier providers (tdx/sina placeholders that always errored, and a second
// EastMoney wrapper) were removed: same-upstream retries only added latency
// and log noise. MootdxQuoteProvider and TencentQuoteProvider live in
// free_data.go.

// RegisterQuoteChain registers all quote providers with the router.
func RegisterQuoteChain(router *datasource.Router) {
	router.RegisterQuoteProvider(&MootdxQuoteProvider{})
	router.RegisterQuoteProvider(&TencentQuoteProvider{})
	router.RegisterQuoteProvider(NewYahooQuoteProvider())
}

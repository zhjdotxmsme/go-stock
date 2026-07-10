package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommodityApi_GetQuote_Spot(t *testing.T) {
	api := NewCommodityApi()
	quote, err := api.GetQuote("XAUUSD")
	require.NoError(t, err, "现货黄金行情获取失败")
	require.NotNil(t, quote)
	assert.Greater(t, quote.Price, 0.0, "价格应大于0")
	assert.Equal(t, "XAUUSD", quote.Code)
}

func TestCommodityApi_GetQuote_Futures_Domestic(t *testing.T) {
	api := NewCommodityApi()
	quote, err := api.GetQuote("AU")
	require.NoError(t, err, "沪金期货行情获取失败")
	require.NotNil(t, quote)
	assert.Greater(t, quote.Price, 0.0, "价格应大于0")
	assert.Equal(t, "AU", quote.Code)
	if quote.Extra != nil {
		assert.Contains(t, quote.Extra, "stale")
	}
}

func TestCommodityApi_GetQuote_ETF(t *testing.T) {
	api := NewCommodityApi()
	quote, err := api.GetQuote("518880")
	require.NoError(t, err, "黄金ETF行情获取失败")
	require.NotNil(t, quote)
	assert.Greater(t, quote.Price, 0.0, "价格应大于0")
	assert.Equal(t, "518880", quote.Code)
}

func TestCommodityApi_GetKLine_Spot(t *testing.T) {
	api := NewCommodityApi()
	bars, err := api.GetKLine("XAUUSD", "day", 60)
	if err != nil {
		t.Skipf("spot K-line sources unavailable in this environment: %v", err)
	}
	require.NotEmpty(t, bars, "K线数据不应为空")
	assert.Greater(t, bars[0].Close, 0.0)
}

func TestCommodityApi_GetKLine_Futures_Domestic(t *testing.T) {
	api := NewCommodityApi()
	bars, err := api.GetKLine("AU", "day", 60)
	if err != nil {
		t.Skipf("domestic futures K-line sources unavailable in this environment: %v", err)
	}
	require.NotEmpty(t, bars, "国内期货 K线数据不应为空")
	assert.Greater(t, bars[0].Close, 0.0, "国内期货价格应大于0")
}

func TestCommodityApi_GetKLine_ETF(t *testing.T) {
	api := NewCommodityApi()
	bars, err := api.GetKLine("518880", "day", 60)
	require.NoError(t, err, "黄金ETF K线获取失败")
	require.NotEmpty(t, bars, "K线数据不应为空")
	assert.Greater(t, bars[0].Close, 0.0)
}

func TestCommodityApi_GetQuote_Unknown(t *testing.T) {
	api := NewCommodityApi()
	quote, err := api.GetQuote("UNKNOWN")
	assert.Error(t, err)
	assert.Nil(t, quote)
	assert.ErrorIs(t, err, ErrCommodityNotFound)
}

func TestCommodityApi_GetQuoteIntl(t *testing.T) {
	api := NewCommodityApi()
	quote, err := api.GetQuoteIntl("AU")
	if err != nil {
		t.Skipf("international reference unavailable in this environment: %v", err)
	}
	require.NotNil(t, quote)
	assert.Greater(t, quote.Price, 0.0)
	assert.Contains(t, quote.Name, "国际参考")
}

func TestCommodityApi_GetKLineIntl(t *testing.T) {
	api := NewCommodityApi()
	bars, err := api.GetKLineIntl("AU", "day", 60)
	if err != nil {
		t.Skipf("international reference K-line unavailable: %v", err)
	}
	require.NotEmpty(t, bars)
	assert.Greater(t, bars[0].Close, 0.0)
}

func TestCommodityApi_GetQuoteIntl_NotSupported(t *testing.T) {
	api := NewCommodityApi()
	quote, err := api.GetQuoteIntl("518880")
	assert.Error(t, err)
	assert.Nil(t, quote)
}

func TestCommodityRegistry_HasInternationalRef(t *testing.T) {
	spot := FindCommodityByCode("XAUUSD")
	require.NotNil(t, spot)
	assert.Equal(t, "GC=F", spot.InternationalRef)

	fut := FindCommodityByCode("AU")
	require.NotNil(t, fut)
	assert.Equal(t, "GC=F", fut.InternationalRef)

	etf := FindCommodityByCode("518880")
	require.NotNil(t, etf)
	assert.Equal(t, "", etf.InternationalRef, "ETF 不应有国际参考")
}

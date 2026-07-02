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

func TestCommodityApi_GetQuote_Futures(t *testing.T) {
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

func TestCommodityApi_GetKLine_Futures_Unavailable(t *testing.T) {
	api := NewCommodityApi()
	bars, err := api.GetKLine("AU", "day", 60)
	assert.Error(t, err)
	assert.Empty(t, bars)
	assert.ErrorIs(t, err, ErrFuturesDataUnavailable)
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

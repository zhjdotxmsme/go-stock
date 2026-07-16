// backend/data/tests/market_data_layer_test.go
package data_test

import (
	"context"
	"testing"

	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
)

func TestMarketDataLayer_GetStockRealTimeData(t *testing.T) {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "sina",
			URL:    "http://hq.sinajs.cn",
			Method: types.MethodGet,
		},
	}

	layer := layers.NewMarketDataLayer(config)
	response, err := layer.FetchData(context.Background(), map[string]any{
		"stock_code": "sh600000",
	})

	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}

	if response.Code != 0 {
		t.Errorf("response.Code = %v, want 0", response.Code)
	}
}

func TestMarketDataLayer_FallbackMechanism(t *testing.T) {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "failing_source",
			URL:    "http://invalid.url",
			Method: types.MethodGet,
		},
		Fallbacks: []types.Endpoint{
			{
				Name:   "mock_fallback",
				URL:    "http://mock.url",
				Method: types.MethodGet,
			},
		},
		Strategy: types.FailoverStrategy,
	}

	layer := layers.NewMarketDataLayer(config)
	response, err := layer.FetchData(context.Background(), map[string]any{
		"stock_code": "sh600000",
	})

	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}

	if !response.Meta.FallbackUsed {
		t.Error("Expected fallback to be used")
	}
}

func TestMarketDataLayer_ValidateParams(t *testing.T) {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "test",
			URL:    "http://test.url",
			Method: types.MethodGet,
		},
	}

	layer := layers.NewMarketDataLayer(config)

	err := layer.ValidateParams(map[string]any{"stock_code": "sh600000"})
	if err != nil {
		t.Errorf("ValidateParams() with valid params error = %v", err)
	}

	err = layer.ValidateParams(map[string]any{})
	if err == nil {
		t.Error("ValidateParams() with missing stock_code expected error")
	}
}

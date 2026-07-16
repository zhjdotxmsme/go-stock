// backend/data/tests/capital_flow_layer_test.go
package data_test

import (
	"context"
	"testing"
	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
)

func TestCapitalFlowLayer_GetCapitalFlowData(t *testing.T) {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney",
			URL:    "http://flow.eastmoney.com",
			Method: types.MethodGet,
		},
	}

	layer := layers.NewCapitalFlowLayer(config)
	response, err := layer.FetchData(context.Background(), map[string]any{
		"stock_code": "sh600000",
		"date":       "2024-01-15",
	})

	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}

	if response.Code != 0 {
		t.Errorf("response.Code = %v, want 0", response.Code)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("response.Data should be map[string]any")
	}

	if _, hasMainFlow := data["main_flow"]; !hasMainFlow {
		t.Error("Expected main_flow field in data")
	}
}

func TestCapitalFlowLayer_ValidateParams(t *testing.T) {
	layer := layers.NewCapitalFlowLayer(&types.DataSourceConfig{})

	tests := []struct {
		name    string
		params  map[string]any
		wantErr bool
	}{
		{
			name: "valid params",
			params: map[string]any{
				"stock_code": "sh600000",
				"date":       "2024-01-15",
			},
			wantErr: false,
		},
		{
			name: "missing stock_code",
			params: map[string]any{
				"date": "2024-01-15",
			},
			wantErr: true,
		},
		{
			name: "missing date",
			params: map[string]any{
				"stock_code": "sh600000",
			},
			wantErr: true,
		},
		{
			name: "invalid date format",
			params: map[string]any{
				"stock_code": "sh600000",
				"date":       "15-01-2024",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := layer.ValidateParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCapitalFlowLayer_Fallback(t *testing.T) {
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

	layer := layers.NewCapitalFlowLayer(config)
	response, err := layer.FetchData(context.Background(), map[string]any{
		"stock_code": "sh600000",
		"date":       "2024-01-15",
	})

	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}

	if !response.Meta.FallbackUsed {
		t.Error("Expected fallback to be used")
	}
}
// backend/data/tests/announcement_layer_test.go
package data_test

import (
	"context"
	"testing"
	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
)

func TestAnnouncementLayer_GetAnnouncements(t *testing.T) {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney",
			URL:    "http://ann.eastmoney.com",
			Method: types.MethodGet,
		},
	}

	layer := layers.NewAnnouncementLayer(config)
	response, err := layer.FetchData(context.Background(), map[string]any{
		"stock_code": "sh600000",
		"days":       30,
	})

	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}

	if response.Code != 0 {
		t.Errorf("response.Code = %v, want 0", response.Code)
	}

	data, ok := response.Data.([]map[string]any)
	if !ok {
		t.Fatalf("response.Data should be []map[string]any")
	}

	if len(data) == 0 {
		t.Error("Expected at least one announcement")
	}
}

func TestAnnouncementLayer_ValidateParams(t *testing.T) {
	layer := layers.NewAnnouncementLayer(&types.DataSourceConfig{})

	tests := []struct {
		name    string
		params  map[string]any
		wantErr bool
	}{
		{
			name: "valid params",
			params: map[string]any{
				"stock_code": "sh600000",
				"days":       30,
			},
			wantErr: false,
		},
		{
			name: "missing stock_code",
			params: map[string]any{
				"days": 30,
			},
			wantErr: true,
		},
		{
			name: "invalid days - negative",
			params: map[string]any{
				"stock_code": "sh600000",
				"days":       -1,
			},
			wantErr: true,
		},
		{
			name: "invalid days - too large",
			params: map[string]any{
				"stock_code": "sh600000",
				"days":       366,
			},
			wantErr: true,
		},
		{
			name: "zero days",
			params: map[string]any{
				"stock_code": "sh600000",
				"days":       0,
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

func TestAnnouncementLayer_Fallback(t *testing.T) {
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

	layer := layers.NewAnnouncementLayer(config)
	response, err := layer.FetchData(context.Background(), map[string]any{
		"stock_code": "sh600000",
		"days":       30,
	})

	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}

	if !response.Meta.FallbackUsed {
		t.Error("Expected fallback to be used")
	}
}
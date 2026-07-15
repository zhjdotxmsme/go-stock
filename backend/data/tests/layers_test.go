// backend/data/tests/layers_test.go
package data_test

import (
	"testing"
	"time"

	"go-stock/backend/data/types"
)

func TestDataLayerInterface(t *testing.T) {
	layer := &MockDataLayer{}

	// Test GetName
	if layer.GetName() != "MockLayer" {
		t.Errorf("GetName() = %v, want MockLayer", layer.GetName())
	}

	// Test GetVersion
	if layer.GetVersion() != "1.0.0" {
		t.Errorf("GetVersion() = %v, want 1.0.0", layer.GetVersion())
	}
}

func TestEndpointStructure(t *testing.T) {
	endpoint := types.Endpoint{
		Name:    "testEndpoint",
		URL:     "http://example.com/api",
		Method:  types.MethodGet,
		Timeout: time.Second * 10,
	}

	if endpoint.Name != "testEndpoint" {
		t.Errorf("Endpoint.Name = %v, want testEndpoint", endpoint.Name)
	}
}

func TestDataSourceConfigValidation(t *testing.T) {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "primary",
			URL:    "http://primary.api",
			Method: types.MethodGet,
		},
		Fallbacks: []types.Endpoint{
			{Name: "fallback1", URL: "http://fallback1.api", Method: types.MethodGet},
		},
		Strategy: types.FailoverStrategy,
	}

	if config.Strategy != types.FailoverStrategy {
		t.Errorf("Strategy = %v, want FAILOVER", config.Strategy)
	}
}

func TestStandardizedResponseFormat(t *testing.T) {
	response := &types.StandardizedResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]any{"test": "data"},
		Meta: types.ResponseMeta{
			Source:    "testSource",
			Latency:   100,
			Cached:    false,
			Timestamp: time.Now(),
		},
	}

	if response.Code != 0 {
		t.Errorf("Code = %v, want 0", response.Code)
	}

	if response.Meta.Source != "testSource" {
		t.Errorf("Meta.Source = %v, want testSource", response.Meta.Source)
	}
}

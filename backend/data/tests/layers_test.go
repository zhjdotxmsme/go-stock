// backend/data/tests/layers_test.go
package data_test

import (
	"testing"
	"time"

	"go-stock/backend/data/layers"
	"go-stock/backend/data/utils"
)

func TestDataLayerInterface(t *testing.T) {
	layer := &layers.MockDataLayer{}

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
	endpoint := utils.Endpoint{
		Name:    "test_endpoint",
		URL:     "http://example.com/api",
		Method:  "GET",
		Timeout: time.Second * 10,
	}

	if endpoint.Name != "test_endpoint" {
		t.Errorf("Endpoint.Name = %v, want test_endpoint", endpoint.Name)
	}
}

func TestDataSourceConfigValidation(t *testing.T) {
	config := &utils.DataSourceConfig{
		Primary: utils.Endpoint{
			Name:   "primary",
			URL:    "http://primary.api",
			Method: "GET",
		},
		Fallbacks: []utils.Endpoint{
			{Name: "fallback1", URL: "http://fallback1.api", Method: "GET"},
		},
		Strategy: utils.FailoverStrategy,
	}

	if config.Strategy != utils.FailoverStrategy {
		t.Errorf("Strategy = %v, want FAILOVER", config.Strategy)
	}
}

func TestStandardizedResponseFormat(t *testing.T) {
	response := &utils.StandardizedResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"test": "data"},
		Meta: utils.ResponseMeta{
			Source:    "test_source",
			Latency:   100,
			Cached:    false,
			Timestamp: time.Now(),
		},
	}

	if response.Code != 0 {
		t.Errorf("Code = %v, want 0", response.Code)
	}

	if response.Meta.Source != "test_source" {
		t.Errorf("Meta.Source = %v, want test_source", response.Meta.Source)
	}
}

// backend/data/layers/base_layer.go
package layers

import (
	"context"
	"time"

	"go-stock/backend/data/utils"
)

// DataLayer defines the interface for all data layers
type DataLayer interface {
	GetName() string
	GetVersion() string
	GetEndpoints() []utils.Endpoint
	GetFallbackEndpoints() []utils.Endpoint
	FetchData(ctx context.Context, params map[string]any) (*utils.StandardizedResponse, error)
	ValidateParams(params map[string]any) error
}

// MockDataLayer implements DataLayer for testing
type MockDataLayer struct {
	name    string
	version string
}

// NewMockDataLayer creates a new MockDataLayer instance
func NewMockDataLayer() *MockDataLayer {
	return &MockDataLayer{
		name:    "MockLayer",
		version: "1.0.0",
	}
}

func (m *MockDataLayer) GetName() string {
	if m.name == "" {
		return "MockLayer"
	}
	return m.name
}

func (m *MockDataLayer) GetVersion() string {
	if m.version == "" {
		return "1.0.0"
	}
	return m.version
}

func (m *MockDataLayer) GetEndpoints() []utils.Endpoint {
	return []utils.Endpoint{
		{
			Name:   "mock_endpoint",
			URL:    "http://mock.api",
			Method: "GET",
		},
	}
}

func (m *MockDataLayer) GetFallbackEndpoints() []utils.Endpoint {
	return []utils.Endpoint{}
}

func (m *MockDataLayer) FetchData(ctx context.Context, params map[string]any) (*utils.StandardizedResponse, error) {
	return &utils.StandardizedResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"mock": "data"},
		Meta: utils.ResponseMeta{
			Source:    m.GetName(),
			Timestamp: time.Now(),
		},
	}, nil
}

func (m *MockDataLayer) ValidateParams(params map[string]any) error {
	return nil
}

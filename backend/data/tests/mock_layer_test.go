// backend/data/tests/mock_layer_test.go
package data_test

import (
	"context"
	"errors"
	"time"

	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
)

// MockDataLayer implements layers.DataLayer for testing.
type MockDataLayer struct {
	name    string
	version string
}

// NewMockDataLayer creates a new MockDataLayer instance.
func NewMockDataLayer() *MockDataLayer {
	return &MockDataLayer{
		name:    "MockLayer",
		version: "1.0.0",
	}
}

// Compile-time assertion that MockDataLayer implements layers.DataLayer.
var _ layers.DataLayer = (*MockDataLayer)(nil)

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

func (m *MockDataLayer) GetEndpoints() []types.Endpoint {
	return []types.Endpoint{
		{
			Name:   "mockEndpoint",
			URL:    "http://mock.api",
			Method: types.MethodGet,
		},
	}
}

func (m *MockDataLayer) GetFallbackEndpoints() []types.Endpoint {
	return []types.Endpoint{}
}

func (m *MockDataLayer) FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error) {
	return &types.StandardizedResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]any{"mock": "data"},
		Meta: types.ResponseMeta{
			Source:    m.GetName(),
			Timestamp: time.Now(),
		},
	}, nil
}

// ValidateParams validates request parameters.
func (m *MockDataLayer) ValidateParams(params map[string]any) error {
	if params == nil {
		return errors.New("params cannot be nil")
	}
	return nil
}

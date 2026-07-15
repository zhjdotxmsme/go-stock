// backend/data/layers/base_layer.go
package layers

import (
	"context"

	"go-stock/backend/data/types"
)

// DataLayer defines the interface for all data layers.
type DataLayer interface {
	// GetName returns the layer name.
	GetName() string
	// GetVersion returns the layer version.
	GetVersion() string
	// GetEndpoints returns the primary endpoints.
	GetEndpoints() []types.Endpoint
	// GetFallbackEndpoints returns fallback endpoints.
	GetFallbackEndpoints() []types.Endpoint
	// FetchData retrieves data from the layer using the provided params.
	FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error)
	// ValidateParams validates request parameters.
	ValidateParams(params map[string]any) error
}

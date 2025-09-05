package query

import (
	"context"
	"log/slog"
	"strings"
)

// MockEngine is a mock implementation for testing
type MockEngine struct {
	products []Product
	err      error
	log      *slog.Logger
}

// NewMockEngine creates a new mock engine for testing
func NewMockEngine(logger *slog.Logger) *MockEngine {
	return &MockEngine{
		log: logger,
		products: []Product{
			{
				Code:        "3017620422003",
				ProductName: "Nutella",
				Brands:      "Ferrero",
				Nutriments: map[string]interface{}{
					"energy":        2255,
					"fat":           30.9,
					"saturated-fat": 10.6,
					"carbohydrates": 57.5,
					"sugars":        56.3,
					"proteins":      6.3,
					"salt":          0.107,
				},
				Link:        "https://world.openfoodfacts.org/product/3017620422003/nutella-ferrero",
				Ingredients: map[string]interface{}{"text": "sugar, hazelnuts, palm oil, cocoa, milk powder"},
			},
			{
				Code:        "1234567890123",
				ProductName: "Test Chocolate",
				Brands:      "Ferrero",
				Nutriments: map[string]interface{}{
					"energy": 2000,
					"fat":    25.0,
				},
				Link:        "https://example.com/test-chocolate",
				Ingredients: map[string]interface{}{"text": "cocoa, sugar"},
			},
		},
	}
}

// SearchProductsByBrandAndName searches for products by name and brand
func (m *MockEngine) SearchProductsByBrandAndName(ctx context.Context, name, brand string, limit int) ([]Product, error) {
	if m.err != nil {
		return nil, m.err
	}

	var results []Product
	for _, product := range m.products {
		if name != "" && !contains(product.ProductName, name) {
			continue
		}
		if brand != "" && !contains(product.Brands, brand) {
			continue
		}
		results = append(results, product)
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// SearchByBarcode searches for a product by barcode
func (m *MockEngine) SearchByBarcode(ctx context.Context, barcode string) (*Product, error) {
	if m.err != nil {
		return nil, m.err
	}

	for _, product := range m.products {
		if product.Code == barcode {
			return &product, nil
		}
	}

	return nil, nil
}

// TestConnection tests the connection (always succeeds for mock)
func (m *MockEngine) TestConnection(ctx context.Context) error {
	return m.err
}

// Close closes the mock engine (no-op)
func (m *MockEngine) Close() error {
	return nil
}

// SetError sets an error to be returned by the mock
func (m *MockEngine) SetError(err error) {
	m.err = err
}

// SetProducts sets the products to be returned by the mock
func (m *MockEngine) SetProducts(products []Product) {
	m.products = products
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

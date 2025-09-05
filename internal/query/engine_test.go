package query

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	engine, err := NewEngine("/nonexistent/path.parquet", logger)
	assert.NoError(t, err)
	assert.NotNil(t, engine)

	defer engine.Close()
}

func TestProduct_JSONSerialization(t *testing.T) {
	product := Product{
		Code:        "12345",
		ProductName: "Test Product",
		Brands:      "Test Brand",
		Nutriments: map[string]interface{}{
			"energy": 100,
			"fat":    5.5,
		},
		Link:        "https://example.com/product/12345",
		Ingredients: map[string]interface{}{"text": "sugar, water"},
	}

	// Test that all fields are properly accessible
	assert.Equal(t, "12345", product.Code)
	assert.Equal(t, "Test Product", product.ProductName)
	assert.Equal(t, "Test Brand", product.Brands)
	assert.Equal(t, "https://example.com/product/12345", product.Link)
	assert.NotNil(t, product.Nutriments)
	assert.NotNil(t, product.Ingredients)

	// Test nutriments access
	energy, ok := product.Nutriments["energy"]
	assert.True(t, ok)
	assert.Equal(t, 100, energy)
}

// Integration test would require a real parquet file
// This is a placeholder for when we have test data
func TestEngine_SearchProducts_Integration(t *testing.T) {
	t.Skip("Integration test - requires actual parquet file")

	// This test would:
	// 1. Create a small test parquet file with known data
	// 2. Initialize the engine with that file
	// 3. Test various search scenarios
	// 4. Verify results match expected data
}

func TestEngine_SearchByBarcode_Integration(t *testing.T) {
	t.Skip("Integration test - requires actual parquet file")

	// This test would:
	// 1. Create a small test parquet file with known barcodes
	// 2. Initialize the engine with that file
	// 3. Test barcode searches
	// 4. Verify exact match behavior
}

func TestEngine_TestConnection_WithInvalidFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	engine, err := NewEngine("/nonexistent/file.parquet", logger)
	assert.NoError(t, err)
	defer engine.Close()

	ctx := context.Background()
	err = engine.TestConnection(ctx)
	assert.Error(t, err, "Should fail with nonexistent file")
}

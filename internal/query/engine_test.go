package query

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewMockEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	engine := NewMockEngine(logger)
	assert.NotNil(t, engine)

	defer engine.Close()
}

func TestMockEngine_SearchProductsByBrandAndName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := NewMockEngine(logger)
	defer engine.Close()

	ctx := context.Background()

	// Test search by name
	products, err := engine.SearchProductsByBrandAndName(ctx, "Nutella", "", 10)
	assert.NoError(t, err)
	assert.Len(t, products, 1)
	assert.Equal(t, "Nutella", products[0].ProductName)
	assert.Equal(t, "3017620422003", products[0].Code)

	// Test search by brand
	products, err = engine.SearchProductsByBrandAndName(ctx, "", "Ferrero", 10)
	assert.NoError(t, err)
	assert.Len(t, products, 2) // Both products are Ferrero

	// Test combined search
	products, err = engine.SearchProductsByBrandAndName(ctx, "chocolate", "Ferrero", 10)
	assert.NoError(t, err)
	assert.Len(t, products, 1)
	assert.Equal(t, "Test Chocolate", products[0].ProductName)

	// Test limit
	products, err = engine.SearchProductsByBrandAndName(ctx, "", "Ferrero", 1)
	assert.NoError(t, err)
	assert.Len(t, products, 1)
}

func TestMockEngine_SearchByBarcode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := NewMockEngine(logger)
	defer engine.Close()

	ctx := context.Background()

	// Test existing barcode
	product, err := engine.SearchByBarcode(ctx, "3017620422003")
	assert.NoError(t, err)
	assert.NotNil(t, product)
	assert.Equal(t, "Nutella", product.ProductName)

	// Test non-existing barcode
	product, err = engine.SearchByBarcode(ctx, "9999999999999")
	assert.NoError(t, err)
	assert.Nil(t, product)
}

func TestMockEngine_TestConnection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := NewMockEngine(logger)
	defer engine.Close()

	ctx := context.Background()

	// Test successful connection
	err := engine.TestConnection(ctx)
	assert.NoError(t, err)

	// Test connection with error
	engine.SetError(assert.AnError)
	err = engine.TestConnection(ctx)
	assert.Error(t, err)
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
func TestEngine_SearchProductsByBrandAndName_Integration(t *testing.T) {
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

	// Create a minimal config for testing
	cfg := &config.Config{
		DuckDBMemoryLimit:            "1GB",
		DuckDBThreads:                2,
		DuckDBCheckpointThreshold:    "512MB",
		DuckDBPreserveInsertionOrder: true,
	}

	engine, err := NewEngine("/nonexistent/file.parquet", cfg, logger)
	assert.NoError(t, err)
	defer engine.Close()

	ctx := context.Background()
	err = engine.TestConnection(ctx)
	assert.Error(t, err, "Should fail with nonexistent file")
}

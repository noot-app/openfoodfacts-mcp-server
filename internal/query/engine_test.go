package query

import (
	"context"
	"database/sql"
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

func TestSearchProductsByBrandAndName(t *testing.T) {
	tests := []struct {
		name        string
		brand       string
		productName string
		expectError bool
		minResults  int
	}{
		{
			name:        "valid search with known brand",
			brand:       "Coca",
			productName: "Cola",
			expectError: false,
			minResults:  1,
		},
		{
			name:        "empty brand and product name",
			brand:       "",
			productName: "",
			expectError: false,
			minResults:  0,
		},
		{
			name:        "non-existent brand",
			brand:       "NonExistentBrand123456",
			productName: "NonExistentProduct123456",
			expectError: false,
			minResults:  0,
		},
	}

	// Skip test if no database file
	if _, err := os.Stat("../../data/product-database.parquet"); os.IsNotExist(err) {
		t.Skip("Database file not found, skipping integration test")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := &config.Config{}
	engine, err := NewEngine("../../data/product-database.parquet", cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results, err := engine.SearchProductsByBrandAndName(ctx, tt.productName, tt.brand, 10)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, len(results), tt.minResults)
			}
		})
	}
}

func TestConvertPythonListToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "[]",
		},
		{
			name:     "simple quoted values",
			input:    "[{'name': 'sodium', 'value': 10.0}]",
			expected: `[{"name": "sodium", "value": 10.0}]`,
		},
		{
			name:     "unquoted string values",
			input:    "[{'name': sodium, 'unit': mg}]",
			expected: `[{"name": "sodium", "unit": "mg"}]`,
		},
		{
			name:     "NULL values",
			input:    "[{'name': 'sodium', 'value': NULL, 'serving': None}]",
			expected: `[{"name": "sodium", "value": null, "serving": null}]`,
		},
		{
			name:     "none values (lowercase)",
			input:    "[{'name': 'sodium', 'value': none, 'serving': null}]",
			expected: `[{"name": "sodium", "value": null, "serving": null}]`,
		},
		{
			name:     "complex field names with dashes",
			input:    "[{'name': fruits-vegetables-nuts-estimate, 'unit': percent}]",
			expected: `[{"name": "fruits-vegetables-nuts-estimate", "unit": "percent"}]`,
		},
		{
			name:     "units with spaces and special chars",
			input:    "[{'name': alcohol, 'unit': % vol}]",
			expected: `[{"name": "alcohol", "unit": "% vol"}]`,
		},
		{
			name:     "mixed data types",
			input:    "[{'name': energy, 'value': 1234, 'unit': kcal, 'per_100g': 1234.0, 'serving': NULL}]",
			expected: `[{"name": "energy", "value": 1234, "unit": "kcal", "per_100g": 1234.0, "serving": null}]`,
		},
		{
			name:     "real world example",
			input:    "[{'name': saturated-fat, 'value': 10.0, '100g': 10.0, 'serving': NULL, 'unit': g, 'prepared_value': NULL, 'prepared_100g': NULL, 'prepared_serving': NULL, 'prepared_unit': NULL}]",
			expected: `[{"name": "saturated-fat", "value": 10.0, "100g": 10.0, "serving": null, "unit": "g", "prepared_value": null, "prepared_100g": null, "prepared_serving": null, "prepared_unit": null}]`,
		},
		{
			name:     "multiple nutrients",
			input:    "[{'name': sodium, 'value': 50, 'unit': mg}, {'name': energy, 'value': 200, 'unit': kcal}]",
			expected: `[{"name": "sodium", "value": 50, "unit": "mg"}, {"name": "energy", "value": 200, "unit": "kcal"}]`,
		},
		{
			name:     "edge case with underscore and percent",
			input:    "[{'name': saturated_fat, 'unit': % DV}]",
			expected: `[{"name": "saturated_fat", "unit": "% DV"}]`,
		},
		{
			name:     "None at start of array",
			input:    "[None, {'name': sodium, 'value': 10}]",
			expected: `[null, {"name": "sodium", "value": 10}]`,
		},
		{
			name:     "None at end of array",
			input:    "[{'name': sodium, 'value': 10}, None]",
			expected: `[{"name": "sodium", "value": 10}, null]`,
		},
		{
			name:     "nested None values",
			input:    "[{'nested': [None, NULL, none]}]",
			expected: `[{"nested": [null, null, null]}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPythonListToJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseNutrimentsJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := &config.Config{}
	engine, err := NewEngine("test.parquet", cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	tests := []struct {
		name     string
		input    sql.NullString
		expected map[string]interface{}
	}{
		{
			name:     "null input",
			input:    sql.NullString{Valid: false},
			expected: map[string]interface{}{},
		},
		{
			name:     "empty string",
			input:    sql.NullString{String: "", Valid: true},
			expected: map[string]interface{}{},
		},
		{
			name:  "single nutrient",
			input: sql.NullString{String: "[{'name': 'sodium', 'value': 10.0, 'unit': 'mg'}]", Valid: true},
			expected: map[string]interface{}{
				"sodium": map[string]interface{}{
					"name":  "sodium",
					"value": 10.0,
					"unit":  "mg",
				},
			},
		},
		{
			name:  "unquoted values",
			input: sql.NullString{String: "[{'name': sodium, 'value': 10.0, 'unit': mg}]", Valid: true},
			expected: map[string]interface{}{
				"sodium": map[string]interface{}{
					"name":  "sodium",
					"value": 10.0,
					"unit":  "mg",
				},
			},
		},
		{
			name:  "multiple nutrients",
			input: sql.NullString{String: "[{'name': sodium, 'value': 50, 'unit': mg}, {'name': energy, 'value': 200, 'unit': kcal}]", Valid: true},
			expected: map[string]interface{}{
				"sodium": map[string]interface{}{
					"name":  "sodium",
					"value": 50.0, // JSON unmarshaling converts to float64
					"unit":  "mg",
				},
				"energy": map[string]interface{}{
					"name":  "energy",
					"value": 200.0,
					"unit":  "kcal",
				},
			},
		},
		{
			name:  "null values",
			input: sql.NullString{String: "[{'name': sodium, 'value': NULL, 'serving': None, 'unit': mg}]", Valid: true},
			expected: map[string]interface{}{
				"sodium": map[string]interface{}{
					"name":    "sodium",
					"value":   nil,
					"serving": nil,
					"unit":    "mg",
				},
			},
		},
		{
			name:  "complex field names",
			input: sql.NullString{String: "[{'name': fruits-vegetables-nuts-estimate, 'value': 25.5, 'unit': percent}]", Valid: true},
			expected: map[string]interface{}{
				"fruits-vegetables-nuts-estimate": map[string]interface{}{
					"name":  "fruits-vegetables-nuts-estimate",
					"value": 25.5,
					"unit":  "percent",
				},
			},
		},
		{
			name:     "invalid JSON - should return empty map",
			input:    sql.NullString{String: "invalid json", Valid: true},
			expected: map[string]interface{}{},
		},
		{
			name:  "nutrient without name - should be ignored",
			input: sql.NullString{String: "[{'value': 10.0, 'unit': 'mg'}, {'name': 'sodium', 'value': 20.0}]", Valid: true},
			expected: map[string]interface{}{
				"sodium": map[string]interface{}{
					"name":  "sodium",
					"value": 20.0,
				},
			},
		},
		{
			name:  "nutrient with empty name - should be ignored",
			input: sql.NullString{String: "[{'name': '', 'value': 10.0}, {'name': 'sodium', 'value': 20.0}]", Valid: true},
			expected: map[string]interface{}{
				"sodium": map[string]interface{}{
					"name":  "sodium",
					"value": 20.0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.parseNutrimentsJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertPythonListToJSON_PerformanceBenchmark tests parsing performance with large datasets
func BenchmarkConvertPythonListToJSON(b *testing.B) {
	// Real-world large nutriment data example
	largeData := "[{'name': 'energy', 'value': 1234, 'unit': 'kcal', '100g': 1234, 'serving': 309}, " +
		"{'name': 'fat', 'value': 12.3, 'unit': 'g', '100g': 12.3, 'serving': 3.08}, " +
		"{'name': 'saturated-fat', 'value': 10.0, 'unit': 'g', '100g': 10.0, 'serving': NULL}, " +
		"{'name': 'trans-fat', 'value': NULL, 'unit': 'g', '100g': NULL, 'serving': NULL}, " +
		"{'name': 'cholesterol', 'value': 0, 'unit': 'mg', '100g': 0, 'serving': 0}, " +
		"{'name': 'sodium', 'value': 50, 'unit': 'mg', '100g': 50, 'serving': 12.5}, " +
		"{'name': 'carbohydrates', 'value': 200, 'unit': 'g', '100g': 200, 'serving': 50}, " +
		"{'name': 'fiber', 'value': 25, 'unit': 'g', '100g': 25, 'serving': 6.25}, " +
		"{'name': 'sugars', 'value': 150, 'unit': 'g', '100g': 150, 'serving': 37.5}, " +
		"{'name': 'proteins', 'value': 8, 'unit': 'g', '100g': 8, 'serving': 2}]"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		convertPythonListToJSON(largeData)
	}
}

// TestParseNutrimentsJSON_PerformanceBenchmark tests overall parsing performance
func BenchmarkParseNutrimentsJSON(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{}
	engine, err := NewEngine("test.parquet", cfg, logger)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	largeData := sql.NullString{
		String: "[{'name': energy, 'value': 1234, 'unit': kcal}, " +
			"{'name': fat, 'value': 12.3, 'unit': g}, " +
			"{'name': saturated-fat, 'value': 10.0, 'unit': g}, " +
			"{'name': sodium, 'value': 50, 'unit': mg}, " +
			"{'name': carbohydrates, 'value': 200, 'unit': g}]",
		Valid: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.parseNutrimentsJSON(largeData)
	}
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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
)

// Tools implements MCP tools for OpenFoodFacts product search
type Tools struct {
	queryEngine query.QueryEngine
	log         *slog.Logger
}

// NewTools creates a new MCP tools instance
func NewTools(queryEngine query.QueryEngine, logger *slog.Logger) *Tools {
	return &Tools{
		queryEngine: queryEngine,
		log:         logger,
	}
}

// SearchProductsByBrandAndNameArgs represents arguments for product search
type SearchProductsByBrandAndNameArgs struct {
	Name  string `json:"name" description:"Product name to search for"`
	Brand string `json:"brand" description:"Brand name to filter by (optional)"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of results to return (default: 10, max: 100)"`
}

// SearchByBarcodeArgs represents arguments for barcode search
type SearchByBarcodeArgs struct {
	Barcode string `json:"barcode" description:"Product barcode (UPC/EAN) to search for"`
}

// SearchProductsByBrandAndName searches for products by name and optional brand
func (t *Tools) SearchProductsByBrandAndName(ctx context.Context, req *mcp.CallToolRequest, args SearchProductsByBrandAndNameArgs) (*mcp.CallToolResult, any, error) {
	// Validate arguments
	if args.Name == "" {
		return nil, nil, fmt.Errorf("name parameter is required")
	}

	// Set default limit and validate
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	t.log.Debug("MCP SearchProductsByBrandAndName called",
		"name", args.Name,
		"brand", args.Brand,
		"limit", args.Limit)

	// Execute search
	products, err := t.queryEngine.SearchProductsByBrandAndName(ctx, args.Name, args.Brand, args.Limit)
	if err != nil {
		t.log.Error("Product search failed", "error", err)
		return nil, nil, fmt.Errorf("search failed: %w", err)
	}

	// Prepare response
	response := map[string]interface{}{
		"found":    len(products) > 0,
		"count":    len(products),
		"products": products,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil, nil
}

// SearchByBarcode searches for a product by its barcode
func (t *Tools) SearchByBarcode(ctx context.Context, req *mcp.CallToolRequest, args SearchByBarcodeArgs) (*mcp.CallToolResult, any, error) {
	// Validate arguments
	if args.Barcode == "" {
		return nil, nil, fmt.Errorf("barcode parameter is required")
	}

	t.log.Debug("MCP SearchByBarcode called", "barcode", args.Barcode)

	// Execute search
	product, err := t.queryEngine.SearchByBarcode(ctx, args.Barcode)
	if err != nil {
		t.log.Error("Barcode search failed", "error", err)
		return nil, nil, fmt.Errorf("barcode search failed: %w", err)
	}

	// Prepare response
	response := map[string]interface{}{
		"found": product != nil,
	}
	if product != nil {
		response["product"] = product
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil, nil
}

// GetNutritionAnalysisArgs represents arguments for nutrition analysis
type GetNutritionAnalysisArgs struct {
	ProductCode string `json:"product_code" description:"Product code to analyze nutrition for"`
}

// GetNutritionAnalysis provides nutrition analysis for a product
func (t *Tools) GetNutritionAnalysis(ctx context.Context, req *mcp.CallToolRequest, args GetNutritionAnalysisArgs) (*mcp.CallToolResult, any, error) {
	// Validate arguments
	if args.ProductCode == "" {
		return nil, nil, fmt.Errorf("product_code parameter is required")
	}

	t.log.Debug("MCP GetNutritionAnalysis called", "product_code", args.ProductCode)

	// Search for product by barcode
	product, err := t.queryEngine.SearchByBarcode(ctx, args.ProductCode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find product: %w", err)
	}

	if product == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Product with code %s not found", args.ProductCode),
				},
			},
		}, nil, nil
	}

	// Analyze nutrition data
	analysis := analyzeNutrition(product)

	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal analysis: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(analysisJSON),
			},
		},
	}, nil, nil
}

// analyzeNutrition performs basic nutrition analysis
func analyzeNutrition(product *query.Product) map[string]interface{} {
	analysis := map[string]interface{}{
		"product_name":        product.ProductName,
		"brands":              product.Brands,
		"code":                product.Code,
		"nutrition_available": len(product.Nutriments) > 0,
	}

	if len(product.Nutriments) == 0 {
		analysis["message"] = "No nutrition data available for this product"
		return analysis
	}

	// Extract key nutrition metrics
	nutrition := make(map[string]interface{})

	// Common nutrition fields
	nutritionFields := map[string]string{
		"energy-kcal_100g":   "calories_per_100g",
		"energy-kj_100g":     "energy_kj_per_100g",
		"proteins_100g":      "proteins_per_100g",
		"carbohydrates_100g": "carbohydrates_per_100g",
		"sugars_100g":        "sugars_per_100g",
		"fat_100g":           "fat_per_100g",
		"saturated-fat_100g": "saturated_fat_per_100g",
		"fiber_100g":         "fiber_per_100g",
		"sodium_100g":        "sodium_per_100g",
		"salt_100g":          "salt_per_100g",
	}

	for originalKey, friendlyKey := range nutritionFields {
		if value, exists := product.Nutriments[originalKey]; exists {
			nutrition[friendlyKey] = value
		}
	}

	analysis["nutrition"] = nutrition

	// Add basic health insights
	insights := []string{}

	if calories, ok := product.Nutriments["energy-kcal_100g"]; ok {
		if calorieFloat, err := parseFloat(calories); err == nil {
			if calorieFloat > 400 {
				insights = append(insights, "High calorie content")
			} else if calorieFloat < 100 {
				insights = append(insights, "Low calorie content")
			}
		}
	}

	if sugars, ok := product.Nutriments["sugars_100g"]; ok {
		if sugarFloat, err := parseFloat(sugars); err == nil {
			if sugarFloat > 15 {
				insights = append(insights, "High sugar content")
			} else if sugarFloat < 5 {
				insights = append(insights, "Low sugar content")
			}
		}
	}

	if fat, ok := product.Nutriments["fat_100g"]; ok {
		if fatFloat, err := parseFloat(fat); err == nil {
			if fatFloat > 20 {
				insights = append(insights, "High fat content")
			} else if fatFloat < 3 {
				insights = append(insights, "Low fat content")
			}
		}
	}

	if len(insights) > 0 {
		analysis["health_insights"] = insights
	}

	return analysis
}

// parseFloat safely converts interface{} to float64
func parseFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

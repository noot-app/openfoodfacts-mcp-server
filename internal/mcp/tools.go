package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

package mcpgo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/auth"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/types"
)

// Server wraps the mark3labs MCP server with authentication
type Server struct {
	mcpServer   *server.MCPServer
	queryEngine query.QueryEngine
	auth        *auth.BearerTokenAuth
	log         *slog.Logger
}

// SearchProductsResponse represents the response from search_products_by_brand_and_name
type SearchProductsResponse struct {
	Found    bool            `json:"found"`
	Count    int             `json:"count"`
	Products []types.Product `json:"products"`
}

// SearchBarcodeResponse represents the response from search_by_barcode
type SearchBarcodeResponse struct {
	Found   bool           `json:"found"`
	Product *types.Product `json:"product,omitempty"`
}

// NewServer creates a new MCP server with the mark3labs SDK
func NewServer(queryEngine query.QueryEngine, authenticator *auth.BearerTokenAuth, logger *slog.Logger) *Server {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"OpenFoodFacts MCP Server",
		"1.0.0",
		server.WithToolCapabilities(false), // Tools don't change dynamically
		server.WithRecovery(),              // Recover from panics
		server.WithLogging(),               // Enable logging
	)

	s := &Server{
		mcpServer:   mcpServer,
		queryEngine: queryEngine,
		auth:        authenticator,
		log:         logger,
	}

	// Add tools
	s.addTools()

	return s
}

func (s *Server) addTools() {
	// Search products by brand and name tool
	searchTool := mcp.NewTool("search_products_by_brand_and_name",
		mcp.WithDescription("Search for branded products by their brand and product name. This tool can only be used if brand and product name are both provided and non-empty."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.MinLength(1), // must be at least 1 char
			mcp.Description("Product name to search for. Required and must be a non-empty string."),
		),
		mcp.WithString("brand",
			mcp.Required(),
			mcp.MinLength(1), // must be at least 1 char
			mcp.Description("Brand name to search for. Required and must be a non-empty string."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (default: 3, max: 10)"),
			mcp.DefaultNumber(3),
			mcp.Min(1),
			mcp.Max(10),
		),
		mcp.WithOutputSchema[SearchProductsResponse](),
		mcp.WithIdempotentHintAnnotation(true),
	)

	s.mcpServer.AddTool(searchTool, s.handleSearchProducts)

	// Search by barcode tool
	barcodeTool := mcp.NewTool("search_by_barcode",
		mcp.WithDescription("Search for a product by its barcode (UPC/EAN)"),
		mcp.WithString("barcode",
			mcp.Required(),
			mcp.Description("The barcode (UPC/EAN) to search for"),
		),
		mcp.WithOutputSchema[SearchBarcodeResponse](),
		mcp.WithIdempotentHintAnnotation(true),
	)

	s.mcpServer.AddTool(barcodeTool, s.handleSearchByBarcode)
}

func (s *Server) handleSearchProducts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'name': %v", err)), nil
	}

	brand, err := request.RequireString("brand") // Required parameter
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'brand': %v", err)), nil
	}

	// Validate minimum lengths
	if len(name) < 1 {
		return mcp.NewToolResultError("Parameter 'name' must be at least 1 character long"), nil
	}
	if len(brand) < 1 {
		return mcp.NewToolResultError("Parameter 'brand' must be at least 1 character long"), nil
	}

	limitFloat := request.GetFloat("limit", 3.0)
	limit := int(limitFloat)
	if limit <= 0 {
		limit = 3
	}
	if limit > 10 {
		limit = 10
	}

	s.log.Debug("MCP SearchProductsByBrandAndName called",
		"name", name,
		"brand", brand,
		"limit", limit)

	// Execute search
	products, err := s.queryEngine.SearchProductsByBrandAndName(ctx, name, brand, limit)
	if err != nil {
		s.log.Error("Product search failed", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	// Prepare response
	response := map[string]interface{}{
		"found":    len(products) > 0,
		"count":    len(products),
		"products": products,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}

func (s *Server) handleSearchByBarcode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments
	barcode, err := request.RequireString("barcode")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'barcode': %v", err)), nil
	}

	s.log.Debug("MCP SearchByBarcode called", "barcode", barcode)

	// Execute search
	product, err := s.queryEngine.SearchByBarcode(ctx, barcode)
	if err != nil {
		s.log.Error("Barcode search failed", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Barcode search failed: %v", err)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}

// ServeHTTP serves the MCP server over HTTP with authentication
func (s *Server) ServeHTTP(addr string) error {
	// Create a custom HTTP handler that includes authentication
	mux := http.NewServeMux()

	// Health endpoint (no auth required)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Test database connection
		ctx := r.Context()
		if err := s.queryEngine.TestConnection(ctx); err != nil {
			s.log.Error("Health check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
		})
	})

	// Create the streamable HTTP server
	streamableServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true), // Stateless for better OpenAI compatibility
	)

	// MCP endpoint with authentication
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// Check authentication for all non-health endpoints
		if !s.auth.IsAuthorized(r) {
			s.auth.SetUnauthorizedHeaders(w)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			s.log.Warn("Unauthorized MCP request", "remote_addr", r.RemoteAddr, "user_agent", r.UserAgent())
			return
		}

		// Forward to the streamable HTTP server
		streamableServer.ServeHTTP(w, r)
	})

	s.log.Info("Starting MCP server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

// ServeStdio serves the MCP server over stdio (no auth required for local use)
func (s *Server) ServeStdio() error {
	s.log.Info("Starting MCP server in stdio mode")
	return server.ServeStdio(s.mcpServer)
}

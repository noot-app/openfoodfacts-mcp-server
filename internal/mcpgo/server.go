package mcpgo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/auth"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/types"
)

// responseRecorder wraps http.ResponseWriter to capture response details
type responseRecorder struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int
	headerWritten bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.headerWritten {
		return // Prevent duplicate WriteHeader calls
	}
	r.statusCode = code
	r.headerWritten = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytesWritten += n
	return n, err
}

// Server wraps the mark3labs MCP server with authentication
type Server struct {
	mcpServer   *server.MCPServer
	queryEngine query.QueryEngine
	auth        *auth.BearerTokenAuth
	log         *slog.Logger

	// Health check caching to prevent DOS attacks
	healthMu        sync.RWMutex
	lastHealthCheck time.Time
	lastHealthError error
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

// SearchProductsSimplifiedResponse represents the simplified response from search_products_by_brand_and_name_simplified
type SearchProductsSimplifiedResponse struct {
	Found    bool                      `json:"found"`
	Count    int                       `json:"count"`
	Products []types.SimplifiedProduct `json:"products"`
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

// checkHealthWithCache checks health with 10-second caching to prevent DOS attacks
func (s *Server) checkHealthWithCache(ctx context.Context) error {
	const cacheDuration = 10 * time.Second

	s.healthMu.RLock()
	if time.Since(s.lastHealthCheck) < cacheDuration {
		err := s.lastHealthError
		s.healthMu.RUnlock()
		s.log.Debug("Health check: using cached result",
			"cached_error", err != nil,
			"cache_age", time.Since(s.lastHealthCheck))
		return err
	}
	s.healthMu.RUnlock()

	// Need to perform actual health check
	s.healthMu.Lock()
	defer s.healthMu.Unlock()

	// Double-check in case another goroutine updated while waiting for write lock
	if time.Since(s.lastHealthCheck) < cacheDuration {
		s.log.Debug("Health check: using cached result after lock",
			"cached_error", s.lastHealthError != nil,
			"cache_age", time.Since(s.lastHealthCheck))
		return s.lastHealthError
	}

	// Perform actual health check
	s.log.Debug("Health check: performing database check")
	err := s.queryEngine.HealthCheck(ctx)
	s.lastHealthCheck = time.Now()
	s.lastHealthError = err

	return err
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

	// Search products by brand and name tool (simplified version)
	searchSimplifiedTool := mcp.NewTool("search_products_by_brand_and_name_simplified",
		mcp.WithDescription("Search for branded products by their brand and product name returning simplified nutrients. This tool can only be used if brand and product name are both provided and non-empty."),
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
		mcp.WithOutputSchema[SearchProductsSimplifiedResponse](),
		mcp.WithIdempotentHintAnnotation(true),
	)

	s.mcpServer.AddTool(searchSimplifiedTool, s.handleSearchProductsSimplified)
}

func (s *Server) handleSearchProducts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.log.Debug("handleSearchProducts: Starting tool call",
		"arguments", request.GetArguments())

	// Extract arguments
	name, err := request.RequireString("name")
	if err != nil {
		s.log.Warn("handleSearchProducts: Missing 'name' parameter", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'name': %v", err)), nil
	}

	brand, err := request.RequireString("brand") // Required parameter
	if err != nil {
		s.log.Warn("handleSearchProducts: Missing 'brand' parameter", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'brand': %v", err)), nil
	}

	// Validate minimum lengths
	if len(name) < 1 {
		s.log.Warn("handleSearchProducts: Invalid 'name' parameter", "length", len(name))
		return mcp.NewToolResultError("Parameter 'name' must be at least 1 character long"), nil
	}
	if len(brand) < 1 {
		s.log.Warn("handleSearchProducts: Invalid 'brand' parameter", "length", len(brand))
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

	// Prepare structured response
	response := SearchProductsResponse{
		Found:    len(products) > 0,
		Count:    len(products),
		Products: products,
	}

	// Create fallback text for backwards compatibility
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		s.log.Error("handleSearchProducts: Failed to marshal response", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
	}

	s.log.Debug("handleSearchProducts: Returning structured result",
		"found", response.Found,
		"count", response.Count,
		"response_size", len(responseJSON))

	// Return both structured content and text fallback for maximum compatibility
	return mcp.NewToolResultStructured(response, string(responseJSON)), nil
}

func (s *Server) handleSearchProductsSimplified(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.log.Debug("handleSearchProductsSimplified: Starting tool call",
		"arguments", request.GetArguments())

	// Extract arguments
	name, err := request.RequireString("name")
	if err != nil {
		s.log.Warn("handleSearchProductsSimplified: Missing 'name' parameter", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'name': %v", err)), nil
	}

	brand, err := request.RequireString("brand") // Required parameter
	if err != nil {
		s.log.Warn("handleSearchProductsSimplified: Missing 'brand' parameter", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'brand': %v", err)), nil
	}

	// Validate minimum lengths
	if len(name) < 1 {
		s.log.Warn("handleSearchProductsSimplified: Invalid 'name' parameter", "length", len(name))
		return mcp.NewToolResultError("Parameter 'name' must be at least 1 character long"), nil
	}
	if len(brand) < 1 {
		s.log.Warn("handleSearchProductsSimplified: Invalid 'brand' parameter", "length", len(brand))
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

	s.log.Debug("MCP SearchProductsByBrandAndNameSimplified called",
		"name", name,
		"brand", brand,
		"limit", limit)

	// Execute search using existing query engine
	products, err := s.queryEngine.SearchProductsByBrandAndName(ctx, name, brand, limit)
	if err != nil {
		s.log.Error("Product search failed", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	// Convert to simplified products
	simplifiedProducts := make([]types.SimplifiedProduct, 0, len(products))
	for _, product := range products {
		simplifiedProducts = append(simplifiedProducts, product.ToSimplified())
	}

	// Prepare structured response
	response := SearchProductsSimplifiedResponse{
		Found:    len(simplifiedProducts) > 0,
		Count:    len(simplifiedProducts),
		Products: simplifiedProducts,
	}

	// Create fallback text for backwards compatibility
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		s.log.Error("handleSearchProductsSimplified: Failed to marshal response", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
	}

	s.log.Debug("handleSearchProductsSimplified: Returning structured result",
		"found", response.Found,
		"count", response.Count,
		"response_size", len(responseJSON))

	// Return both structured content and text fallback for maximum compatibility
	return mcp.NewToolResultStructured(response, string(responseJSON)), nil
}

func (s *Server) handleSearchByBarcode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.log.Debug("handleSearchByBarcode: Starting tool call",
		"arguments", request.GetArguments())

	// Extract arguments
	barcode, err := request.RequireString("barcode")
	if err != nil {
		s.log.Warn("handleSearchByBarcode: Missing 'barcode' parameter", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'barcode': %v", err)), nil
	}

	s.log.Debug("MCP SearchByBarcode called", "barcode", barcode)

	// Execute search
	product, err := s.queryEngine.SearchByBarcode(ctx, barcode)
	if err != nil {
		s.log.Error("Barcode search failed", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Barcode search failed: %v", err)), nil
	}

	// Prepare structured response
	response := SearchBarcodeResponse{
		Found:   product != nil,
		Product: product,
	}

	// Create fallback text for backwards compatibility
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		s.log.Error("handleSearchByBarcode: Failed to marshal response", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
	}

	s.log.Debug("handleSearchByBarcode: Returning structured result",
		"found", response.Found,
		"has_product", response.Product != nil,
		"response_size", len(responseJSON))

	// Return both structured content and text fallback for maximum compatibility
	return mcp.NewToolResultStructured(response, string(responseJSON)), nil
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

		// Use cached health check to prevent DOS attacks
		ctx := r.Context()
		if err := s.checkHealthWithCache(ctx); err != nil {
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

	// MCP endpoint with authentication and enhanced error logging
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// Add recovery middleware for better error handling
		defer func() {
			if recovery := recover(); recovery != nil {
				s.log.Error("MCP endpoint panic recovered",
					"panic", recovery,
					"method", r.Method,
					"url", r.URL.String(),
					"remote_addr", r.RemoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			}
		}()

		s.log.Debug("MCP request received",
			"method", r.Method,
			"url", r.URL.String(),
			"content_type", r.Header.Get("Content-Type"),
			"content_length", r.ContentLength,
			"remote_addr", r.RemoteAddr)

		// Check authentication for all non-health endpoints
		if !s.auth.IsAuthorized(r) {
			s.auth.SetUnauthorizedHeaders(w)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			s.log.Warn("Unauthorized MCP request", "remote_addr", r.RemoteAddr, "user_agent", r.UserAgent())
			return
		}

		// Create a custom ResponseWriter to capture response details
		recorder := &responseRecorder{ResponseWriter: w}

		// Forward to the streamable HTTP server
		streamableServer.ServeHTTP(recorder, r)

		s.log.Debug("MCP response sent",
			"status_code", recorder.statusCode,
			"response_size", recorder.bytesWritten,
			"content_type", recorder.Header().Get("Content-Type"))
	})

	s.log.Info("Starting MCP server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

// ServeStdio serves the MCP server over stdio (no auth required for local use)
func (s *Server) ServeStdio() error {
	s.log.Info("Starting MCP server in stdio mode")
	return server.ServeStdio(s.mcpServer)
}

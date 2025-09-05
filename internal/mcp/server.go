package mcp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
)

// Server represents the MCP server for OpenFoodFacts
type Server struct {
	mcpServer   *mcp.Server
	tools       *Tools
	config      *config.Config
	log         *slog.Logger
	queryEngine query.QueryEngine
}

// NewServer creates a new MCP server instance
func NewServer(cfg *config.Config, queryEngine query.QueryEngine, logger *slog.Logger) *Server {
	// Create tools instance
	tools := NewTools(queryEngine, logger)

	// Create MCP server implementation
	implementation := &mcp.Implementation{
		Name:    "openfoodfacts-mcp-server",
		Version: "1.0.0",
	}

	// Create MCP server
	mcpServer := mcp.NewServer(implementation, &mcp.ServerOptions{})

	// Register tools
	registerTools(mcpServer, tools)

	return &Server{
		mcpServer:   mcpServer,
		tools:       tools,
		config:      cfg,
		log:         logger,
		queryEngine: queryEngine,
	}
}

// registerTools registers all MCP tools with the server
func registerTools(server *mcp.Server, tools *Tools) {
	// Register product search tool
	searchTool := &mcp.Tool{
		Name:        "search_products_by_brand_and_name",
		Description: "Search for products by name and optional brand filter",
	}
	mcp.AddTool(server, searchTool, tools.SearchProductsByBrandAndName)

	// Register barcode search tool
	barcodeTool := &mcp.Tool{
		Name:        "search_by_barcode",
		Description: "Search for a product by its barcode (UPC/EAN)",
	}
	mcp.AddTool(server, barcodeTool, tools.SearchByBarcode)
}

// GetMCPServer returns the underlying MCP server for stdio mode
func (s *Server) GetMCPServer() *mcp.Server {
	return s.mcpServer
}

// CreateHandler creates an HTTP handler for the MCP server with API key authentication
func (s *Server) CreateHandler() http.Handler {
	// Create MCP handler
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, &mcp.StreamableHTTPOptions{})

	// Simple API key authentication middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.log.Warn("Missing Authorization header")
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract Bearer token
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			s.log.Warn("Invalid Authorization header format")
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Bearer token required", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)
		if token == "" {
			s.log.Warn("Empty Bearer token")
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Bearer token cannot be empty", http.StatusUnauthorized)
			return
		}

		// Validate API key
		if token != s.config.AuthToken {
			s.log.Warn("Invalid API key provided")
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// API key is valid, proceed with request
		s.log.Debug("API key authentication successful")
		mcpHandler.ServeHTTP(w, r)
	})
}

// CreateHealthHandler creates a health check handler
func (s *Server) CreateHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Test database connection
		ctx := r.Context()
		err := s.queryEngine.TestConnection(ctx)

		if err != nil {
			s.log.Error("Health check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"status":"unhealthy","ready":false,"error":"%s"}`, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy","ready":true}`)
	}
}

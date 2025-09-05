package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
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

	// Register nutrition analysis tool
	nutritionTool := &mcp.Tool{
		Name:        "get_nutrition_analysis",
		Description: "Get nutrition analysis and health insights for a product",
	}
	mcp.AddTool(server, nutritionTool, tools.GetNutritionAnalysis)
}

// CreateHandler creates an HTTP handler for the MCP server with authentication
func (s *Server) CreateHandler() http.Handler {
	// Create token verifier for Bearer token authentication
	verifier := func(ctx context.Context, tokenString string) (*auth.TokenInfo, error) {
		// Simple token verification - in production you'd validate JWT or API keys
		if tokenString == s.config.AuthToken {
			return &auth.TokenInfo{
				Scopes: []string{"read", "write"},
				Extra: map[string]any{
					"user_id": "authenticated-user",
				},
			}, nil
		}
		return nil, auth.ErrInvalidToken
	}

	// Create authentication middleware
	authMiddleware := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		Scopes: []string{"read"}, // Require at least read scope
	})

	// Create MCP handler
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, &mcp.StreamableHTTPOptions{})

	// Apply authentication middleware
	return authMiddleware(mcpHandler)
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

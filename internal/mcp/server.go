package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/auth"
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
	auth        *auth.BearerTokenAuth
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
		auth:        auth.NewBearerTokenAuth(cfg.AuthToken),
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
	// Simple API key authentication middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.Info("MCP request received",
			"method", r.Method,
			"path", r.URL.Path,
			"user_agent", r.Header.Get("User-Agent"),
			"auth_header", r.Header.Get("Authorization"),
			"content_type", r.Header.Get("Content-Type"))

		// Add CORS headers for browser-based clients like OpenAI
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Only allow POST requests for MCP
		if r.Method != "POST" {
			s.log.Warn("Invalid method", "method", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.auth.IsAuthorized(r) {
			s.log.Warn("Authentication failed",
				"auth_header", r.Header.Get("Authorization"),
				"expected_token_prefix", "Bearer")
			s.auth.SetUnauthorizedHeaders(w)
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		// API key is valid, proceed with request
		s.log.Debug("API key authentication successful")
		s.handleJSONRPC(w, r)
	})
}

// handleJSONRPC handles JSON-RPC requests directly
func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.log.Error("Failed to read request body", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	s.log.Debug("Received JSON-RPC request", "body", string(body))

	// Parse JSON-RPC request
	var rpcReq map[string]interface{}
	if err := json.Unmarshal(body, &rpcReq); err != nil {
		s.log.Error("Failed to parse JSON-RPC request", "error", err, "body", string(body))
		s.writeJSONRPCError(w, nil, -32700, "Parse error", nil)
		return
	}

	// Extract request ID for response
	var reqID interface{}
	if id, ok := rpcReq["id"]; ok {
		reqID = id
	}

	// Get the method
	method, ok := rpcReq["method"].(string)
	if !ok {
		s.log.Error("Missing or invalid method", "request", rpcReq)
		s.writeJSONRPCError(w, reqID, -32600, "Invalid Request", nil)
		return
	}

	s.log.Info("Handling JSON-RPC request", "method", method, "id", reqID)

	// Handle different MCP methods
	switch method {
	case "initialize":
		s.handleInitialize(w, reqID, rpcReq)
	case "tools/list":
		s.handleToolsList(w, reqID, rpcReq)
	case "tools/call":
		s.handleToolsCall(w, reqID, rpcReq)
	default:
		s.log.Warn("Unknown method", "method", method)
		s.writeJSONRPCError(w, reqID, -32601, "Method not found", nil)
	}
}

// writeJSONRPCError writes a JSON-RPC error response
func (s *Server) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	if data != nil {
		response["error"].(map[string]interface{})["data"] = data
	}

	json.NewEncoder(w).Encode(response)
}

// writeJSONRPCSuccess writes a JSON-RPC success response
func (s *Server) writeJSONRPCSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	json.NewEncoder(w).Encode(response)
}

// handleInitialize handles the initialize method
func (s *Server) handleInitialize(w http.ResponseWriter, id interface{}, req map[string]interface{}) {
	result := map[string]interface{}{
		"protocolVersion": "1.0.0",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "openfoodfacts-mcp-server",
			"version": "1.0.0",
		},
	}
	s.writeJSONRPCSuccess(w, id, result)
}

// handleToolsList handles the tools/list method
func (s *Server) handleToolsList(w http.ResponseWriter, id interface{}, req map[string]interface{}) {
	tools := []map[string]interface{}{
		{
			"name":        "search_products_by_brand_and_name",
			"description": "Search for products by name and optional brand filter",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Product name to search for",
					},
					"brand": map[string]interface{}{
						"type":        "string",
						"description": "Brand name to filter by (optional)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results (default: 10)",
						"default":     10,
					},
				},
				"required": []string{"name"},
			},
		},
		{
			"name":        "search_by_barcode",
			"description": "Search for a product by its barcode (UPC/EAN)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"barcode": map[string]interface{}{
						"type":        "string",
						"description": "The barcode (UPC/EAN) to search for",
					},
				},
				"required": []string{"barcode"},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}
	s.writeJSONRPCSuccess(w, id, result)
}

// handleToolsCall handles the tools/call method
func (s *Server) handleToolsCall(w http.ResponseWriter, id interface{}, req map[string]interface{}) {
	params, ok := req["params"].(map[string]interface{})
	if !ok {
		s.writeJSONRPCError(w, id, -32602, "Invalid params", nil)
		return
	}

	toolName, ok := params["name"].(string)
	if !ok {
		s.writeJSONRPCError(w, id, -32602, "Missing tool name", nil)
		return
	}

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		s.writeJSONRPCError(w, id, -32602, "Missing arguments", nil)
		return
	}

	s.log.Debug("Calling tool", "name", toolName, "arguments", arguments)

	// Call the appropriate tool
	var result *mcp.CallToolResult
	var err error

	switch toolName {
	case "search_products_by_brand_and_name":
		// Convert arguments to proper struct
		var args SearchProductsByBrandAndNameArgs
		argBytes, _ := json.Marshal(arguments)
		if err := json.Unmarshal(argBytes, &args); err != nil {
			s.writeJSONRPCError(w, id, -32602, "Invalid arguments format", err.Error())
			return
		}
		result, _, err = s.tools.SearchProductsByBrandAndName(context.Background(), nil, args)

	case "search_by_barcode":
		// Convert arguments to proper struct
		var args SearchByBarcodeArgs
		argBytes, _ := json.Marshal(arguments)
		if err := json.Unmarshal(argBytes, &args); err != nil {
			s.writeJSONRPCError(w, id, -32602, "Invalid arguments format", err.Error())
			return
		}
		result, _, err = s.tools.SearchByBarcode(context.Background(), nil, args)

	default:
		s.writeJSONRPCError(w, id, -32601, "Tool not found", nil)
		return
	}

	if err != nil {
		s.log.Error("Tool call failed", "tool", toolName, "error", err)
		s.writeJSONRPCError(w, id, -32603, "Tool execution error", err.Error())
		return
	}

	s.writeJSONRPCSuccess(w, id, result)
}

// CreateHealthHandler creates a health check handler
func (s *Server) CreateHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

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

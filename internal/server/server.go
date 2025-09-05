package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/dataset"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
)

// QueryRequest represents the JSON body for search requests
type QueryRequest struct {
	Name    string `json:"name"`
	Brand   string `json:"brand"`
	Barcode string `json:"barcode"`
	Limit   int    `json:"limit"`
}

// QueryResponse represents the JSON response for search requests
type QueryResponse struct {
	Found    bool            `json:"found"`
	Products []query.Product `json:"products"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status string `json:"status"`
	Ready  bool   `json:"ready"`
}

// Server represents the MCP server
type Server struct {
	config      *config.Config
	dataManager *dataset.Manager
	queryEngine query.QueryEngine
	log         *slog.Logger
	ready       bool
}

// New creates a new server instance
func New(cfg *config.Config, logger *slog.Logger) *Server {
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		logger,
	)

	return &Server{
		config:      cfg,
		dataManager: dataManager,
		log:         logger,
		ready:       false,
	}
}

// Start starts the HTTP server and background processes
func (s *Server) Start(ctx context.Context) error {
	s.log.Info("Starting OpenFoodFacts MCP Server", "port", s.config.Port)

	// Initialize dataset and query engine
	if err := s.initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Start refresh loop if configured
	if s.config.RefreshIntervalHours > 0 {
		s.startRefreshLoop(ctx)
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/query", s.handleQuery)

	// Create server
	server := &http.Server{
		Addr:    ":" + s.config.Port,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		s.log.Info("HTTP server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.log.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	s.log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		s.log.Error("Server shutdown error", "error", err)
	}

	if s.queryEngine != nil {
		s.queryEngine.Close()
	}

	s.log.Info("Server stopped")
	return nil
}

// initialize sets up the dataset and query engine
func (s *Server) initialize(ctx context.Context) error {
	start := time.Now()
	s.log.Info("Initializing server...")

	// Log development mode warning
	if s.config.IsDevelopment() {
		s.log.Warn("🚧 DEVELOPMENT MODE ENABLED 🚧",
			"environment", s.config.Environment,
			"note", "Detailed error messages will be returned to clients")
	}

	// Ensure dataset is available
	if err := s.dataManager.EnsureDataset(ctx); err != nil {
		return fmt.Errorf("failed to ensure dataset: %w", err)
	}

	// Initialize query engine
	engine, err := query.NewQueryEngine(s.config.ParquetPath, s.log)
	if err != nil {
		return fmt.Errorf("failed to create query engine: %w", err)
	}
	s.queryEngine = engine

	// Test connection
	if err := s.queryEngine.TestConnection(ctx); err != nil {
		return fmt.Errorf("failed to test connection: %w", err)
	}

	s.ready = true
	s.log.Info("Server initialized successfully", "duration", time.Since(start))
	return nil
}

// startRefreshLoop starts the background refresh process
func (s *Server) startRefreshLoop(ctx context.Context) {
	interval := s.config.RefreshInterval()
	s.log.Info("Starting refresh loop", "interval", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.log.Info("Refresh loop stopping due to context cancellation")
				return
			case <-ticker.C:
				s.log.Info("Refresh tick: checking dataset")
				if err := s.dataManager.EnsureDataset(ctx); err != nil {
					s.log.Error("Refresh failed", "error", err)
				} else {
					s.log.Info("Refresh completed successfully")
				}
			}
		}
	}()
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status: "ok",
		Ready:  s.ready,
	}

	w.Header().Set("Content-Type", "application/json")
	if !s.ready {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

// handleQuery handles product search requests
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check method
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authorization
	if !s.isAuthorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if server is ready
	if !s.ready {
		http.Error(w, "server not ready", http.StatusServiceUnavailable)
		return
	}

	// Parse request
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.log.Warn("Bad request", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Set default limit
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	start := time.Now()
	s.log.Debug("Incoming query", "name", req.Name, "brand", req.Brand, "barcode", req.Barcode, "limit", req.Limit)

	var products []query.Product
	var err error

	// Handle barcode search (exact match)
	if req.Barcode != "" {
		product, err := s.queryEngine.SearchByBarcode(ctx, req.Barcode)
		if err != nil {
			s.log.Error("Barcode search failed", "error", err, "barcode", req.Barcode)
			s.sendErrorResponse(w, err, "internal error", http.StatusInternalServerError)
			return
		}
		if product != nil {
			products = []query.Product{*product}
		}
	} else {
		// Handle name/brand search
		products, err = s.queryEngine.SearchProductsByBrandAndName(ctx, req.Name, req.Brand, req.Limit)
		if err != nil {
			s.log.Error("Product search failed", "error", err, "name", req.Name, "brand", req.Brand)
			s.sendErrorResponse(w, err, "internal error", http.StatusInternalServerError)
			return
		}
	}

	duration := time.Since(start)
	s.log.Info("Query completed", "found", len(products), "duration", duration)

	response := QueryResponse{
		Found:    len(products) > 0,
		Products: products,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// isAuthorized checks if the request is properly authorized
func (s *Server) isAuthorized(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" || token == authHeader {
		return false
	}

	return token == s.config.AuthToken
}

// sendErrorResponse sends an error response, with detailed error in development mode
func (s *Server) sendErrorResponse(w http.ResponseWriter, err error, message string, statusCode int) {
	if s.config.IsDevelopment() {
		// In development mode, return detailed error
		http.Error(w, fmt.Sprintf("%s: %v", message, err), statusCode)
	} else {
		// In production mode, return generic message
		http.Error(w, message, statusCode)
	}
}

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/dataset"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
)

// MCPServer represents the MCP-enabled server
type MCPServer struct {
	config      *config.Config
	dataManager *dataset.Manager
	queryEngine query.QueryEngine
	mcpServer   *mcp.Server
	log         *slog.Logger
	ready       bool
}

// NewMCPServer creates a new MCP-enabled server instance
func NewMCPServer(cfg *config.Config, logger *slog.Logger) *MCPServer {
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		logger,
	)

	return &MCPServer{
		config:      cfg,
		dataManager: dataManager,
		log:         logger,
		ready:       false,
	}
}

// Start starts the MCP server and background processes
func (s *MCPServer) Start(ctx context.Context) error {
	s.log.Info("🚀 Initializing OpenFoodFacts MCP Server (HTTP Mode)",
		"mode", "http",
		"port", s.config.Port,
		"auth_required", "yes (Bearer token)",
		"health_endpoint", "/health (no auth required)",
		"mcp_endpoint", "/mcp (auth required)")

	// Initialize dataset and query engine
	if err := s.initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Start refresh loop if configured
	if s.config.RefreshIntervalHours > 0 {
		s.startRefreshLoop(ctx)
	}

	// Create MCP server
	s.mcpServer = mcp.NewServer(s.config, s.queryEngine, s.log)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Health check endpoint (no auth required)
	mux.HandleFunc("/health", s.mcpServer.CreateHealthHandler())

	// MCP endpoint with authentication
	mux.Handle("/mcp", s.mcpServer.CreateHandler())

	// Create server with timeouts and keep-alive settings
	server := &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		s.log.Info("🌐 MCP HTTP server ready for remote connections",
			"addr", server.Addr,
			"mode", "http",
			"endpoints", map[string]string{
				"/health": "health check (no auth)",
				"/mcp":    "MCP JSON-RPC 2.0 (auth required)",
			})
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.log.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	s.log.Info("Shutting down MCP server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		s.log.Error("Server shutdown error", "error", err)
	}

	if s.queryEngine != nil {
		s.queryEngine.Close()
	}

	s.log.Info("MCP server stopped")
	return nil
}

// initialize sets up the dataset and query engine
func (s *MCPServer) initialize(ctx context.Context) error {
	start := time.Now()
	s.log.Info("Initializing MCP server...")

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

	// Mark as ready
	s.ready = true

	s.log.Info("MCP server initialized successfully", "duration", time.Since(start))
	return nil
}

// startRefreshLoop starts a background goroutine to refresh the dataset
func (s *MCPServer) startRefreshLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Duration(s.config.RefreshIntervalHours) * time.Hour)
		defer ticker.Stop()

		s.log.Info("Started dataset refresh loop", "interval_hours", s.config.RefreshIntervalHours)

		for {
			select {
			case <-ctx.Done():
				s.log.Info("Stopping dataset refresh loop")
				return
			case <-ticker.C:
				s.log.Info("Refreshing dataset...")
				if err := s.dataManager.EnsureDataset(ctx); err != nil {
					s.log.Error("Failed to refresh dataset", "error", err)
				} else {
					s.log.Info("Dataset refresh completed")
				}
			}
		}
	}()
}

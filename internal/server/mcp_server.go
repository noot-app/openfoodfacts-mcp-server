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
	"github.com/noot-app/openfoodfacts-mcp-server/internal/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
)

// MCPServer represents the MCP-enabled server
type MCPServer struct {
	config      *config.Config
	queryEngine query.QueryEngine
	mcpServer   *mcp.Server
	log         *slog.Logger
	ready       bool
	initializer *ServerInitializer
}

// NewMCPServer creates a new MCP-enabled server instance
func NewMCPServer(cfg *config.Config, logger *slog.Logger) *MCPServer {
	initializer := NewServerInitializer(cfg, logger)

	return &MCPServer{
		config:      cfg,
		log:         logger,
		ready:       false,
		initializer: initializer,
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
	if s.config.RefreshIntervalSeconds > 0 {
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
		ReadTimeout:  HTTPReadTimeout,
		WriteTimeout: HTTPWriteTimeout,
		IdleTimeout:  HTTPIdleTimeout,
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), HTTPShutdownTimeout)
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
	// Use common initializer
	engine, err := s.initializer.Initialize(ctx)
	if err != nil {
		return err
	}
	s.queryEngine = engine

	// Mark as ready
	s.ready = true
	return nil
}

// startRefreshLoop starts a background goroutine to refresh the dataset
func (s *MCPServer) startRefreshLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Duration(s.config.RefreshIntervalSeconds) * time.Second)
		defer ticker.Stop()

		s.log.Info("Started dataset refresh loop", "interval_seconds", s.config.RefreshIntervalSeconds)

		for {
			select {
			case <-ctx.Done():
				s.log.Info("Stopping dataset refresh loop")
				return
			case <-ticker.C:
				s.log.Info("Refreshing dataset...")
				if err := s.initializer.RefreshDataset(ctx); err != nil {
					s.log.Error("Failed to refresh dataset", "error", err)
				} else {
					s.log.Info("Dataset refresh completed")
				}
			}
		}
	}()
}

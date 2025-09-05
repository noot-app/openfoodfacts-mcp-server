package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/dataset"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
)

// ServerInitializer handles common server initialization logic
type ServerInitializer struct {
	config      *config.Config
	log         *slog.Logger
	dataManager *dataset.Manager
}

// NewServerInitializer creates a new server initializer
func NewServerInitializer(cfg *config.Config, logger *slog.Logger) *ServerInitializer {
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		cfg,
		logger,
	)

	return &ServerInitializer{
		config:      cfg,
		log:         logger,
		dataManager: dataManager,
	}
}

// Initialize performs common server initialization steps
func (si *ServerInitializer) Initialize(ctx context.Context) (query.QueryEngine, error) {
	start := time.Now()
	si.log.Info("Initializing server...")

	// Log development mode warning
	if si.config.IsDevelopment() {
		si.log.Warn("🚧 DEVELOPMENT MODE ENABLED 🚧",
			"environment", si.config.Environment,
			"note", "Detailed error messages will be returned to clients")
	}

	// Ensure dataset is available
	if err := si.dataManager.EnsureDataset(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure dataset: %w", err)
	}

	// Initialize query engine
	engine, err := query.NewQueryEngine(si.config.ParquetPath, si.config, si.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create query engine: %w", err)
	}

	// Test connection
	if err := engine.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to test connection: %w", err)
	}

	si.log.Info("Server initialized successfully", "duration", time.Since(start))
	return engine, nil
}

// RefreshDataset refreshes the dataset
func (si *ServerInitializer) RefreshDataset(ctx context.Context) error {
	return si.dataManager.EnsureDataset(ctx)
}

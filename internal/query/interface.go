package query

import (
	"context"
	"log/slog"
	"os"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/types"
)

// QueryEngine defines the interface for querying the product database
type QueryEngine interface {
	SearchProductsByBrandAndName(ctx context.Context, name, brand string, limit int) ([]types.Product, error)
	SearchByBarcode(ctx context.Context, barcode string) (*types.Product, error)
	TestConnection(ctx context.Context) error
	Close() error
}

// NewQueryEngine creates a new query engine
// Uses mock engine if QUERY_ENGINE_MOCK environment variable is set
func NewQueryEngine(parquetPath string, cfg *config.Config, logger *slog.Logger) (QueryEngine, error) {
	if os.Getenv("QUERY_ENGINE_MOCK") == "true" {
		return NewMockEngine(logger), nil
	}
	return NewEngine(parquetPath, cfg, logger)
}

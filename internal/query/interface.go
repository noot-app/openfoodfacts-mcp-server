package query

import (
	"context"
	"log/slog"
	"os"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
)

// QueryEngine defines the interface for querying the product database
type QueryEngine interface {
	SearchProductsByBrandAndName(ctx context.Context, name, brand string, limit int) ([]Product, error)
	SearchByBarcode(ctx context.Context, barcode string) (*Product, error)
	TestConnection(ctx context.Context) error
	Close() error
}

// Product represents a product from the Open Food Facts dataset
type Product struct {
	Code        string                 `json:"code"`
	ProductName string                 `json:"product_name"`
	Brands      string                 `json:"brands"`
	Nutriments  map[string]interface{} `json:"nutriments"`
	Link        string                 `json:"link"`
	Ingredients interface{}            `json:"ingredients"`
}

// NewQueryEngine creates a new query engine
// Uses mock engine if QUERY_ENGINE_MOCK environment variable is set
func NewQueryEngine(parquetPath string, cfg *config.Config, logger *slog.Logger) (QueryEngine, error) {
	if os.Getenv("QUERY_ENGINE_MOCK") == "true" {
		return NewMockEngine(logger), nil
	}
	return NewEngine(parquetPath, cfg, logger)
}

package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/server"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "openfoodfacts-mcp-server",
	Short: "OpenFoodFacts MCP Server with DuckDB",
	Long: `OpenFoodFacts MCP Server provides access to the Open Food Facts dataset
via a remote MCP server using DuckDB for fast queries.

The server downloads and caches the Open Food Facts Parquet dataset
and provides MCP-compliant endpoints for product searches, nutrition analysis,
and barcode lookups with proper authentication and JSON-RPC 2.0 support.

Available MCP Tools:
- search_products_by_brand_and_name: Search products by name and brand
- search_by_barcode: Find product by barcode (UPC/EAN)

Authentication:
Bearer token authentication is required for all MCP endpoints.
Use the OPENFOODFACTS_MCP_TOKEN environment variable to set the token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Setup structured logging
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		// Load configuration
		cfg := config.Load()

		// Create and start MCP server
		srv := server.NewMCPServer(cfg, logger)
		return srv.Start(context.Background())
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// Run is the main entry point for the CLI application
func Run() error {
	return Execute()
}

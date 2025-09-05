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
and provides HTTP endpoints for product searches by name, brand, and barcode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Setup structured logging
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		// Load configuration
		cfg := config.Load()

		// Create and start server
		srv := server.New(cfg, logger)
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

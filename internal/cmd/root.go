package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/dataset"
	mcpserver "github.com/noot-app/openfoodfacts-mcp-server/internal/mcp"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
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
		// Check if we should run in stdio mode (for Claude Desktop)
		stdio, _ := cmd.Flags().GetBool("stdio")

		if stdio {
			return runStdioMode(cmd, args)
		} else {
			return runHTTPMode(cmd, args)
		}
	},
}

func init() {
	rootCmd.Flags().Bool("stdio", false, "Run in stdio mode for Claude Desktop integration")
}

// runStdioMode runs the MCP server in stdio mode for Claude Desktop
func runStdioMode(cmd *cobra.Command, args []string) error {
	// Use a logger that writes to stderr to avoid interfering with stdio MCP communication
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Only show warnings and errors in stdio mode
	}))

	// Load configuration
	cfg := config.Load()

	logger.Info("Starting OpenFoodFacts MCP Server in stdio mode")

	// Initialize dataset manager
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		logger,
	)

	// Ensure dataset is available
	ctx := context.Background()
	if err := dataManager.EnsureDataset(ctx); err != nil {
		logger.Error("Failed to ensure dataset", "error", err)
		return err
	}

	// Initialize query engine
	queryEngine, err := query.NewQueryEngine(cfg.ParquetPath, logger)
	if err != nil {
		logger.Error("Failed to create query engine", "error", err)
		return err
	}
	defer queryEngine.Close()

	// Test connection
	if err := queryEngine.TestConnection(ctx); err != nil {
		logger.Error("Failed to test connection", "error", err)
		return err
	}

	// Create MCP server
	mcpSrv := mcpserver.NewServer(cfg, queryEngine, logger)

	// Run the MCP server on stdio transport
	transport := &mcp.StdioTransport{}
	return mcpSrv.GetMCPServer().Run(context.Background(), transport)
}

// runHTTPMode runs the MCP server in HTTP mode for remote deployment
func runHTTPMode(cmd *cobra.Command, args []string) error {
	// Setup structured logging for HTTP mode
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	cfg := config.Load()

	// Create and start MCP server in HTTP mode
	srv := server.NewMCPServer(cfg, logger)
	return srv.Start(context.Background())
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

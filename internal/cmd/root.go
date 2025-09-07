package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/auth"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/dataset"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/mcpgo"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "openfoodfacts-mcp-server",
	Short: "OpenFoodFacts MCP Server with DuckDB",
	Long: `OpenFoodFacts MCP Server provides access to the Open Food Facts dataset
via a remote MCP server using DuckDB for fast queries.

The server operates in three modes:

1. STDIO Mode (--stdio): For local Claude Desktop integration
   - Uses stdio pipes for communication
   - No authentication required
   - Perfect for local development and Claude Desktop

2. HTTP Mode (default): For remote deployment over the internet
   - Exposes HTTP endpoints with JSON-RPC 2.0
   - Requires Bearer token authentication (except /health)
   - Ideal for shared/remote MCP server deployments

3. Fetch Database Mode (--fetch-db): Download dataset and exit
   - Downloads/updates the OpenFoodFacts Parquet dataset
   - Checks if local dataset is up-to-date with remote
   - Exits after download completion (does not start server)
   - Useful for pre-populating dataset cache

The server downloads and caches the Open Food Facts Parquet dataset
and provides MCP-compliant endpoints for product searches, nutrition analysis,
and barcode lookups.

Available MCP Tools:
- search_products_by_brand_and_name: Search products by name and brand
- search_by_barcode: Find product by barcode (UPC/EAN)

Authentication (HTTP Mode Only):
Bearer token authentication is required for all MCP endpoints except /health.
Use the OPENFOODFACTS_MCP_TOKEN environment variable to set the token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we should only fetch the database
		fetchDB, _ := cmd.Flags().GetBool("fetch-db")
		if fetchDB {
			return runFetchDBMode(cmd, args)
		}

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
	rootCmd.Flags().Bool("stdio", false, "Run in stdio mode for local Claude Desktop integration (default: HTTP mode for remote deployment)")
	rootCmd.Flags().Bool("fetch-db", false, "Fetch the database and exit (useful for downloading the dataset without starting the server)")
}

// runFetchDBMode fetches the database and exits
func runFetchDBMode(cmd *cobra.Command, args []string) error {
	// Setup logger for fetch mode
	logger := config.NewTextLogger(os.Stdout)

	// Load configuration
	cfg := config.Load()

	logger.Info("🗄️  Starting database fetch",
		"mode", "fetch-db",
		"description", "Download and cache the OpenFoodFacts dataset",
		"target_dir", filepath.Dir(cfg.ParquetPath))

	logger.Info("⚠️  Large dataset warning",
		"message", "The OpenFoodFacts dataset is approximately 4+ GB in size",
		"note", "Initial download may take several minutes depending on your internet connection")

	// Initialize dataset manager
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		cfg,
		logger,
	)

	// Ensure dataset is available (this will download if needed)
	ctx := context.Background()
	if err := dataManager.EnsureDataset(ctx); err != nil {
		logger.Error("Failed to fetch dataset", "error", err)
		return err
	}

	logger.Info("✅ Database fetch completed successfully",
		"parquet_path", cfg.ParquetPath,
		"metadata_path", cfg.MetadataPath)

	return nil
}

// runStdioMode runs the MCP server in stdio mode for Claude Desktop
func runStdioMode(cmd *cobra.Command, args []string) error {
	// Use a logger that writes to stderr to avoid interfering with stdio MCP communication
	logger := config.NewLogger(true) // true for stdio mode

	// Load configuration
	cfg := config.Load()

	logger.Info("🔌 Starting OpenFoodFacts MCP Server in STDIO mode",
		"mode", "stdio",
		"description", "Local MCP server for Claude Desktop integration",
		"auth", "not required for stdio mode",
		"transport", "stdio pipes")

	// Initialize dataset manager
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		cfg,
		logger,
	)

	// Ensure dataset is available
	ctx := context.Background()
	if err := dataManager.EnsureDataset(ctx); err != nil {
		logger.Error("Failed to ensure dataset", "error", err)
		return err
	}

	// Initialize query engine
	queryEngine, err := query.NewEngine(cfg.ParquetPath, cfg, logger)
	if err != nil {
		logger.Error("Failed to create query engine", "error", err)
		return err
	}

	// Test connection
	if err := queryEngine.TestConnection(ctx); err != nil {
		logger.Error("Failed to test connection", "error", err)
		return err
	}

	// Create auth (not needed for stdio but required by constructor)
	authenticator := auth.NewBearerTokenAuth(cfg.AuthToken)

	// Create MCP server
	mcpSrv := mcpgo.NewServer(queryEngine, authenticator, logger)

	// Run the MCP server on stdio transport (no auth needed for local use)
	return mcpSrv.ServeStdio()
}

// runHTTPMode runs the MCP server in HTTP mode for remote deployment
func runHTTPMode(cmd *cobra.Command, args []string) error {
	// Setup structured logging for HTTP mode
	logger := config.NewLogger(false) // false for HTTP mode

	// Load configuration
	cfg := config.Load()

	logger.Info("🌐 Starting OpenFoodFacts MCP Server in HTTP mode",
		"mode", "http",
		"description", "Remote MCP server with API key authentication",
		"auth", "Bearer token required (except /health endpoint)",
		"transport", "HTTP/JSON-RPC 2.0",
		"port", cfg.Port)

	// Initialize dataset manager
	dataManager := dataset.NewManager(
		cfg.ParquetURL,
		cfg.ParquetPath,
		cfg.MetadataPath,
		cfg.LockFile,
		cfg,
		logger,
	)

	// Ensure dataset is available
	ctx := context.Background()
	if err := dataManager.EnsureDataset(ctx); err != nil {
		logger.Error("Failed to ensure dataset", "error", err)
		return err
	}

	// Create query engine
	queryEngine, err := query.NewEngine(cfg.ParquetPath, cfg, logger)
	if err != nil {
		logger.Error("Failed to create query engine", "error", err)
		return err
	}

	// Test connection
	if err := queryEngine.TestConnection(ctx); err != nil {
		logger.Error("Failed to test connection", "error", err)
		return err
	}

	// Create auth
	authenticator := auth.NewBearerTokenAuth(cfg.AuthToken)

	// Create MCP server
	mcpSrv := mcpgo.NewServer(queryEngine, authenticator, logger)

	// Run the MCP server on HTTP transport with auth
	return mcpSrv.ServeHTTP(":" + cfg.Port)
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

# OpenFoodFacts MCP Server 🥘

A high-performance MCP (Model Context Protocol) server that provides access to the Open Food Facts dataset using DuckDB and parquet for fast queries. Designed for both local development and remote deployment with authentication.

## How It Works

This MCP server downloads and caches the Open Food Facts Parquet dataset locally, then uses DuckDB for fast product searches. It provides two main tools:

- **search_products_by_brand_and_name**: Search products by name and brand
- **search_by_barcode**: Find product by barcode (UPC/EAN)

The server automatically manages dataset updates, uses file locking for concurrent safety, and provides structured JSON logging.

## Local Setup for Claude Desktop

### 1. Build the Server

```bash
script/build --single-target
```

### 2. Configure Claude Desktop

Add this to your Claude Desktop MCP settings (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "openfoodfacts": {
      "command": "/path/to/openfoodfacts-mcp-server",
      "args": ["--stdio"],
      "env": {
        "OPENFOODFACTS_MCP_TOKEN": "your-secret-token",
        "DATA_DIR": "/full/path/to/openfoodfacts-mcp-server/data",
        "ENV": "development"
      }
    }
  }
}
```

### 3. Start Using

Restart Claude Desktop. The server will automatically download the dataset on first run and be ready for food product queries.

## Remote Deployment Configuration

### Environment Variables

For production deployment (HTTP mode), configure these environment variables:

```bash
# Required: Authentication
OPENFOODFACTS_MCP_TOKEN=your-production-secret-token

# Optional: Data management
DATA_DIR=./data
PARQUET_URL=https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet
REFRESH_INTERVAL_HOURS=24

# Optional: Server configuration  
PORT=8080
ENV=production
```

### Running in HTTP Mode

For remote deployment, run without the `--stdio` flag:

```bash
./openfoodfacts-mcp-server
```

This will start an HTTP server on the configured port (default 8080).

### Claude Desktop Remote Setup

For remote MCP server, update your Claude Desktop config:

```json
{
  "mcpServers": {
    "openfoodfacts": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-fetch"],
      "env": {
        "FETCH_HOST": "https://your-server.com",
        "FETCH_API_KEY": "your-production-secret-token"
      }
    }
  }
}
```

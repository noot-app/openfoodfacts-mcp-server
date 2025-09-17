# OpenFoodFacts MCP Server 🔌

[![lint](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/lint.yml/badge.svg)](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/lint.yml)
[![test](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/test.yml/badge.svg)](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/test.yml)
[![build](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/build.yml/badge.svg)](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/build.yml)
[![docker](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/docker.yml/badge.svg)](https://github.com/noot-app/openfoodfacts-mcp-server/actions/workflows/docker.yml)

A MCP (Model Context Protocol) server that provides access to the Open Food Facts dataset using DuckDB and parquet for fast queries. Supports both local Claude Desktop integration and remote deployment with authentication.

![logo](./docs/assets/logo.png)

## Usage 💻

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/openfoodfacts-mcp-server?referralCode=0D8Grd&utm_medium=integration&utm_source=template&utm_campaign=generic)

This MCP server can operate in two distinct modes:

### 1. **STDIO Mode** (Local Claude Desktop Integration)

- **Use case**: Local development and Claude Desktop integration
- **Command**: `./openfoodfacts-mcp-server --stdio`
- **Transport**: stdio pipes
- **Authentication**: None required
- **Perfect for**: Claude Desktop, local development, testing

### 2. **HTTP Mode** (Remote Deployment)

- **Use case**: Remote MCP server accessible over the internet
- **Command**: `./openfoodfacts-mcp-server` (default mode)
- **Transport**: HTTP with JSON-RPC 2.0
- **Authentication**: Bearer token required (except `/health` endpoint)
- **Perfect for**: Shared deployments, cloud hosting, team access, mcp as a service

## Demo 📹

<https://github.com/user-attachments/assets/e742c4d3-a36d-46af-97b6-dcd41180b5aa>

## How It Works 💡

This MCP server downloads and caches the Open Food Facts Parquet dataset locally, then uses DuckDB for fast product searches. It provides two main tools:

- **search_products_by_brand_and_name**: Search products by name and brand
- **search_by_barcode**: Find product by barcode (UPC/EAN)

The server automatically manages dataset updates, uses file locking for concurrent safety, and provides structured JSON logging.

## Local Setup for Claude Desktop (STDIO Mode)

This setup uses **STDIO mode** for local Claude Desktop integration.

### 1. Build the Binary

```bash
script/build --simple
```

### 2. Fetch the Database

```bash
openfoodfacts-mcp-server --fetch-db
```

### 3. Configure Claude Desktop

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

### 3. Try it Out

Restart Claude Desktop. The mcp server will automatically start and be ready for food product queries.

## Remote Deployment (HTTP Mode)

This setup uses **HTTP mode** for remote deployment with authentication.

### Environment Variables

For production deployment (HTTP mode), configure these environment variables:

```bash
# Required: Authentication
OPENFOODFACTS_MCP_TOKEN=your-production-secret-token

# Optional: Data management
DATA_DIR=./data
PARQUET_URL=https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet
REFRESH_INTERVAL_SECONDS=86400

# Optional: Server configuration  
PORT=8080
ENV=production

# Optional: DuckDB Performance Tuning
DUCKDB_MEMORY_LIMIT=4GB                    # Memory limit (e.g. 2GB, 4GB, 8GB, 16GB)
DUCKDB_THREADS=4                           # Number of threads (1-16 typically)
DUCKDB_CHECKPOINT_THRESHOLD=1GB            # When to write to disk (e.g. 512MB, 1GB, 2GB)
DUCKDB_PRESERVE_INSERTION_ORDER=true       # Set to false for large datasets to reduce memory
```

### Running in HTTP Mode

For remote deployment, run **without** the `--stdio` flag (HTTP mode is the default):

```bash
./openfoodfacts-mcp-server
```

This will start an HTTP server on the configured port (default 8080) with:

- `/health` endpoint (no authentication required)
- `/mcp` endpoint (Bearer token authentication required)

## Quick Reference

### Command Options

| Mode | Command | Use Case | Authentication | Transport |
|------|---------|----------|----------------|-----------|
| **STDIO** | `./openfoodfacts-mcp-server --stdio` | Claude Desktop, local development | None | stdio pipes |
| **HTTP** | `./openfoodfacts-mcp-server` | Remote deployment, shared access | Bearer token | HTTP/JSON-RPC |
| **Fetch DB** | `./openfoodfacts-mcp-server --fetch-db` | Download/update dataset locally | None | N/A |

### Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENFOODFACTS_MCP_TOKEN` | Yes (HTTP mode) | - | Bearer token for authentication |
| `DATA_DIR` | No | `./data` | Directory for dataset storage |
| `PORT` | No | `8080` | HTTP server port (HTTP mode only) |
| `ENV` | No | `production` | Environment (development/production) |
| `DUCKDB_MEMORY_LIMIT` | No | `4GB` | DuckDB memory limit (2GB, 4GB, 8GB, etc.) |
| `DUCKDB_THREADS` | No | `4` | Number of DuckDB threads (1-16) |
| `DUCKDB_CHECKPOINT_THRESHOLD` | No | `1GB` | Checkpoint threshold (512MB, 1GB, 2GB) |
| `DUCKDB_PRESERVE_INSERTION_ORDER` | No | `true` | Preserve insertion order (false for better performance) |

### HTTP Endpoints (HTTP Mode Only)

| Endpoint | Authentication | Description |
|----------|----------------|-------------|
| `/health` | None | Health check endpoint |
| `/mcp` | Bearer token | MCP JSON-RPC 2.0 endpoint |

### STDIO Mode (Local Development)

A cool tip for developing locally, you can actually do this and it will return a result from the MCP server:

```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "search_products_by_brand_and_name_simplified", "arguments": {"name": "cream soda", "brand": "olipop", "limit": 1}}}' | go run cmd/openfoodfacts-mcp-server/main.go --stdio
```

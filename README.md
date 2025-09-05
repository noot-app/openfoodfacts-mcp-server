# OpenFoodFacts MCP Server 🥘

A high-performance MCP (Model Context Protocol) server that provides access to the Open Food Facts dataset using DuckDB and parquet for fast queries. Supports both local Claude Desktop integration and remote deployment with authentication.

## Two Ways to Run

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
- **Perfect for**: Shared deployments, cloud hosting, team access

## How It Works

This MCP server downloads and caches the Open Food Facts Parquet dataset locally, then uses DuckDB for fast product searches. It provides two main tools:

- **search_products_by_brand_and_name**: Search products by name and brand
- **search_by_barcode**: Find product by barcode (UPC/EAN)

The server automatically manages dataset updates, uses file locking for concurrent safety, and provides structured JSON logging.

## Local Setup for Claude Desktop (STDIO Mode)

This setup uses **STDIO mode** for local Claude Desktop integration.

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

## Quick Reference

### Command Options

| Mode | Command | Use Case | Authentication | Transport |
|------|---------|----------|----------------|-----------|
| **STDIO** | `./openfoodfacts-mcp-server --stdio` | Claude Desktop, local development | None | stdio pipes |
| **HTTP** | `./openfoodfacts-mcp-server` | Remote deployment, shared access | Bearer token | HTTP/JSON-RPC |

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

## DuckDB Performance Tuning

This server includes comprehensive DuckDB performance optimizations based on the [official DuckDB performance guide](https://duckdb.org/docs/stable/guides/performance/overview). You can tune performance using environment variables:

### Memory Configuration

- **`DUCKDB_MEMORY_LIMIT`**: Set based on available system memory
  - Small systems: `2GB` or `4GB`
  - Medium systems: `8GB` or `16GB`
  - Large systems: `32GB` or higher
  - Rule of thumb: 1-4GB per thread for aggregation workloads

### Threading Configuration

- **`DUCKDB_THREADS`**: Set based on CPU cores
  - Small systems: `2-4` threads
  - Medium systems: `4-8` threads  
  - Large systems: `8-16` threads
  - Avoid setting higher than your CPU core count

### Performance vs Memory Trade-offs

- **`DUCKDB_PRESERVE_INSERTION_ORDER=false`**: Allows DuckDB to reorder results for better memory efficiency
- **`DUCKDB_CHECKPOINT_THRESHOLD`**: Higher values = more memory usage, but potentially better performance

### Production Recommendations

```bash
# Small production server (2-4 CPU cores, 8GB RAM)
DUCKDB_MEMORY_LIMIT=4GB
DUCKDB_THREADS=4
DUCKDB_CHECKPOINT_THRESHOLD=1GB
DUCKDB_PRESERVE_INSERTION_ORDER=false

# Medium production server (8 CPU cores, 16GB RAM)
DUCKDB_MEMORY_LIMIT=8GB
DUCKDB_THREADS=6
DUCKDB_CHECKPOINT_THRESHOLD=2GB
DUCKDB_PRESERVE_INSERTION_ORDER=false

# Large production server (16+ CPU cores, 32GB+ RAM)
DUCKDB_MEMORY_LIMIT=16GB
DUCKDB_THREADS=12
DUCKDB_CHECKPOINT_THRESHOLD=4GB
DUCKDB_PRESERVE_INSERTION_ORDER=false
```

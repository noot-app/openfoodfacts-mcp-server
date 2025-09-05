# OpenFoodFacts MCP Server

A high-performance MCP (Model Context Protocol) server that provides access to the Open Food Facts dataset using DuckDB for fast querying. Built in Go with support for Railway deployment with persistent volumes.

## Features

- 🚀 **Fast Queries**: Uses DuckDB to query Parquet files with sub-100ms response times
- 📦 **Automatic Dataset Management**: Downloads and caches the Open Food Facts dataset locally
- 🔄 **Smart Refresh**: Periodically checks for dataset updates using ETag/hash comparison
- 🔒 **Concurrent Safe**: File locking prevents multiple instances from downloading simultaneously
- 🐳 **Railway Ready**: Designed for Railway deployment with persistent volume support
- 🔐 **Bearer Auth**: Secure API access with configurable authentication tokens
- 📊 **Structured Logging**: JSON-structured logs for monitoring and debugging

## API Endpoints

### Health Check
```bash
GET /health
```

Returns server status and readiness.

### Product Search
```bash
POST /query
Authorization: Bearer <AUTH_TOKEN>
Content-Type: application/json

{
  "name": "Nutella",
  "brand": "Ferrero", 
  "limit": 10
}
```

Search by barcode:
```bash
POST /query
Authorization: Bearer <AUTH_TOKEN>
Content-Type: application/json

{
  "barcode": "3017620422003",
  "limit": 1
}
```

## Configuration

Configure the server using environment variables:

```bash
# Authentication
AUTH_TOKEN=your-secret-token

# Dataset Configuration
PARQUET_URL=https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet
DATA_DIR=/data/off
PARQUET_PATH=/data/off/product-database.parquet
METADATA_PATH=/data/off/metadata.json
LOCK_FILE=/data/off/refresh.lock

# Refresh Behavior
REFRESH_INTERVAL_HOURS=24  # 0 to disable periodic refresh

# Server
PORT=8080
```

## Development

### Prerequisites

- Go 1.23+
- DuckDB (automatically handled by Go module)

### Running Locally

```bash
# Clone the repository
git clone https://github.com/noot-app/openfoodfacts-mcp-server
cd openfoodfacts-mcp-server

# Install dependencies
go mod download
go mod vendor

# Set environment variables
export AUTH_TOKEN=your-secret-token
export DATA_DIR=./data
export REFRESH_INTERVAL_HOURS=0  # Disable refresh for local dev

# Run the server
go run ./cmd/openfoodfacts-mcp-server
```

### Testing

```bash
# Run all tests
script/test

# Run tests with coverage
go test -v -cover ./...

# Lint code
script/lint
```

### Building

```bash
# Build for current platform
script/build

# Build for single target (faster during development)
script/build --single-target
```

## Railway Deployment

### 1. Setup Railway Project

1. Create a new Railway project
2. Connect your GitHub repository
3. Add a persistent volume mounted to `/data/off`

### 2. Environment Variables

Set the following environment variables in Railway:

```bash
AUTH_TOKEN=your-production-secret-token
DATA_DIR=/data/off
REFRESH_INTERVAL_HOURS=24
PORT=8080
RAILWAY_RUN_UID=0  # Required for volume permissions
```

### 3. Volume Configuration

- **Mount Path**: `/data/off`
- **Size**: At least 5GB (dataset is ~3GB)
- **Backup**: Recommended for production

### 4. Health Checks

Railway will automatically use the health check endpoint at `/health`.

## Usage Examples

### Search by Product Name

```bash
curl -X POST "https://your-app.railway.app/query" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Coca Cola","limit":5}'
```

### Search by Brand

```bash
curl -X POST "https://your-app.railway.app/query" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"brand":"Nestle","limit":10}'
```

### Search by Barcode

```bash
curl -X POST "https://your-app.railway.app/query" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"barcode":"3017620422003"}'
```

### Combined Search

```bash
curl -X POST "https://your-app.railway.app/query" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"chocolate","brand":"Ferrero","limit":5}'
```

## Response Format

```json
{
  "found": true,
  "products": [
    {
      "code": "3017620422003",
      "product_name": "Nutella",
      "brands": "Ferrero",
      "nutriments": {
        "energy": 2255,
        "fat": 30.9,
        "saturated-fat": 10.6,
        "carbohydrates": 57.5,
        "sugars": 56.3,
        "proteins": 6.3,
        "salt": 0.107
      },
      "link": "https://world.openfoodfacts.org/product/3017620422003/nutella-ferrero",
      "ingredients": {...}
    }
  ]
}
```

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│   MCP Server     │───▶│   DuckDB        │
│                 │    │                  │    │                 │
│ - Name Search   │    │ - Authentication │    │ - Parquet Query │
│ - Brand Search  │    │ - Request Parse  │    │ - Fast Lookups  │
│ - Barcode Query │    │ - Response Build │    │ - Predicate     │
└─────────────────┘    └──────────────────┘    │   Pushdown      │
                                                └─────────────────┘
                                 │
                                 ▼
                       ┌──────────────────┐
                       │ Dataset Manager  │
                       │                  │
                       │ - Download       │
                       │ - ETag Check     │
                       │ - File Locking   │
                       │ - Periodic Sync  │
                       └──────────────────┘
                                 │
                                 ▼
                       ┌──────────────────┐
                       │ Persistent Volume│
                       │                  │
                       │ - Parquet File   │
                       │ - Metadata       │
                       │ - Lock Files     │
                       └──────────────────┘
```

## Performance

- **Cold Start**: ~30-60 seconds (dataset download)
- **Warm Queries**: <100ms for most searches
- **Memory Usage**: ~500MB-1GB (depending on query complexity)
- **Dataset Size**: ~3GB compressed Parquet file
- **Concurrent Safety**: File locking prevents corruption

## Monitoring

The server provides structured JSON logs with the following levels:

- `INFO`: Normal operations, query completions
- `WARN`: Non-fatal issues, metadata check failures
- `ERROR`: Serious errors, query failures
- `DEBUG`: Detailed debugging information

Example log entry:
```json
{
  "time": "2025-01-09T10:30:45Z",
  "level": "INFO",
  "msg": "Query completed",
  "found": 5,
  "duration": "45ms",
  "name": "chocolate",
  "brand": "ferrero"
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for your changes
4. Ensure all tests pass: `script/test`
5. Lint your code: `script/lint`  
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

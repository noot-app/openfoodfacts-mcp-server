# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o openfoodfacts-mcp-server ./cmd/openfoodfacts-mcp-server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN adduser -D -s /bin/sh appuser

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/openfoodfacts-mcp-server .

# Create data directory and set permissions
RUN mkdir -p ./data && chown -R appuser:appuser ./data

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set default environment variables
ENV DATA_DIR=./data
ENV PORT=8080
ENV PARQUET_URL=https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet
ENV REFRESH_INTERVAL_SECONDS=86400

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./openfoodfacts-mcp-server"]

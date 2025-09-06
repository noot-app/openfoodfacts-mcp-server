# Multi-stage Dockerfile

# Build argument for Go version (can be overridden at build time)
ARG GO_VERSION=1.24.4

# Build stage - uses Go version from build arg (defaults to .go-version content)
FROM golang:${GO_VERSION}-bookworm AS builder

# Install build dependencies for DuckDB static linking
RUN apt update && apt install -y \
    zip build-essential libc++-dev libc++abi-dev

# Set working directory
WORKDIR /build

# Copy source code and vendor directory (full dependency vendoring)
COPY . .

# Get build information for embedding into binary (matching script/build --simple)
RUN VERSION_TAG="$(git describe --tags 2>/dev/null || echo 'dev')" && \
    COMMIT_SHA="$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" && \
    BUILD_TIME="$(git log -1 --format=%cI 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')" && \
    PROJECT_NAME="openfoodfacts-mcp-server" && \
    MODULE_PATH="github.com/noot-app/openfoodfacts-mcp-server" && \
    \
    # Build the binary with proper DuckDB static linking
    CGO_ENABLED=1 \
    GOPROXY=off \
    GOSUMDB=off \
    SOURCE_DATE_EPOCH="$(git log -1 --format=%ct 2>/dev/null || echo '0')" \
    go build \
        -mod=vendor \
        -trimpath \
        -v \
        -ldflags="-s -w -X ${MODULE_PATH}/internal/version.tag=${VERSION_TAG} -X ${MODULE_PATH}/internal/version.commit=${COMMIT_SHA} -X ${MODULE_PATH}/internal/version.buildTime=${BUILD_TIME}" \
        -o /build/${PROJECT_NAME} \
        ./cmd/${PROJECT_NAME}

# Runtime stage - use debian slim instead of scratch for DuckDB dependencies
FROM debian:bookworm-slim@sha256:b1a741487078b369e78119849663d7f1a5341ef2768798f7b7406c4240f86aef

# Create a non-root user with predictable UID/GID
RUN groupadd -r -g 1001 nonroot && useradd -r -u 1001 -g nonroot -s /bin/false nonroot

# Add ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from the builder stage and set ownership
COPY --from=builder --chown=nonroot:nonroot /build/openfoodfacts-mcp-server /openfoodfacts-mcp-server

# Create tmp-data directory for temporary downloads with proper ownership
RUN mkdir -p /tmp-data && chown nonroot:nonroot /tmp-data

# Switch to non-root user
USER nonroot

# Set the binary as the entrypoint
ENTRYPOINT ["/openfoodfacts-mcp-server"]

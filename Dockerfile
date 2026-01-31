# =============================================================================
# Service Feature - Multi-stage Dockerfile
# =============================================================================
# Build stage for compiling the Go binary
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum* ./

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o /build/builder \
    ./apps/default/cmd/main.go

# =============================================================================
# Runtime stage - minimal image
FROM alpine:3.19 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git \
    openssh-client \
    docker-cli

# Create non-root user
RUN addgroup -g 1000 feature && \
    adduser -u 1000 -G feature -s /bin/sh -D feature

# Create required directories
RUN mkdir -p /var/lib/feature-service/workspaces && \
    mkdir -p /var/log/feature-service && \
    mkdir -p /home/feature/.ssh && \
    chown -R feature:feature /var/lib/feature-service && \
    chown -R feature:feature /var/log/feature-service && \
    chown -R feature:feature /home/feature

# Copy binary from builder
COPY --from=builder /build/builder /usr/local/bin/builder

# Copy migrations if they exist
COPY --from=builder /build/migrations /app/migrations 2>/dev/null || true

# Set working directory
WORKDIR /app

# Switch to non-root user
USER feature

# Environment defaults
ENV SERVICE_NAME=service_feature \
    LOG_LEVEL=info \
    SERVER_ADDRESS=:8080 \
    WORKSPACE_BASE_PATH=/var/lib/feature-service/workspaces \
    SANDBOX_ENABLED=true \
    SANDBOX_TYPE=docker \
    METRICS_ENABLED=true \
    TRACING_ENABLED=false

# Expose ports
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the service
ENTRYPOINT ["/usr/local/bin/builder"]

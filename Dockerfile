# Multi-stage build for anchor Go application
FROM --platform=linux/amd64 golang:1.26.2-alpine AS builder

# Build arguments for metadata
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set GOPRIVATE for private modules
ENV GOPRIVATE=github.com/nanostack-dev/*

# Set working directory to the anchor app
WORKDIR /app

# Copy the Go client library first (required by replace directive)
COPY clients/go/ /clients/go/

# Copy go.mod and go.sum first for better layer caching
COPY apps/anchor/go.mod apps/anchor/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY apps/anchor/ ./

# Build the Go application with build info
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s -X anchor/internal/buildinfo.Version=${VERSION} -X anchor/internal/buildinfo.CommitSHA=${COMMIT_SHA} -X anchor/internal/buildinfo.BuildDate=${BUILD_DATE}" \
    -o anchor ./cmd/main.go

# Production stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Add metadata labels
LABEL org.opencontainers.image.title="anchor"
LABEL org.opencontainers.image.description="Multi-tenant service platform"
LABEL org.opencontainers.image.source="https://github.com/nanostack-dev/anchor"
LABEL org.opencontainers.image.licenses="MIT"

# Set working directory
WORKDIR /app

# Copy application binary
COPY --from=builder /app/anchor .

# Copy configuration and migrations
COPY --from=builder /app/application.yaml .
COPY --from=builder /app/migrations ./migrations

# Expose port
EXPOSE 8080

# Set default environment variables (non-sensitive defaults only)
ENV ENVIRONMENT=production
ENV SERVER_PORT=8080

# Mandatory environment variables that MUST be provided at runtime
# These will cause the application to fail if not set:
# - POSTGRES_USER
# - POSTGRES_PASSWORD
# - POSTGRES_DB
# - ADMIN_JWT_SECRET

# Container healthcheck: the app binary probes its own /health endpoint (exec
# form — this is a distroless image with no shell), so a process that is running
# but no longer serving is reported unhealthy and Swarm can reschedule it.
# start-period covers boot + migrations.
HEALTHCHECK --interval=15s --timeout=5s --start-period=45s --retries=3 \
    CMD ["/app/anchor", "-healthcheck"]

# Run the application
ENTRYPOINT ["/app/anchor"]
CMD []

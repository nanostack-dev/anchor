# Multi-stage build for anchor Go application
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS builder

# Build arguments for metadata
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE
ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

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

# Build the Go application with build info (scripts/build-anchor.sh, also
# what CI runs natively). The cache mount keeps Go's build cache between
# builds, so a rebuild compiles only what changed. The modules stay in the
# layer above, cached while go.sum does not change.
COPY scripts/build-anchor.sh /usr/local/bin/build-anchor
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH build-anchor anchor

# The binary alone, at /<arch>/anchor. CI replaces this stage with a
# directory of binaries it built natively, amd64/anchor and arm64/anchor,
# with Go's caches warm (`--build-context binary=<dir>`, see
# .github/actions/build-image); the builder stage then does not run. One
# directory serves every platform of a multi-platform build, so the final
# stage picks its binary by TARGETARCH. Keep the Go version above equal to the
# toolchain in apps/anchor/go.mod.
FROM scratch AS binary
ARG TARGETARCH
COPY --from=builder /app/anchor /${TARGETARCH}/anchor

# Production stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

# Add metadata labels
LABEL org.opencontainers.image.title="anchor"
LABEL org.opencontainers.image.description="Multi-tenant service platform"
LABEL org.opencontainers.image.source="https://github.com/nanostack-dev/anchor"
LABEL org.opencontainers.image.licenses="MIT"

# The infra deploy reads this label to pick the application.yaml format it
# renders for this image (scripts/image-config-format.sh). An image without
# it gets the old fx format, which this binary refuses at boot.
LABEL dev.nanostack.config-format="nano"

# Set working directory
WORKDIR /app

# Copy application binary
ARG TARGETARCH
COPY --from=binary /${TARGETARCH}/anchor .

# Copy configuration and migrations
COPY apps/anchor/application.yaml .
COPY apps/anchor/migrations ./migrations

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

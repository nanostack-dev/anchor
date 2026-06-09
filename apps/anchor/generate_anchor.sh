#!/usr/bin/env bash
set -eo pipefail # Exit on error, pipefail ensures pipeline errors are caught

CURRENT_DIR=$(pwd)
APP_NAME=anchor
ANCHOR_DB_PORT=25895
ANCHOR_DB_PASSWORD=$(openssl rand -base64 32 | tr -d /=+ | cut -c1-32)
POSTGRES_CONTAINER="temp-postgres"

export ANCHOR_DB_PORT
export ANCHOR_DB_PASSWORD

# Clean up function to ensure we always remove the container
cleanup() {
  echo "Cleaning up resources..."
  docker stop ${POSTGRES_CONTAINER} 2>/dev/null || true
  docker rm ${POSTGRES_CONTAINER} 2>/dev/null || true
}

# Ensure cleanup runs even if script fails
trap cleanup EXIT

# Check required tools are installed
check_dependencies() {
  for cmd in docker migrate go; do
    if ! command -v $cmd &>/dev/null; then
      echo "Error: $cmd is required but not installed. Please install it first."
      exit 1
    fi
  done
}

# Start a PostgreSQL container and wait for it to be ready
start_postgres() {
  echo "Starting PostgreSQL container on port ${ANCHOR_DB_PORT}..."
  cleanup # Ensure no old container exists

  docker run --name ${POSTGRES_CONTAINER} \
    -e POSTGRES_PASSWORD=${ANCHOR_DB_PASSWORD} \
    -p ${ANCHOR_DB_PORT}:5432 \
    -d postgres

  # Wait for PostgreSQL to be ready
  echo "Waiting for PostgreSQL to be ready..."
  for i in {1..30}; do
    if docker exec ${POSTGRES_CONTAINER} pg_isready -U postgres &>/dev/null; then
      echo "PostgreSQL is ready!"
      sleep 2 # Give it a moment more to fully initialize
      return 0
    fi
    echo -n "."
    sleep 1
  done

  echo "Error: PostgreSQL failed to start in time"
  exit 1
}

# Create database
create_database() {
  echo "Creating database 'anchor'..."
  docker exec ${POSTGRES_CONTAINER} createdb -U postgres anchor || {
    echo "Error: Failed to create database"
    exit 1
  }
}

run_migrations() {
  local folder="."
  if [ -d "$folder/migrations" ]; then
    MODULE_NAME=$(basename "$folder")
    echo "===== Processing module: $MODULE_NAME ====="
    # Check if migrations directory has files
    if [ -z "$(ls -A $folder/migrations 2>/dev/null)" ]; then
      echo "No migration files found in $folder/migrations, skipping..."
      return 0
    fi

    echo "Running migrations from: $folder/migrations"
    migrate -path $folder/migrations \
      -database "postgres://postgres:${ANCHOR_DB_PASSWORD}@localhost:${ANCHOR_DB_PORT}/anchor?sslmode=disable" up || {
      echo "Error: Migration failed for $MODULE_NAME"
      exit 1
    }

    echo "✓ Completed migrations for: $MODULE_NAME"
  fi
}

# Generate code and client
generate_code() {
  echo "===== Running code generation steps ====="

  # create openapi-cleaned.yaml if it doesn't exist
  sed -e '/x-go-type: string/!{/x-go-type:/d;}' \
        -e '/x-go-type-import:/d' \
        -e '/\s*path: anchor\/internal\/domain.*/d' \
        -e '/\s*path: github\.com\/nanostack-dev\/nanostack-framework\/pkg\/search.*/d' ./cmd/http/openapi.yaml > ./cmd/http/openapi-cleaned.yaml

  go generate tools.go

  # OAPI Codegen
  echo "Generating OpenAPI code..."
  go generate tools.go || {
    echo "Error: Failed to generate OAPI code"
    exit 1
  }

  # Database code generation
  echo "Running database code generation..."
  go run dbgen.go || {
    echo "Error: Failed to run dbgen.go"
    exit 1
  }
}

generate_client_frontend () {
  echo "Generating client frontend code..."
  CLIENT_DIR="../../anchor-ui"
  if [ -d "$CLIENT_DIR" ]; then
    cd "$CLIENT_DIR"
    pnpm run openapi-ts || {
      echo "Error: Failed to generate client frontend"
      exit 1
    }
    cd "$CURRENT_DIR"
  else
    echo "Warning: Client frontend directory not found at $CLIENT_DIR"
  fi
}
# Main execution
main() {
  echo "🚀 Starting build process for $APP_NAME"
  check_dependencies
  start_postgres
  create_database
  run_migrations
  generate_code
  generate_client_frontend

  echo "✅ Build process completed successfully!"
}

main

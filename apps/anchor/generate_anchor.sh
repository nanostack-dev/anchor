#!/usr/bin/env bash
# Regenerates every generated file of apps/anchor and the anchor-ui client.
# Run it from apps/anchor: every path in it is relative to that directory.
#
# go tool nanostack gen runs the db, repo, config and wire steps. Its db step reads no
# database while the migrations, the image and the generated files match
# internal/db/gen/anchor/nanogen.sum. Otherwise it applies the migrations to a
# container that it starts with Docker, or to the empty server that
# NANO_GEN_DATABASE_URL names.
set -euo pipefail

CURRENT_DIR=$(pwd)
CLEANED_SPEC=./cmd/http/openapi-cleaned.yaml

check_dependencies() {
  for cmd in go pnpm; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "Error: $cmd is required but not installed. Please install it first."
      exit 1
    fi
  done
}

generate_api() {
  echo "Generating the OpenAPI server and clients..."
  ../../scripts/client-spec.sh ./cmd/http/openapi.yaml "$CLEANED_SPEC"
  trap 'rm -f "$CURRENT_DIR/$CLEANED_SPEC"' EXIT
  go tool oapi-codegen --config=codegen.yaml ./cmd/http/openapi.yaml
  go tool oapi-codegen --config=client-types-codegen.yaml "$CLEANED_SPEC"
  go tool oapi-codegen --config=client-codegen.yaml "$CLEANED_SPEC"
}

generate_nano() {
  echo "Running nanostack gen (database models, repository bases, config and wiring)..."
  go tool nanostack gen
}

generate_client_frontend() {
  echo "Generating the anchor-ui client..."
  local client_dir="../../anchor-ui"
  if [ -d "$client_dir" ]; then
    (cd "$client_dir" && pnpm run openapi-ts)
  else
    echo "Warning: client frontend directory not found at $client_dir"
  fi
}

main() {
  check_dependencies
  generate_api
  generate_nano
  generate_client_frontend
  echo "Generation completed."
}

main

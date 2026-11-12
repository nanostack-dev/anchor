#!/bin/sh
# Builds the anchor binary: what the Dockerfile's builder stage runs, and what
# CI runs natively before it packages the image (.github/workflows/go.yml,
# publish). One command, so both builds use the same flags.
#
# Usage: scripts/build-anchor.sh <output>   (run in apps/anchor)
# Env: VERSION, COMMIT_SHA, BUILD_DATE; GOOS and GOARCH to cross-compile.
set -eu
CGO_ENABLED=0 go build \
  -ldflags="-w -s -X anchor/internal/buildinfo.Version=${VERSION:-dev} -X anchor/internal/buildinfo.CommitSHA=${COMMIT_SHA:-unknown} -X anchor/internal/buildinfo.BuildDate=${BUILD_DATE:-}" \
  -o "$1" ./cmd/main.go

#!/bin/sh
# Builds the anchor binary. The Dockerfile's builder stage and CI's native
# build (.github/workflows/go.yml, publish) both run this script, so both
# builds use the same flags.
#
# Usage: scripts/build-anchor.sh <output>   (run in apps/anchor)
# Env:   VERSION, COMMIT_SHA, BUILD_DATE (stamped into internal/buildinfo);
#        GOOS and GOARCH to cross-compile.
set -eu

buildinfo=anchor/internal/buildinfo
# -w -s drop the symbol table and DWARF data: a smaller image.
ldflags="-w -s"
ldflags="$ldflags -X $buildinfo.Version=${VERSION:-dev}"
ldflags="$ldflags -X $buildinfo.CommitSHA=${COMMIT_SHA:-unknown}"
ldflags="$ldflags -X $buildinfo.BuildDate=${BUILD_DATE:-}"

CGO_ENABLED=0 go build -ldflags="$ldflags" -o "$1" ./cmd/main.go

#!/usr/bin/env bash
# Fails when the generated code under apps/anchor differs from what
# `nanostack gen` produces from a fresh database: what the Dagger
# generated-code check runs, outside Dagger so Go's caches stay warm.
#
# Usage: scripts/check-generated.sh   (needs Docker and Go)
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
name="anchor-nanogen-$$"
# Keep the image equal to generatedCode in .dagger/modules/anchor/main.dang.
docker run -d --rm --name "$name" -e POSTGRES_PASSWORD=nanogen -e NO_TS_TUNE=true \
  -p 127.0.0.1::5432 timescale/timescaledb:2.23.0-pg18 > /dev/null
trap 'docker stop "$name" > /dev/null 2>&1 || true' EXIT
port="$(docker port "$name" 5432/tcp | head -1 | sed 's/.*://')"
# Over TCP: during its init the server listens on the Unix socket only.
for _ in $(seq 120); do
  if docker exec "$name" pg_isready -h 127.0.0.1 -U postgres > /dev/null 2>&1; then break; fi
  sleep 0.5
done
docker exec "$name" pg_isready -h 127.0.0.1 -U postgres > /dev/null

cd "$root_dir/apps/anchor"
GOWORK=off NANO_GEN_DATABASE_URL="postgres://postgres:nanogen@127.0.0.1:$port/postgres?sslmode=disable" \
  go tool nanostack gen -check -fresh -diff

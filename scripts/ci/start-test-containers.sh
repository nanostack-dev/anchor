#!/usr/bin/env bash
# Starts the component and service suites' Postgres and Redis in the
# background, and pulls Mailpit's image, so they are ready by the time Go has
# compiled the tests (they took 5 s to start after a 22 s compile).
#
# The suites use these containers instead of starting their own when
# ANCHOR_IT_POSTGRES_ADDR and ANCHOR_IT_REDIS_ADDR are set
# (apps/anchor/cmd/it/shared/test_setup.go); this script exports both to the
# following GitHub Actions steps. Keep the images and settings equal to
# test_setup.go and the Mailpit helper.
#
# Usage: scripts/ci/start-test-containers.sh   (in a GitHub Actions step)
set -euo pipefail

readonly postgres_addr=127.0.0.1:55432
readonly redis_addr=127.0.0.1:56379

start_in_background() {
  nohup "$@" > /dev/null 2>&1 &
}

start_in_background docker run -d --name anchor-it-postgres -p "$postgres_addr:5432" \
  -e POSTGRES_DB=anchor -e POSTGRES_USER=anchor -e POSTGRES_PASSWORD=anchor -e NO_TS_TUNE=true \
  timescale/timescaledb:2.23.0-pg18
start_in_background docker run -d --name anchor-it-redis -p "$redis_addr:6379" redis:7-alpine
start_in_background docker pull -q axllent/mailpit:v1.20

echo "ANCHOR_IT_POSTGRES_ADDR=$postgres_addr" >> "$GITHUB_ENV"
echo "ANCHOR_IT_REDIS_ADDR=$redis_addr" >> "$GITHUB_ENV"

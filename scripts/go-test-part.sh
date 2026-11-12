#!/usr/bin/env bash
# Runs one part of a Go package's top-level tests: every <parts>th test name,
# from offset <part>, so one suite splits over several runners. Test names come
# from the sources (`go test -list` would run TestMain, which starts the
# containers). Fails unless every selected test ran, so a name the filter
# misses cannot pass silently.
#
# Usage: scripts/go-test-part.sh <package dir> <part> <parts>
# Example: scripts/go-test-part.sh apps/anchor/cmd/it/ct 0 5
set -euo pipefail

if [[ $# -ne 3 || ! $2 =~ ^[0-9]+$ || ! $3 =~ ^[1-9][0-9]*$ || $2 -ge $3 ]]; then
  printf 'Usage: %s <package dir> <part> <parts>\n' "$0" >&2
  exit 2
fi
dir="$1" part="$2" parts="$3"
cd "$dir"
tests=$(grep -hoE '^func Test[A-Za-z0-9_]+\(t \*testing\.T\)' ./*_test.go |
  sed -E 's/^func (Test[A-Za-z0-9_]+).*/\1/' | sort -u |
  awk -v n="$parts" -v p="$part" 'NR % n == p' | paste -sd'|' -)
if [[ -z $tests ]]; then
  printf 'No tests selected in %s, part %s of %s\n' "$dir" "$part" "$parts" >&2
  exit 1
fi
json="$(mktemp)"
rc=0
# MEASURE (temporary): pull, compile, then the test run.
t0=$(date +%s)
for img in timescale/timescaledb:2.23.0-pg18 redis:7-alpine axllent/mailpit:v1.20; do docker pull -q "$img" > /dev/null & done; wait
t1=$(date +%s)
go test -c -o "$json.bin" .
t2=$(date +%s)
go run gotest.tools/gotestsum@v1.13.0 --format pkgname-and-test-fails --jsonfile "$json" -- -run "^($tests)\$" . || rc=$?
t3=$(date +%s)
echo "MEASURE pull=$((t1-t0))s compile=$((t2-t1))s gotestsum=$((t3-t2))s"
want=$(tr '|' '\n' <<< "$tests" | wc -l)
ran=$(grep -oE '"Action":"(pass|skip)","Package":"[^"]*","Test":"Test[^/"]*"' "$json" | wc -l)
printf 'ran %s of %s selected tests\n' "$ran" "$want"
if [[ $ran -ne $want ]]; then exit 1; fi
exit "$rc"

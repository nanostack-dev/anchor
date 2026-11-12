#!/usr/bin/env bash
# Runs one part of a Go package's top-level tests: every <parts>th test name,
# from offset <part>, so one suite splits over several runners. Test names come
# from the sources (`go test -list` would run TestMain, which starts the
# containers). Fails unless every selected test ran, so a name the filter
# misses cannot pass silently.
#
# Usage: scripts/go-test-part.sh <package dir> <part> <parts>
# Example: scripts/go-test-part.sh apps/anchor/cmd/it/ct 0 5
# GO_TEST_FLAGS adds flags to `go test` (CI sets -parallel).
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
read -ra extra <<< "${GO_TEST_FLAGS:-}"
# MEASURE (temporary): what the test binary compiles, and how long.
xlog="$(mktemp)"
t0=$(date +%s)
go test -c -x -o "$xlog.bin" . 2> "$xlog" || true
echo "MEASURE compile=$(( $(date +%s) - t0 ))s packages=$(grep -cE '/compile( |$)' "$xlog" || true)"
grep -E '/compile( |$)' "$xlog" | grep -oE -- ' -p [^ ]+' | head -60 | tr '\n' ' ' || true; echo
json="$(mktemp)"
rc=0
go run gotest.tools/gotestsum@v1.13.0 --format pkgname-and-test-fails --jsonfile "$json" -- ${extra[@]+"${extra[@]}"} -run "^($tests)\$" . || rc=$?
want=$(tr '|' '\n' <<< "$tests" | wc -l)
ran=$(grep -oE '"Action":"(pass|skip)","Package":"[^"]*","Test":"Test[^/"]*"' "$json" | wc -l)
printf 'ran %s of %s selected tests\n' "$ran" "$want"
if [[ $ran -ne $want ]]; then exit 1; fi
exit "$rc"

#!/usr/bin/env bash
# Runs one part of a Go package's top-level tests, so one suite splits over
# several runners. Test names come from the sources (`go test -list` would run
# TestMain, which starts the containers). Fails unless every selected test ran,
# so a name the filter misses cannot pass silently.
#
# Without GO_TEST_TIMES a part takes every <parts>th test name from offset
# <part>. GO_TEST_TIMES names a file of "<test> <seconds>" lines: the tests
# then go, longest first, to the part with the least time so far (a test
# missing from the file counts 0.5 s), so the parts take about as long.
#
# Usage: scripts/go-test-part.sh <package dir> <part> <parts>
# Example: GO_TEST_TIMES=scripts/ct-test-times.txt scripts/go-test-part.sh apps/anchor/cmd/it/ct 0 8
# GO_TEST_FLAGS adds flags to `go test` (CI sets -parallel); GO_TEST_GCFLAGS
# sets -gcflags (CI: "all=-N -l", which compiles faster).
set -euo pipefail

if [[ $# -ne 3 || ! $2 =~ ^[0-9]+$ || ! $3 =~ ^[1-9][0-9]*$ || $2 -ge $3 ]]; then
  printf 'Usage: %s <package dir> <part> <parts>\n' "$0" >&2
  exit 2
fi
dir="$1" part="$2" parts="$3"
times=""
if [[ -n ${GO_TEST_TIMES:-} ]]; then times="$(cd "$(dirname "$GO_TEST_TIMES")" && pwd)/$(basename "$GO_TEST_TIMES")"; fi
cd "$dir"
names=$(grep -hoE '^func Test[A-Za-z0-9_]+\(t \*testing\.T\)' ./*_test.go |
  sed -E 's/^func (Test[A-Za-z0-9_]+).*/\1/' | sort -u)
if [[ -n $times ]]; then
  tests=$(awk -v times="$times" '
      BEGIN { while ((getline line < times) > 0) if (line !~ /^#/) { split(line, f, " "); secs[f[1]] = f[2] } }
      { printf "%s %s\n", (($0 in secs) ? secs[$0] : 0.5), $0 }' <<< "$names" |
    LC_ALL=C sort -k1,1gr -k2,2 |
    awk -v n="$parts" -v p="$part" '
      { best = 0; for (i = 1; i < n; i++) if (load[i] < load[best]) best = i
        load[best] += $1; if (best == p) print $2 }' | paste -sd'|' -)
else
  tests=$(awk -v n="$parts" -v p="$part" 'NR % n == p' <<< "$names" | paste -sd'|' -)
fi
if [[ -z $tests ]]; then
  printf 'No tests selected in %s, part %s of %s\n' "$dir" "$part" "$parts" >&2
  exit 1
fi
read -ra extra <<< "${GO_TEST_FLAGS:-}"
if [[ -n ${GO_TEST_GCFLAGS:-} ]]; then extra+=("-gcflags=$GO_TEST_GCFLAGS"); fi
json="$(mktemp)"
SECONDS=0; go test -c -o /dev/null ${extra[@]+"${extra[@]}"} . ; echo "MEASURE compile=${SECONDS}s"; SECONDS=0
rc=0
go run gotest.tools/gotestsum@v1.13.0 --format pkgname-and-test-fails --jsonfile "$json" -- ${extra[@]+"${extra[@]}"} -run "^($tests)\$" . || rc=$?
echo "MEASURE test=${SECONDS}s"
want=$(tr '|' '\n' <<< "$tests" | wc -l)
ran=$(grep -oE '"Action":"(pass|skip)","Package":"[^"]*","Test":"Test[^/"]*"' "$json" | wc -l)
printf 'ran %s of %s selected tests\n' "$ran" "$want"
# The slow tests, to keep scripts/ct-test-times.txt honest.
printf 'top-level tests over 5 s:\n'
go run gotest.tools/gotestsum@v1.13.0 tool slowest --jsonfile "$json" --threshold 5s | awk '$2 !~ /\//' || true
if [[ $ran -ne $want ]]; then exit 1; fi
exit "$rc"
